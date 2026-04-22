package internal

import (
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"os/exec"
)

func GetCWD() string {
	dir, err := os.Getwd()
	if err != nil {
		l.Logger.Fatalf("Error trying to read CWD: %v", err)
	}
	return dir
}

var OSHostName string = GetOsHome()

func GetOsHome() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		l.Logger.Fatalf("Error trying to read CWD: %v", err)
	}
	return dir
}


func ParseFileLoc(path string, useHomeDir bool) (string) {
	if useHomeDir {
		homeDir, _ := os.UserHomeDir()
		return fmt.Sprintf(`%s/%s`, homeDir, path)
	} else {
		return path
	}
}

func ExecCommand(progName string, fileLocation string, optionalArgs string) string {
	path, err := exec.LookPath(progName)
	if err != nil {
		l.Logger.Fatal(err)
	}
	l.Logger.Debug("[LookPath]", progName, path)

	fullCommand := fmt.Sprintf("%s %s %s", progName, fileLocation, optionalArgs)

	cmd := exec.Command("sh", "-c", fullCommand)
	
	l.Logger.Debug("[Full Command]", "cmd", cmd)

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		l.Logger.Debugf("Error trying to exec command: %v", err)
	}

	return string(stdoutStderr)
}