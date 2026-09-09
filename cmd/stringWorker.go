package cmd

import (
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var stringWorker = &cobra.Command{
	Use:   "bst",
	Short: "Bulk String Time Handler",
	Long: `Handles time calculation from a bulk of string.
Essentially takes in raw, or formatted input to calculate time as an output.`,
	Run: func(cmd *cobra.Command, args []string) {
		pretty, _ := cmd.Flags().GetBool("pretty")
		if message != "" && webshit == false {
			l.Logger.Debug("Starting Text Process")
			process := processText(message)
			total := calculateTotalSeconds(process)
			if pretty {
				formatResult(total, true)
				l.Logger.Debug("\nFinished Text Process")
			} else {
				formatResult(total, false)
				l.Logger.Debug("Finished Text Process")
			}
		}
		if message != "" && webshit == true {
			l.Logger.Debug("Starting Text Process")
			total := calculateTotalSeconds(message)
			if pretty {
				formatResult(total, true)
			} else {
				formatResult(total, false)
			}
			l.Logger.Debug("Finished Text Process")
		}
	},
}

var message string
var webshit bool
var pretty bool

func init() {
	RootCmd.AddCommand(stringWorker)
	stringWorker.Flags().StringVarP(&message, "message", "m", "", "Text to infer from")
	stringWorker.Flags().BoolVarP(&webshit, "formatted", "f", false, "Use this if your text is preformatted")
	stringWorker.Flags().BoolVarP(&pretty, "pretty", "p", false, "Use this to get a main pretty output")
}

func processText(message string) string {
	hoursAndMinsReGex, err := regexp.Compile(`(\d\d:\d\d)`)
	if err != nil {
		l.Logger.Fatal(err)
	}

	matches := hoursAndMinsReGex.FindAllString(message, -1)
	l.Logger.Debug("[MATCHES LENGTH]", "Matches Arr", len(matches))
	l.Logger.Debug("[PROCESSED TEXT]", "Matches Arr", matches)

	str := strings.Join(matches, "m + ")
	str = fmt.Sprint(str + "m")
	str = strings.ReplaceAll(str, ":", "h ")

	l.Logger.Debug("New String", "str", str)

	return str
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
		case "d":
			seconds = val * 86400
		case "h":
			seconds = val * 3600
		case "m":
			seconds = val * 60
		case "s":
			seconds = val
		}

		totalSeconds += (seconds * currentOp)
	}
	return totalSeconds
}

// bstResult is the --json shape of a bst calculation. The float fields keep
// full precision rather than the 3-6 decimals the table prints, so a consumer
// can round to whatever it needs.
type bstResult struct {
	TotalSeconds int64        `json:"total_seconds"`
	Formatted    string       `json:"formatted"`
	Breakdown    bstBreakdown `json:"breakdown"`

	AsDays    float64 `json:"as_days"`
	AsHours   float64 `json:"as_hours"`
	AsMinutes float64 `json:"as_minutes"`
}

type bstBreakdown struct {
	Days    int64 `json:"days"`
	Hours   int64 `json:"hours"`
	Minutes int64 `json:"minutes"`
	Seconds int64 `json:"seconds"`
}

func formatResult(totalSec int64, pretty bool) {
	d := totalSec / 86400
	remainder := totalSec % 86400
	h := remainder / 3600
	remainder %= 3600
	m := remainder / 60
	s := remainder % 60

	// --json wins over --pretty: both ask for one machine-readable line, and the
	// JSON document is the more useful of the two.
	if jsonEnabled {
		if err := emitJSON(bstResult{
			TotalSeconds: totalSec,
			Formatted:    fmt.Sprintf("%dd %dh %dm %ds", d, h, m, s),
			Breakdown:    bstBreakdown{Days: d, Hours: h, Minutes: m, Seconds: s},
			AsDays:       float64(totalSec) / 86400.0,
			AsHours:      float64(totalSec) / 3600.0,
			AsMinutes:    float64(totalSec) / 60.0,
		}); err != nil {
			l.Logger.Fatal("Error while writing JSON output", "Error", err)
		}
		return
	}

	if !pretty {
		// The %10 widths align the unit column; they are presentation only, which
		// is why the JSON branch above carries numbers instead of these strings.
		l.Logger.Printf("= %dd %dh %dm %ds", d, h, m, s)
		l.Logger.Printf("= %10.6f d", float64(totalSec)/86400.0)
		l.Logger.Printf("= %10.5f h", float64(totalSec)/3600.0)
		l.Logger.Printf("= %10.3f m", float64(totalSec)/60.0)
		l.Logger.Printf("= %10d s", totalSec)
	} else {
		pewp := fmt.Sprintf("%dd %dh %dm %ds\n", d, h, m, s)
		// fmt.Printf("%dd %dh %dm %ds", d, h, m, s)
		fmt.Fprint(os.Stdout, pewp)
	}
}
