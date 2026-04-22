/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"barbtils/cmd/context"
	ch "barbtils/internal/cmdHelper"
	l "barbtils/internal/logger"
	"fmt"

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
			context.GatherDetails()
		}
		if len(command) != 0 {
			l.Debug("[COMMANDS ARR]", "command", command)
			// ch.ExecCommand(command[0], command[1], command[2])
			res := ch.ExecCommand(command[0], command[1], command[2])
			fmt.Print(res)
		}
	},
}

var command []string;

func init() {
	RootCmd.AddCommand(ContextCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// contextCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	ContextCmd.Flags().BoolP("new", "n", false, "Create new context for the current session")
	ContextCmd.Flags().StringSliceVarP(&command, "command", "c", command, "Run a command")
	// contextCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
