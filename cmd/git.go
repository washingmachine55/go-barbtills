/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	cmdHelper "barbtils/internal/cmdHelper"
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// gitCmd represents the git command
var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if create, err := cmd.Flags().GetString("create"); err == nil && create != "" {
			l.Info(string(ReadOrCreateStorageFile(create)))
		}
		if collect, err := cmd.Flags().GetString("collect"); err == nil && collect != "" {
			WriteGitShit(collect)
		}
		if status, err := cmd.Flags().GetBool("status"); err == nil && status != false {
			if cmd, err := cmd.Flags().GetString("cmd"); err == nil && cmd != "" {
				runGitCmd(cmd)
			} else {
				runGitStatus()
			}
		}
	},
}

func init() {
	ContextCmd.AddCommand(gitCmd)

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// gitCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// gitCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	gitCmd.Flags().String("create", "", "Create storage file for gitshit later :3")
	gitCmd.Flags().StringP("collect", "c", "", "Collect local git repo dir to do stuff to it later :3")
	gitCmd.Flags().BoolP("status", "s", false, "run git status")
	gitCmd.Flags().String("cmd", "", "run custom git command")
}

func runGitCmd(cmd string) {
	var sf []byte = ReadOrCreateStorageFile(DefaultStoragePath + DefaultStorageFileName)
	data := strings.Split(string(sf), "\n")
	for i := range len(data)-1 {
		// git --git-dir /home/hmed42/Work/3-week-plan/.git --work-tree /home/hmed42/Work/3-week-plan/ status
		compose := fmt.Sprintf("--git-dir %s.git --work-tree %s", data[i],  data[i])
		res := cmdHelper.ExecCommand("git", compose, cmd)
		l.Logger.Info("[Results]", "for", data[i])
		fmt.Fprintln(os.Stdout, fmt.Sprint(res))
	}
}
func runGitStatus() {
	var sf []byte = ReadOrCreateStorageFile(DefaultStoragePath + DefaultStorageFileName)
	data := strings.Split(string(sf), "\n")
	for i := range len(data)-1 {
		// git --git-dir /home/hmed42/Work/3-week-plan/.git --work-tree /home/hmed42/Work/3-week-plan/ status
		compose := fmt.Sprintf("--git-dir %s.git --work-tree %s", data[i],  data[i])
		res := cmdHelper.ExecCommand("git", compose, "status --porcelain")
		l.Logger.Info("[Results]", "for", data[i])
		fmt.Fprintln(os.Stdout, fmt.Sprint(res))
	}
}

func WriteGitShit(filePath string) {
	dirs, err := os.ReadDir(filePath)
	if err != nil {
		l.Error("Error while reading given dir", "Error", err)
	}
	l.Debugf("dirs: %v\n", dirs)

	if hasGit(dirs) {
		data, err := os.ReadFile(DefaultStoragePath+DefaultStorageFileName)
		currentFile := strings.Split(string(data), "\n")
		for i := range currentFile {
			if currentFile[i] == filePath {
				l.Fatalf("That Git repo (%s) is already added to the storage file!", filePath)
			}
		}

		f, err := os.OpenFile(DefaultStoragePath+DefaultStorageFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			l.Fatal("Error while trying to open storage file", "Error", err)
		}
		defer f.Close()

		if _, err := f.WriteString(filePath+"\n"); err != nil {
			l.Fatal("Error while trying to write to storage file", "Error", err)
		}
	} else {
		l.Fatal("Error found while trying to find git repo")
	}
}

func hasGit(dirs []os.DirEntry) bool {
	for i := range dirs {
		if dirs[i].Name() == ".git" {
			return true
		} 
	}
	return false
}


func ReadOrCreateStorageFile(filePath string) []byte {
	texo, err := os.ReadFile(filePath)
	if err != nil {
		l.Logger.Warn("Error while trying to read file", "Error", err)
		l.Logger.Debug("Proceeding to create a file...")
			
		tex, erar := os.Create(DefaultStoragePath+DefaultStorageFileName)
		if erar != nil {
			l.Logger.Warn("Error while trying to create file", "Error", erar)
			l.Logger.Debug("Proceeding to create a directory...")

			errMkDir := os.MkdirAll(DefaultStoragePath, 0755)
			if errMkDir != nil {
				l.Logger.Fatal("Error while trying to create a directory", "Error", errMkDir)
			}
		}
		tex.Close()
	}
	return texo
}