/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	l "barbtils/internal/logger"
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// asciiCmd represents the ascii command
var asciiCmd = &cobra.Command{
	Use:   "ascii",
	Short: "Outputs ASCII Art to 'stdout' based on available fonts",
	Run: func(cmd *cobra.Command, args []string) {
		if asciiArt != "" && asciiArt != "-" {
			l.Logger.Debugf("Used Opts: %s", asciiOpts)
			asciiArting(asciiArt, asciiOpts)
			// l.Logger.Warnf("Output: %s", m)
		}
		if asciiOptsHelp != false {
			l.Logger.Info("Available options are: ")
			printFontTable()
		}
		val := asciiArt
		if val == "-" {
			// Read from Stdin
			reader := bufio.NewReader(os.Stdin)
			content, _ := io.ReadAll(reader)
			val = string(content)
			l.Logger.Debug(os.Stderr, "DEBUG: raw input received: %q\n", val)
			// Process 'val'
			asciiArting(strings.TrimSpace(val), asciiOpts)
		}
	},
}

func init() {
	RootCmd.AddCommand(asciiCmd)
	asciiCmd.Flags().StringVar(&asciiArt, "message", "", "String to convert to ASCII Art (use '-' for stdin if piping into this)")
	asciiCmd.Flags().StringVarP(&asciiOpts, "opts", "o", "slant", "Font Option to use for ASCII art generation")
	asciiCmd.Flags().BoolVar(&asciiOptsHelp, "opts-help", false, "Prints all available options for ASCII art fonts")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// asciiCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// asciiCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	asciiCmd.RegisterFlagCompletionFunc("opts", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return availableFonts, cobra.ShellCompDirectiveNoFileComp
	})
}
