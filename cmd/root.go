/*
Copyright © 2026 Ahmed Babar
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const APP_VERSION string = "1.5"

var cfgFile string
var (
	asciiArt      string
	asciiOpts     string
	asciiOptsHelp bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "barbtils",
	Short: "My utils that I sorta need on a usual basis",
	Long: `I am retarded, and my unconventional ways require me to make shit like this
so that I can stay alive as a functional human being.
		
Use this shit at your own risk lol.`,
	Run: func(cmd *cobra.Command, args []string) {
		if asciiArt != "" && asciiArt != "-" {
			Logger.Debugf("Used Opts: %s", asciiOpts)
			asciiArting(asciiArt, asciiOpts)
			// Logger.Warnf("Output: %s", m)
		}
		if asciiOptsHelp != false {
			Logger.Info("Available options are: ")
			printFontTable()
		}
		val := asciiArt
		if val == "-" {
			// Read from Stdin
			reader := bufio.NewReader(os.Stdin)
			content, _ := io.ReadAll(reader)
			val = string(content)
			Logger.Debug(os.Stderr, "DEBUG: raw input received: %q\n", val)
			// Process 'val'
			asciiArting(strings.TrimSpace(val), asciiOpts)
		}
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			LoggerSetLevelDebug()
			Logger.Debug("Logger Set to Debug")
		}
		av, _ := cmd.Flags().GetBool("version")
		if av {
			Logger.Info("[CURRENT VERSION]", "barbtils", APP_VERSION)
		}
		// Only the root command (no subcommand) inherits this timeout; avoids killing `tasks` and others.
		if cmd.Parent() != nil {
			return
		}
		streamC, _ := streamTextCmd.Flags().GetBool("serve");
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
	cobra.OnInitialize(loggerInit)
	cobra.OnInitialize(initConfig)
	// cobra.OnInitialize(initDB)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "$HOME/.config/barbtils/config.toml", "config file path")
	RootCmd.PersistentFlags().BoolP("debug", "d", false, "Set Log level to debug")
	RootCmd.PersistentFlags().BoolP("version", "v", false, "Print app version")

	RootCmd.Flags().StringVar(&asciiArt, "ass", "", "ASCII art YEEEETTT. (use '-' for stdin)")
	RootCmd.Flags().StringVarP(&asciiOpts, "opts", "o", "slant", "Options for ASCII art")
	RootCmd.Flags().BoolVar(&asciiOptsHelp, "opts-help", false, "Prints all available options for ASCII art fonts")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	RootCmd.RegisterFlagCompletionFunc("opts", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return availableFonts, cobra.ShellCompDirectiveNoFileComp
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		//TODO
		Logger.Debug("Have to create something something that outputs the example json file.")
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.SetConfigName("config")
		viper.SetConfigType("toml")
		viper.AddConfigPath(home + "/.config/barbtils")
	}

	// viper.AutomaticEnv() // read in environment variables that match

	// // If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		// fmt.Fprintln(os.Stderr, "No config file found", viper.ConfigFileUsed())
		Logger.Debugf("No config file found @ %s", viper.ConfigFileUsed())
	} else {
		Logger.Debug("[Config]", "Using config file @", viper.ConfigFileUsed())
		//TODO - The message above does not show due to the lifecycle of the program. 
		// Logger level starts at info, if the user start the program with the debug flag, it will initialize logger, run this function and only then set logger level, which is why this debug never runs
	}
}

var availableFonts = []string{"3-d", "3x5", "5lineoblique", "acrobatic", "alligator", "alligator2", "alphabet", "avatar", "banner", "banner3-D", "banner3", "banner4", "barbwire", "basic", "bell", "big", "bigchief", "binary", "block", "bubble", "bulbhead", "calgphy2", "caligraphy", "catwalk", "chunky", "coinstak", "colossal", "computer", "contessa", "contrast", "cosmic", "cosmike", "cricket", "cursive", "cyberlarge", "cybermedium", "cybersmall", "diamond", "digital", "doh", "doom", "dotmatrix", "drpepper", "eftichess", "eftifont", "eftipiti", "eftirobot", "eftitalic", "eftiwall", "eftiwater", "epic", "fender", "fourtops", "fuzzy", "goofy", "gothic", "graffiti", "hollywood", "invita", "isometric1", "isometric2", "isometric3", "isometric4", "italic", "ivrit", "jazmine", "jerusalem", "katakana", "kban", "larry3d", "lcd", "lean", "letters", "linux", "lockergnome", "madrid", "marquee", "maxfour", "mike", "mini", "mirror", "mnemonic", "morse", "moscow", "nancyj-fancy", "nancyj-underlined", "nancyj", "nipples", "ntgreek", "o8", "ogre", "pawp", "peaks", "pebbles", "pepper", "poison", "puffy", "pyramid", "rectangles", "relief", "relief2", "rev", "roman", "rot13", "rounded", "rowancap", "rozzo", "runic", "runyc", "sblood", "script", "serifcap", "shadow", "short", "slant", "slide", "slscript", "small", "smisome1", "smkeyboard", "smscript", "smshadow", "smslant", "smtengwar", "speed", "stampatello", "standard", "starwars", "stellar", "stop", "straight", "tanja", "tengwar", "term", "thick", "thin", "threepoint", "ticks", "ticksslant", "tinker-toy", "tombstone", "trek", "tsalagi", "twopoint", "univers", "usaflag", "wavy", "weird"}

func asciiArting(m string, font string) {

	if m == "" {
		Logger.Fatal("Can't proceed with Nil Chars")
	}

	fontSet := make(map[string]struct{}) // Use an empty struct{} for memory efficiency

	for _, availableFont := range availableFonts {
		fontSet[availableFont] = struct{}{}
	}

	// Check for presence using the map
	_, found := fontSet[font]
	if found == false {
		Logger.Fatalf("Selected font '%s' is not available in the Font List", font)
	}
	myFigure := figure.NewFigure(m, font, true)
	wolu := myFigure.ColorString()
	color.RGB(255, 128, 0).Printf("%s", wolu)
}

func printFontTable() {
	// minwidth, tabwidth, padding, padchar, flags
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Println("AVAILABLE FONTS:")
	fmt.Println("----------------")

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
