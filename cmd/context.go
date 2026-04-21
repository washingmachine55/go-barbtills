/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	ch "barbtils/internal/cmdHelper"
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// contextCmd represents the context command
var ContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Easy Context switching",
	Long: `I know my ADHD is deliberatly fucking my shit up, so here i am,   
hyperfocusing to create this command so that perhaps this...might help me

LMFAO I SURE HOPE IT DOES LOL`,
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("new").Value.String() == "true" {
			gatherDetails()
		}
	},
}

func init() {
	RootCmd.AddCommand(ContextCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// contextCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	ContextCmd.Flags().BoolP("new", "n", false, "Create new context for the current session")
	// contextCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func gatherDetails() {
	l.Logger.Infof("[CWD]: %v\n", ch.GetCWD())
	l.Logger.Print(SepLine)
	l.Logger.Info("[Last 5 Fish Commands]:")
	fmt.Fprint(os.Stdout, getLast5FishCommands())

	sessions, err := getFishSessions()
	if err != nil {
		l.Logger.Error("failed to get fish sessions", "err", err)
		return
	}

	l.Logger.Print(SepLine)
	l.Logger.Info("[Last Fish Sessions]:")
	for _, s := range sessions {
		running := s.Running
		if running == "" {
			running = "(idle)"
		}
		fmt.Printf("PID %-6d  TTY %-12s  CWD %-30s  CMD %s\n", s.PID, s.TTY, s.CWD, running)
	}
}


func findShellProcesses(shellName string) ([]int, error) {
    entries, _ := os.ReadDir("/proc")
    var pids []int
    for _, e := range entries {
        pid, err := strconv.Atoi(e.Name())
        if err != nil { continue }
        comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
        if err != nil { continue }
        if strings.TrimSpace(string(comm)) == shellName {
            pids = append(pids, pid)
        }
    }
    return pids, nil
}

type FishSession struct {
    PID        int
    CWD        string
    Running    string // what fish is currently running, empty if idle
    TTY        string
}

func getFishSessions() ([]FishSession, error) {
    pids, err := findShellProcesses("fish")
    if err != nil {
        return nil, err
    }

    var sessions []FishSession
    for _, pid := range pids {
        session := FishSession{PID: pid}

        // CWD
        cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
        if err == nil {
            session.CWD = cwd
        }

        // TTY - identifies which terminal window this is
        tty, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/0", pid))
        if err == nil {
            session.TTY = tty
        }

        // Find child processes - what fish is actually running right now
        session.Running = getChildCommand(pid)

        sessions = append(sessions, session)
    }
    return sessions, nil
}

func getChildCommand(parentPID int) string {
    entries, err := os.ReadDir("/proc")
    if err != nil {
        return ""
    }

    for _, e := range entries {
        pid, err := strconv.Atoi(e.Name())
        if err != nil {
            continue
        }

        // Read this process's parent PID
        statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
        if err != nil {
            continue
        }

        // stat format: pid (comm) state ppid ...
        // we need the ppid which is after the closing paren
        stat := string(statData)
        closeParen := strings.LastIndex(stat, ")")
        if closeParen == -1 {
            continue
        }
        fields := strings.Fields(stat[closeParen+2:])
        if len(fields) == 0 {
            continue
        }
        ppid, err := strconv.Atoi(fields[0]) // first field after ") state" is ppid
        if err != nil || ppid != parentPID {
            continue
        }

        // This process's parent is fish, read what it is
        cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
        if err != nil {
            continue
        }
        // cmdline is null-separated
        cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
        return strings.TrimSpace(cmd)
    }

    return "" // fish is idle, waiting for input
}

func getLast5FishCommands() string {
	path := ch.ParseFileLoc(".local/share/fish/fish_history", true)
	return ch.ExecCommand("cat", path, " | tail -n 15")
}

