/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		Logger.Debug("config called")
		loggerInit()
		Logger.Debug("Logger initiated")
	},
}

var Logger *log.Logger
const sepLine string = "===========================================================\n"

func loggerInit() {
	Logger = log.NewWithOptions(os.Stderr, log.Options{
    	// ReportCaller: true,
		// ReportTimestamp: true,
		// TimeFormat: time.Second.String(),
	})
	Logger.SetLevel(log.DebugLevel)

	styles := log.DefaultStyles()
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString("FATAL 💀").
		Padding(0, 1, 0, 1).
		Background(lipgloss.Color("204")).
		Foreground(lipgloss.Color("0"))
	// Add a custom style for key `err`
	styles.Keys["err"] = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	styles.Values["err"] = lipgloss.NewStyle().Bold(true)
	
	Logger.SetStyles(styles)
}

func init() {
	RootCmd.AddCommand(configCmd)
}
