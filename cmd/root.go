/*
Copyright © 2026 Ahmed Babar
*/
package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var (
	asciiArt string
	asciiOpts string
	asciiOptsHelp bool
) 
var DB *sql.DB

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "tc",
	Short: "Calculate Time using Strings",
	Long: 
		`A longer description that spans multiple lines and likely contains
		examples and usage of using your application. For example:

		Cobra is a CLI library for Go that empowers applications.
		This application is a tool to generate the needed files
		to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if asciiArt != "" {
			Logger.Debugf("Used Opts: %s", asciiOpts)
			asciiArting(asciiArt, asciiOpts)
			// Logger.Warnf("Output: %s", m)
		}
		if asciiOptsHelp != false {
			Logger.Info("Available options are: ")
			printFontTable()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(loggerInit)
	// cobra.OnInitialize(initDB)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gocli.yaml)")
	RootCmd.PersistentFlags().StringVar(&asciiArt, "ass", "", "ASCII art YEEEETTT")
	RootCmd.Flags().StringVarP(&asciiOpts, "opts", "o", "slant", "Options for ASCII art")
	RootCmd.Flags().BoolVar(&asciiOptsHelp, "opts-help", false, "ASCII art YEEEETTT")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
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
		viper.AddConfigPath(home+"/.config/time-calc")
		viper.SetConfigType("json")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initDB() error {
	var (
		db_url string = viper.GetString("DB_URL")
	)

	DB, err := sql.Open("postgres", db_url)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	
	defer func() {
		if err := DB.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	err = DB.Ping()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	
	fmt.Println("✓ Successfully connected to PostgreSQL database")
	return nil
}

var availableFonts = [...]string{"3-d", "3x5", "5lineoblique", "acrobatic", "alligator", "alligator2", "alphabet", "avatar", "banner", "banner3-D", "banner3", "banner4", "barbwire", "basic", "bell", "big", "bigchief", "binary", "block", "bubble", "bulbhead", "calgphy2", "caligraphy", "catwalk", "chunky", "coinstak", "colossal", "computer", "contessa", "contrast", "cosmic", "cosmike", "cricket", "cursive", "cyberlarge", "cybermedium", "cybersmall", "diamond", "digital", "doh", "doom", "dotmatrix", "drpepper", "eftichess", "eftifont", "eftipiti", "eftirobot", "eftitalic", "eftiwall", "eftiwater", "epic", "fender", "fourtops", "fuzzy", "goofy", "gothic", "graffiti", "hollywood", "invita", "isometric1", "isometric2", "isometric3", "isometric4", "italic", "ivrit", "jazmine", "jerusalem", "katakana", "kban", "larry3d", "lcd", "lean", "letters", "linux", "lockergnome", "madrid", "marquee", "maxfour", "mike", "mini", "mirror", "mnemonic", "morse", "moscow", "nancyj-fancy", "nancyj-underlined", "nancyj", "nipples", "ntgreek", "o8", "ogre", "pawp", "peaks", "pebbles", "pepper", "poison", "puffy", "pyramid", "rectangles", "relief", "relief2", "rev", "roman", "rot13", "rounded", "rowancap", "rozzo", "runic", "runyc", "sblood", "script", "serifcap", "shadow", "short", "slant", "slide", "slscript", "small", "smisome1", "smkeyboard", "smscript", "smshadow", "smslant", "smtengwar", "speed", "stampatello", "standard", "starwars", "stellar", "stop", "straight", "tanja", "tengwar", "term", "thick", "thin", "threepoint", "ticks", "ticksslant", "tinker-toy", "tombstone", "trek", "tsalagi", "twopoint", "univers", "usaflag", "wavy", "weird"}

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