/*
Copyright © 2026 Ahmed Babar
*/
package cmd

import (
	cmdHelper "barbtils/internal/cmdHelper"
	l "barbtils/internal/logger"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const APP_VERSION string = "1.8"

var DefaultConfigPath string = cmdHelper.OSHostName+"/.config/barbtils/config.toml"
var DefaultStoragePath string = cmdHelper.OSHostName+"/.local/share/barbtils/"
const DefaultStorageFileName string = "gitshit"

var cfgFile string = DefaultConfigPath


var (
	asciiArt      string
	asciiOpts     string
	asciiOptsHelp bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "barbtils",
	Short: "My utils that I sorta need on a usual basis",
	Long: `I am a little weird, and my unconventional ways require me to make things like this
so that I can stay alive as a functional human being.
		
Use this at your own risk lol.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			l.LoggerSetLevelDebug()
			l.Logger.Debug("Logger Set to Debug")
		}
		initConfig()
		av, _ := cmd.Flags().GetBool("version")
		if av {
			l.Logger.Info("[CURRENT VERSION]", "barbtils", APP_VERSION)
		}
		// Only the root command (no subcommand) inherits this timeout; avoids killing `tasks` and others.
		if cmd.Parent() != nil {
			return
		}
		streamC, _ := streamTextCmd.Flags().GetBool("serve")
		interactive, _ := tasksCmd.Flags().GetBool("interactive")
		if !interactive || !streamC {
			go func() {
				time.Sleep(5 * time.Second)
				fmt.Fprintln(os.Stderr, "Error: Command timed out!")
				os.Exit(1)
			}()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	signal.Ignore(syscall.SIGPIPE)
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(l.LoggerInit)
	// cobra.OnInitialize(initDB)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", DefaultConfigPath, "config file path")
	RootCmd.PersistentFlags().BoolP("debug", "d", false, "Set Log level to debug. Can be used with any command and subcommands")
	RootCmd.PersistentFlags().BoolP("version", "v", false, "Print app version")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match
	if cfgFile != DefaultConfigPath {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		// TODO
		l.Logger.Debug("Have to create something something that outputs the example json file.")
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
		viper.AddConfigPath(home + "/.config/barbtils/.")
	}

	// // If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		// fmt.Fprintln(os.Stderr, "No config file found", viper.ConfigFileUsed())
		l.Logger.Debugf("No config file found @ %s", viper.ConfigFileUsed())
	} else {
		l.Logger.Debug("[Config]", "Using config file @", viper.ConfigFileUsed())
		// TODO - The message above does not show due to the lifecycle of the program.
		// Logger level starts at info, if the user start the program with the debug flag, it will initialize logger, run this function and only then set logger level, which is why this debug never runs
	}
}

var availableFonts = []string{"3-d", "3x5", "5lineoblique", "acrobatic", "alligator", "alligator2", "alphabet", "avatar", "banner", "banner3-D", "banner3", "banner4", "barbwire", "basic", "bell", "big", "bigchief", "binary", "block", "bubble", "bulbhead", "calgphy2", "caligraphy", "catwalk", "chunky", "coinstak", "colossal", "computer", "contessa", "contrast", "cosmic", "cosmike", "cricket", "cursive", "cyberlarge", "cybermedium", "cybersmall", "diamond", "digital", "doh", "doom", "dotmatrix", "drpepper", "eftichess", "eftifont", "eftipiti", "eftirobot", "eftitalic", "eftiwall", "eftiwater", "epic", "fender", "fourtops", "fuzzy", "goofy", "gothic", "graffiti", "hollywood", "invita", "isometric1", "isometric2", "isometric3", "isometric4", "italic", "ivrit", "jazmine", "jerusalem", "katakana", "kban", "larry3d", "lcd", "lean", "letters", "linux", "lockergnome", "madrid", "marquee", "maxfour", "mike", "mini", "mirror", "mnemonic", "morse", "moscow", "nancyj-fancy", "nancyj-underlined", "nancyj", "nipples", "ntgreek", "o8", "ogre", "pawp", "peaks", "pebbles", "pepper", "poison", "puffy", "pyramid", "rectangles", "relief", "relief2", "rev", "roman", "rot13", "rounded", "rowancap", "rozzo", "runic", "runyc", "sblood", "script", "serifcap", "shadow", "short", "slant", "slide", "slscript", "small", "smisome1", "smkeyboard", "smscript", "smshadow", "smslant", "smtengwar", "speed", "stampatello", "standard", "starwars", "stellar", "stop", "straight", "tanja", "tengwar", "term", "thick", "thin", "threepoint", "ticks", "ticksslant", "tinker-toy", "tombstone", "trek", "tsalagi", "twopoint", "univers", "usaflag", "wavy", "weird"}

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
