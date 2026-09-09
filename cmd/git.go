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
	Short: "Get context on your locally saved git repos",
	Long: `You can use this command to save git repo direcotires to a file and then
run commands on them. For example:

"barbtils context git -s" would run git status on all of the git repos that you save using:
"barbtils context git --collect $HOME/example-git-repo/"`,
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
	for i := range len(data) - 1 {
		// git --git-dir /home/hmed42/Work/3-week-plan/.git --work-tree /home/hmed42/Work/3-week-plan/ status
		compose := fmt.Sprintf("--git-dir %s.git --work-tree %s", data[i], data[i])
		res := cmdHelper.ExecCommand("git", compose, cmd)
		l.Logger.Info("[Results]", "for", data[i])
		fmt.Fprintln(os.Stdout, fmt.Sprint(res))
	}
}

// gitRepoStatus is one repo's porcelain status, grouped by what happened to
// each file.
type gitRepoStatus struct {
	Path      string   `json:"path"`
	RepoName  string   `json:"repo_name"`
	Modified  []string `json:"modified"`
	Untracked []string `json:"untracked"`
	Added     []string `json:"added"`
	Deleted   []string `json:"deleted"`
}

func runGitStatus() {
	var sf []byte = ReadOrCreateStorageFile(DefaultStoragePath + DefaultStorageFileName)
	data := strings.Split(string(sf), "\n")
	repos := make([]gitRepoStatus, 0, max(len(data)-1, 0))
	for i := range len(data) - 1 {
		compose := fmt.Sprintf("--git-dir %s.git --work-tree %s", data[i], data[i])
		res := cmdHelper.ExecCommand("git", compose, "status --porcelain")
		repos = append(repos, parseGitStatus(data[i], res))
	}

	// JSON goes to stdout as one document, so `barbtils context git -s -j | jq`
	// needs no 2>&1 and never has a log line spliced into it.
	if jsonEnabled {
		if err := emitJSON(map[string]any{
			"repos": repos,
			"count": len(repos),
		}); err != nil {
			l.Logger.Fatal("Error while writing JSON output", "Error", err)
		}
		return
	}

	for _, r := range repos {
		l.Logger.Info(
			"[Results]",
			"path", r.Path,
			"repo_name", r.RepoName,
			"modified", r.Modified,
			"untracked", r.Untracked,
			"added", r.Added,
			"deleted", r.Deleted,
		)
	}
}

// parseGitStatus groups the lines of `git status --porcelain` by change type.
// Empty groups stay as [], so a JSON consumer can iterate them unconditionally.
func parseGitStatus(path, porcelain string) gitRepoStatus {
	repoName := path
	if dirs := strings.Split(path, "/"); len(dirs) >= 2 {
		repoName = dirs[len(dirs)-2]
	}

	status := gitRepoStatus{
		Path:      path,
		RepoName:  repoName,
		Modified:  []string{},
		Untracked: []string{},
		Added:     []string{},
		Deleted:   []string{},
	}
	for _, line := range strings.Split(porcelain, "\n") {
		entry := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(entry, "??"):
			status.Untracked = append(status.Untracked, strings.Trim(entry, "?? "))
		case strings.HasPrefix(entry, "M"):
			status.Modified = append(status.Modified, strings.Trim(entry, " M "))
		case strings.HasPrefix(entry, "A"):
			status.Added = append(status.Added, strings.Trim(entry, " A "))
		case strings.HasPrefix(entry, "D"):
			status.Deleted = append(status.Deleted, strings.Trim(entry, " D "))
		}
	}
	return status
}

func WriteGitShit(filePath string) {
	l.Debug("Given File path", filePath)
	checkTrailingSlash := strings.HasSuffix(filePath, "/")
	if !checkTrailingSlash {
		filePath = filePath + "/"
		l.Debug("Edited file path", filePath)
	}

	dirs, err := os.ReadDir(filePath)
	if err != nil {
		l.Error("Error while reading given dir", "Error", err)
	}
	l.Debugf("dirs: %v\n", dirs)

	if hasGit(dirs) {
		data, err := os.ReadFile(DefaultStoragePath + DefaultStorageFileName)
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

		if _, err := f.WriteString(filePath + "\n"); err != nil {
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
	storageTextFile, err := os.ReadFile(filePath)
	if err != nil {
		l.Logger.Warn("Error while trying to read file", "Error", err)
		l.Logger.Debug("Proceeding to create a file...")

		createStorageTextFile, err := os.Create(DefaultStoragePath + DefaultStorageFileName)
		if err != nil {
			l.Logger.Warn("Error while trying to create file", "Error", err)
			l.Logger.Debug("Proceeding to create a directory...")

			errMkDir := os.MkdirAll(DefaultStoragePath, 0755)
			if errMkDir != nil {
				l.Logger.Fatal("Error while trying to create a directory", "Error", errMkDir)
			}
		}
		createStorageTextFile.Close()
	}
	return storageTextFile
}
