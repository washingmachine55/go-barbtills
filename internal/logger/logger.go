package logger

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var Logger *log.Logger

func LoggerInit() {
	Logger = log.NewWithOptions(os.Stderr, log.Options{
		// ReportCaller: true,
		ReportTimestamp: true,
		TimeFormat: "[03:04:05 PM]",
	})
	Logger.SetLevel(log.InfoLevel)
	Logger.SetOutput(os.Stderr)
	

	styles := log.DefaultStyles()
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString("FATAL 💀").
		Padding(0, 1, 0, 1).
		Background(lipgloss.Color("204")).
		Foreground(lipgloss.Color("0"))
	// Add a custom style for key `err`
	styles.Keys["err"] = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Italic(true)
	styles.Values["err"] = lipgloss.NewStyle().Bold(true)
	
	Logger.SetStyles(styles)
}

// Debug prints a debug message.
func Debug(msg any, keyvals ...any) {
	Logger.Debug(msg, keyvals...)
}

// Info prints an info message.
func Info(msg any, keyvals ...any) {
	Logger.Info(msg, keyvals...)
}

// Warn prints a warning message.
func Warn(msg any, keyvals ...any) {
	Logger.Warn(msg, keyvals...)
}

// Error prints an error message.
func Error(msg any, keyvals ...any) {
	Logger.Error(msg, keyvals...)
}

// Fatal prints a fatal message and exits.
func Fatal(msg any, keyvals ...any) {
	Logger.Fatal(msg, keyvals...)
	os.Exit(1)
}

// Print prints a message with no level.
func Print(msg any, keyvals ...any) {
	Logger.Print(msg, keyvals...)
}

// Debugf prints a debug message with formatting.
func Debugf(format string, args ...any) {
	Logger.Debugf(fmt.Sprintf(format, args...))
}

// Infof prints an info message with formatting.
func Infof(format string, args ...any) {
	Logger.Infof(fmt.Sprintf(format, args...))
}

// Warnf prints a warning message with formatting.
func Warnf(format string, args ...any) {
	Logger.Warnf(fmt.Sprintf(format, args...))
}

// Errorf prints an error message with formatting.
func Errorf(format string, args ...any) {
	Logger.Errorf(fmt.Sprintf(format, args...))
}

// Fatalf prints a fatal message with formatting and exits.
func Fatalf(format string, args ...any) {
	Logger.Fatalf(fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Printf prints a message with no level and formatting.
func Printf(format string, args ...any) {
	Logger.Printf(fmt.Sprintf(format, args...))
}