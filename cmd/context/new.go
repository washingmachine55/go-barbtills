package context

import (
	ch "barbtils/internal/cmdHelper"
	"barbtils/internal/logger"
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GatherDetails() {
	l.Logger.Debugf("[CWD]: %v\n", ch.GetCWD())
	l.Logger.Info("[Last ~10 Fish Commands]:")
	getLast5FishCommands()

	sessions, err := getFishSessions()
	if err != nil {
		l.Logger.Error("failed to get fish sessions", "err", err)
		return
	}

	l.Logger.Print(l.SepLine)
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
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) == shellName {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

type FishSession struct {
	PID     int
	CWD     string
	Running string // what fish is currently running, empty if idle
	TTY     string
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

func getLast5FishCommands() {
	path := ch.ParseFileLoc(".local/share/fish/fish_history", true)
	history := ch.ExecCommand("cat", path, " | awk '/paths/ {getline; next} 1' | tail -n 4")
	rr, err := regexp.Compile(`(\d{10})`)
	if err != nil {
		logger.Fatalf("Could not compile regex")
	}

	u := func(s string) string {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			logger.Fatalf("Could not convert regex string to int")
		}
		tt := time.Unix(i, i)
		return tt.Format("Mon, 2006-01-02 @ 03:04:05 PM")
	}

	result := rr.ReplaceAllStringFunc(
		history,
		u,
	)

	// =======================================================
	// 					Text Manipulation
	// =======================================================
	commands := strings.Split(result, "- cmd: ")

	for i := range len(commands) {
		// if i == len(commands)-1 {
		// 	break
		// }

		l.Print(commands[i])
		// command := strings.Split(commands[i], "when: ")

		// l.Print(command)

		l.Logger.Infof(
			"Command #%d",
			i)

		// fmt.Printf(
		// 	"Command: %-40s  Executed at: %-30s  \n",
		// 	strings.TrimSpace(command[0]),
		// 	strings.TrimSpace(command[1]),
		// )

	}

}
