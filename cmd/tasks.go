package cmd

import (
	"barbtils/internal/database"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Lipgloss styles for task output (stdout). Keep contrast reasonable on light and dark terminals.
var (
	tasksAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	tasksOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	tasksWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	tasksErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	tasksMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tasksBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
)

func openTasksDB() (*sql.DB, error) {
	dbURL := viper.GetString("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is not configured — set it in your barbtils.toml or environment")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to reach database: %w", err)
	}
	return db, nil
}

var (
	flagInteractive   bool
	flagStopTask      int
	flagTaskID        int
	flagStopShort     bool
	flagGetTasks      bool
	flagRunning       bool
	flagNewTask       string
	flagShowTask      int
	flagTruncateTable string
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Task timer tracker",
	Long: `Task timer tracker — create tasks with running timers, stop them, and view elapsed time.
Uses PostgreSQL as the database (DB Requires Manual setup, including schema creation.)

Examples:
  barbtils tasks -i                    Full-screen TUI (arrow keys + enter)
  barbtils tasks --get_tasks           List running timers (end_time IS NULL)
  barbtils tasks --get_tasks --running List completed timers
  barbtils tasks --stop_task 53        Stop timer for task ID 53
  barbtils tasks -s -t 53              Same (short form: stop + task id)
  barbtils tasks --new "My task"       Create a new task
  barbtils tasks --show 12             Show details for task ID 12
  barbtils tasks --truncate yes        Removes all tasks, and resets counter to 1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagInteractive {
			db, err := openTasksDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return taskInteractiveLoop(db)
		}

		if flagGetTasks {
			db, err := openTasksDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return getAllRunningTimers(db, flagRunning, nil)
		}

		stopID := 0
		switch {
		case flagStopTask > 0:
			stopID = flagStopTask
		case flagStopShort && flagTaskID > 0:
			stopID = flagTaskID
		case flagTaskID > 0 && !flagStopShort:
			return fmt.Errorf("use --stop (-s) with --task-id (-t), or use --stop_task <id>")
		}
		if stopID > 0 {
			db, err := openTasksDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return stopTimer(db, stopID, nil)
		}

		if flagNewTask != "" {
			db, err := openTasksDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return saveNewTask(db, flagNewTask, nil)
		}

		if flagShowTask > 0 {
			db, err := openTasksDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return showSpecificTimer(db, flagShowTask, nil)
		}

		allowedString := []string{"yes", "y", "1"}
		if flagTruncateTable != "" {
			flagTruncateTable = strings.ToLower(flagTruncateTable)
			if slices.Contains(allowedString, flagTruncateTable) {
				db, err := openTasksDB()
				if err != nil {
					return err
				}
				return truncateAllTasks(db, os.Stdout)
			} else {
				return fmt.Errorf("Please confirm with Yes, Y or 1")
			}
		}

		fmt.Println(tasksMuted.Render("No action selected."))
		fmt.Println(tasksMuted.Render("Run with --help to see options, or use -i for the interactive menu."))
		return nil
	},
}

func taskWriter(w io.Writer) io.Writer {
	if w == nil {
		return os.Stdout
	}
	return w
}

func saveNewTask(db *sql.DB, name string, w io.Writer) error {
	ctx := context.Background()
	sqlQ := database.New(db)
	w = taskWriter(w)
	now := time.Now()

	if name == "" {
		return fmt.Errorf("Task name must not be empty!")
	}

	t, err := sqlQ.CreateNewTask(ctx, database.CreateNewTaskParams{TaskName: name, StartTime: now})
	if err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}

	fmt.Fprintln(w, tasksOK.Render("Created task #"+strconv.Itoa(int(t.ID)))+" "+tasksMuted.Render(t.TaskName))
	fmt.Fprintln(w, formatTaskBlock(t))
	return nil
}

func savedEditedTask(db *sql.DB, taskID int, endTime time.Time, w io.Writer) error {
	ctx := context.Background()
	sqlQ := database.New(db)
	w = taskWriter(w)

	t, err := sqlQ.UpdateSelectedTask(ctx, database.UpdateSelectedTaskParams{
		ID: int32(taskID),
		EndTime: sql.NullTime{
			Time:  endTime,
			Valid: true,
		},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no running task found with ID %d", taskID)
		}
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Fprintln(w, tasksOK.Render("Stopped task #"+strconv.Itoa(int(t.ID))))
	fmt.Fprintln(w, formatTaskBlock(t))

	diff := t.EndTime.Time.Sub(t.StartTime)
	fmt.Fprintln(w, tasksWarn.Render("Elapsed ")+tasksAccent.Render(fmtDuration(diff)))
	return nil
}

func stopTimer(db *sql.DB, id int, w io.Writer) error {
	return savedEditedTask(db, id, time.Now(), w)
}

// truncateAllTasks removes all rows from tasks (used by interactive TUI after confirmation).
func truncateAllTasks(db *sql.DB, w io.Writer) error {
	w = taskWriter(w)
	fmt.Fprintln(w, tasksWarn.Render("Truncating tasks table…"))
	if _, err := db.Exec(`TRUNCATE TABLE tasks RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("truncate failed: %w", err)
	}
	fmt.Fprintln(w, tasksOK.Render("Done. All task rows were removed."))
	return nil
}

func getAllRunningTimers(db *sql.DB, isCompleted bool, w io.Writer) error {
	ctx := context.Background()
	sqlQ := database.New(db)
	w = taskWriter(w)
	var title string
	if isCompleted {
		title = "Completed tasks"
	} else {
		title = "Running tasks"
	}

	var tasks []database.Task
	if ct, err := sqlQ.CompletedTasks(ctx); isCompleted {
		if err != nil {
			return fmt.Errorf("Error fetching running tasks: %v", err)
		}
		tasks = ct
	} else {
		rt, err := sqlQ.RunningTasks(ctx)
		if err != nil {
			return fmt.Errorf("Error fetching running tasks: %v", err)
		}
		tasks = rt
	}

	fmt.Fprintln(w, tasksAccent.Render(title))
	if len(tasks) == 0 {
		fmt.Fprintln(w, tasksMuted.Render("  (none)"))
		return nil
	}

	if isCompleted {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tTASK\tSTART TIME\tEND TIME\tTOTAL TASK TIME")
		for _, r := range tasks {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", r.ID, r.TaskName, r.StartTime.Format("03:04:05 PM"), r.EndTime.Time.Format("03:04:05 PM"), tasksAccent.Render(getHumanReadableTimeDiff(r, w, false)))
		}
		return tw.Flush()
	} else {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tTASK\tSTART TIME\tElapsed Time")
		for _, r := range tasks {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.ID, r.TaskName, r.StartTime.Format("03:04:05 PM"), tasksAccent.Render(getHumanReadableTimeDiff(r, w, false)))
		}
		return tw.Flush()
	}
}

func showSpecificTimer(db *sql.DB, id int, w io.Writer) error {
	ctx := context.Background()
	sqlQ := database.New(db)
	w = taskWriter(w)
	t, err := sqlQ.ShowSpecificTimer(ctx, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no task found with ID %d", id)
		}
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Fprintln(w, tasksAccent.Render("Task #"+strconv.Itoa(int(t.ID))))
	fmt.Fprintln(w, formatTaskBlock(t))

	fmt.Fprintln(w, tasksWarn.Render("Elapsed ")+tasksAccent.Render(getHumanReadableTimeDiff(t, w, true)))
	return nil
}

func getHumanReadableTimeDiff(t database.Task, w io.Writer, status bool) string {
	var diff time.Duration
	if !t.EndTime.Valid {
		// diff = time.Now().In(time.Local).Sub(t.StartTime)
		diff = time.Now().UTC().Sub(t.StartTime.UTC())
		if status {
			fmt.Fprintln(w, tasksWarn.Render("Status: running"))
		}
	} else {
		diff = t.EndTime.Time.Sub(t.StartTime)
		if status {
			fmt.Fprintln(w, tasksMuted.Render("Status: completed"))
		}
	}

	return fmtDuration(diff)
}

func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	total := int(d.Seconds())
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d days", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hours", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d minutes", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d seconds", seconds))
	}
	return strings.Join(parts, ", ")
}

func formatTaskBlock(t database.Task) string {
	end := "—"
	if t.EndTime.Valid {
		end = t.EndTime.Time.Format(time.RFC3339)
	}
	lines := []string{
		tasksMuted.Render("Start: ") + t.StartTime.Format(time.RFC3339),
		tasksMuted.Render("End:   ") + end,
	}
	if t.TaskName != "" {
		lines = append([]string{tasksMuted.Render("Name:  ") + t.TaskName}, lines...)
	}
	return strings.Join(lines, "\n")
}

func init() {
	tasksCmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "Start interactive session")
	tasksCmd.Flags().BoolVarP(&flagGetTasks, "get_tasks", "g", false, "List tasks")
	tasksCmd.Flags().BoolVarP(&flagRunning, "running", "r", false, "Show completed tasks (use with --get_tasks)")
	tasksCmd.Flags().IntVar(&flagStopTask, "stop_task", 0, "Stop timer for task ID")
	tasksCmd.Flags().BoolVarP(&flagStopShort, "stop", "s", false, "Stop action (use with -t)")
	tasksCmd.Flags().IntVarP(&flagTaskID, "task-id", "t", 0, "Task ID (use with -s)")
	tasksCmd.Flags().StringVarP(&flagNewTask, "new", "n", "", "Create a new task with given name")
	tasksCmd.Flags().IntVar(&flagShowTask, "show", 0, "Show details for task ID")
	tasksCmd.Flags().StringVar(&flagTruncateTable, "truncate", "", "Truncate all data in the tasks table. \nAcceptabled values are 'Y', 'Yes', or 1")

	rootCmd.AddCommand(tasksCmd)
}
