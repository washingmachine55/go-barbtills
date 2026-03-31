/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"regexp"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var stringWorker = &cobra.Command{
	Use:   "bs",
	Short: "Bulk String Handler",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. `,
	Run: func(cmd *cobra.Command, args []string) {
		if message != "" && webshit == false {
			Logger.Info("Starting Text Process")
			finalHours, finalMins := processText(message)
			Logger.Warn("", "Hours", finalHours, "Mins", finalMins)
			Logger.Info("Finished Text Process")
		}
		if message != "" && webshit == true {
			Logger.Info("Starting Text Process")
			total := calculateTotalSeconds(message)
			formatResult(total)
			Logger.Info("Finished Text Process")
		}
	},
}

var message string
var webshit bool

func init() {
	RootCmd.AddCommand(stringWorker)
	stringWorker.Flags().StringVarP(&message, "message", "m", "", "Text to infer from")
	stringWorker.Flags().BoolVarP(&webshit, "formatted", "f", false, "Text to infer from")

}

func processText(message string) (int, int) {
	
	lines := strings.Split(strings.TrimSpace(message), "\n")
        var totalMinutes int

        for _, line := range lines {
            // Split by comma
            parts := strings.Split(line, ",")
            if len(parts) < 5 {
                continue
            }

            // Get the last part (the time) and trim whitespace
            timeStr := strings.TrimSpace(parts[len(parts)-1]) // e.g., "00:22"
            
            // Split HH:MM
            t := strings.Split(timeStr, ":")
            if len(t) == 2 {
                hours, _ := strconv.Atoi(t[0])
                mins, _ := strconv.Atoi(t[1])
                totalMinutes += (hours * 60) + mins
            }
        }

        // Convert back to your specific 99H 60M format if needed
        finalHours := totalMinutes / 60
        finalMins := totalMinutes % 60
	return finalHours, finalMins
}

// calculateTotalSeconds takes a string like "1d 2h + 4h 5s - 2030s"
func calculateTotalSeconds(input string) int64 {
	// Regular expression to find blocks like "1d", "2030s", "+", "-"
	re := regexp.MustCompile(`([+-])|(\d+)([dhms])`)
	matches := re.FindAllStringSubmatch(input, -1)

	var totalSeconds int64
	currentOp := int64(1) // Start with addition by default

	for _, match := range matches {
		if match[1] != "" { // It's an operator
			if match[1] == "-" {
				currentOp = -1
			} else {
				currentOp = 1
			}
			continue
		}

		// It's a value + unit
		val, _ := strconv.ParseInt(match[2], 10, 64)
		unit := match[3]

		var seconds int64
		switch unit {
		case "d": seconds = val * 86400
		case "h": seconds = val * 3600
		case "m": seconds = val * 60
		case "s": seconds = val
		}

		totalSeconds += (seconds * currentOp)
	}
	return totalSeconds
}

func formatResult(totalSec int64) {
	d := totalSec / 86400
	remainder := totalSec % 86400
	h := remainder / 3600
	remainder %= 3600
	m := remainder / 60
	s := remainder % 60

	// Output exactly like your screenshot
	Logger.Printf("= %dd %dh %dm %ds", d, h, m, s)
	Logger.Printf("= %10.6f d", float64(totalSec)/86400.0)
	Logger.Printf("= %10.5f h", float64(totalSec)/3600.0)
	Logger.Printf("= %10.3f m", float64(totalSec)/60.0)
	Logger.Printf("= %10d s", totalSec)
}