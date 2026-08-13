/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	l "barbtils/internal/logger"
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
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

func asciiArting(m string, font string) {
	if m == "" {
		l.Logger.Fatal("Can't proceed with Nil Chars")
	}

	fontSet := make(map[string]struct{}) // Use an empty struct{} for memory efficiency

	for _, availableFont := range availableFonts {
		fontSet[availableFont] = struct{}{}
	}

	// Check for presence using the map
	_, found := fontSet[font]
	if found == false {
		l.Logger.Fatalf("Selected font '%s' is not available in the Font List", font)
	}
	myFigure := figure.NewFigure(m, font, true)
	wolu := myFigure.ColorString()
	color.RGB(255, 128, 0).Printf("%s", wolu)
}

func printFontTable() {
	// minwidth, tabwidth, padding, padchar, flags
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// fmt.Println("AVAILABLE FONTS:")
	fmt.Println("-----------------------------------------------------------")

	columns := 4 // You can increase this for wider terminals
	for i, font := range availableFonts {
		fmt.Fprintf(w, "%s\t", font)

		// Start a new line after every N columns
		if (i+1)%columns == 0 {
			fmt.Fprintln(w)
		}
	}

	// Print a final newline if the loop didn't end exactly on a column break
	if len(availableFonts)%columns != 0 {
		fmt.Fprintln(w)
	}

	w.Flush()
}
