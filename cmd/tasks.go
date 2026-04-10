package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
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

type Task struct {
	ID        int
	TaskName  sql.NullString
	StartTime time.Time
	EndTime   sql.NullTime
}

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
	flagInteractive bool
	flagStopTask    int
	flagTaskID      int
	flagStopShort   bool
	flagGetTasks    bool
	flagRunning     bool
	flagNewTask     string
	flagShowTask    int
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Task timer tracker",
	Long: `Task timer tracker — create tasks with running timers, stop them, and view elapsed time.

Examples:
  barbtils tasks -i                    Full-screen TUI (arrow keys + enter)
  barbtils tasks --get_tasks           List running timers (end_time IS NULL)
  barbtils tasks --get_tasks --running List completed timers
  barbtils tasks --stop_task 53        Stop timer for task ID 53
  barbtils tasks -s -t 53              Same (short form: stop + task id)
  barbtils tasks --new "My task"       Create a new task
  barbtils tasks --show 12             Show details for task ID 12`,
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
	w = taskWriter(w)
	now := time.Now()

	var nameArg interface{}
	if name != "" {
		nameArg = name
	}

	row := db.QueryRow(
		`INSERT INTO tasks (task_name, start_time) VALUES ($1, $2) RETURNING id, task_name, start_time`,
		nameArg, now,
	)

	var t Task
	if err := row.Scan(&t.ID, &t.TaskName, &t.StartTime); err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}

	fmt.Fprintln(w, tasksOK.Render("Created task #"+strconv.Itoa(t.ID))+" "+tasksMuted.Render(nullStrDisplay(t.TaskName)))
	fmt.Fprintln(w, formatTaskBlock(t))
	return nil
}

func savedEditedTask(db *sql.DB, taskID int, endTime time.Time, w io.Writer) error {
	w = taskWriter(w)
	row := db.QueryRow(
		`UPDATE tasks SET end_time = $1 WHERE id = $2 RETURNING id, task_name, start_time, end_time`,
		endTime, taskID,
	)

	var t Task
	if err := row.Scan(&t.ID, &t.TaskName, &t.StartTime, &t.EndTime); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no task found with ID %d", taskID)
		}
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Fprintln(w, tasksOK.Render("Stopped task #"+strconv.Itoa(t.ID)))
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
	w = taskWriter(w)
	var query string
	var title string
	if isCompleted {
		query = `SELECT id, task_name, start_time FROM tasks WHERE end_time IS NOT NULL ORDER BY id`
		title = "Completed tasks"
	} else {
		query = `SELECT id, task_name, start_time FROM tasks WHERE end_time IS NULL ORDER BY id`
		title = "Running tasks"
	}

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	type rowT struct {
		id   int
		name string
		startTime time.Time
	}
	var list []rowT
	for rows.Next() {
		var id int
		var name sql.NullString
		var startTime time.Time
		if err := rows.Scan(&id, &name, &startTime); err != nil {
			return err
		}
		list = append(list, rowT{id: id, name: nullStrDisplay(name), startTime: startTime})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Fprintln(w, tasksAccent.Render(title))
	if len(list) == 0 {
		fmt.Fprintln(w, tasksMuted.Render("  (none)"))
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTASK\tSTART TIME")
	for _, r := range list {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", r.id, r.name, r.startTime.Format("2006-01-02 - 15:04:05 AM"))
	}
	return tw.Flush()
}

func showSpecificTimer(db *sql.DB, id int, w io.Writer) error {
	w = taskWriter(w)
	row := db.QueryRow(`SELECT id, task_name, start_time, end_time FROM tasks WHERE id = $1`, id)

	var t Task
	if err := row.Scan(&t.ID, &t.TaskName, &t.StartTime, &t.EndTime); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no task found with ID %d", id)
		}
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Fprintln(w, tasksAccent.Render("Task #"+strconv.Itoa(t.ID)))
	fmt.Fprintln(w, tasksAccent.Render("Start time"+ t.StartTime.String()))
	fmt.Fprintln(w, formatTaskBlock(t))

	var diff time.Duration
	if !t.EndTime.Valid {
		diff = time.Since(t.StartTime)
		fmt.Fprintln(w, tasksWarn.Render("Status: running"))
	} else {
		diff = t.EndTime.Time.Sub(t.StartTime)
		fmt.Fprintln(w, tasksMuted.Render("Status: completed"))
	}

	fmt.Fprintln(w, tasksWarn.Render("Elapsed ")+tasksAccent.Render(fmtDuration(diff)))
	return nil
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

func nullStrDisplay(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return "—"
}

func formatTaskBlock(t Task) string {
	end := "—"
	if t.EndTime.Valid {
		end = t.EndTime.Time.Format(time.RFC3339)
	}
	lines := []string{
		tasksMuted.Render("Start: ") + t.StartTime.Format(time.RFC3339),
		tasksMuted.Render("End:   ") + end,
	}
	if t.TaskName.Valid && t.TaskName.String != "" {
		lines = append([]string{tasksMuted.Render("Name:  ") + t.TaskName.String}, lines...)
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

	RootCmd.AddCommand(tasksCmd)
}
