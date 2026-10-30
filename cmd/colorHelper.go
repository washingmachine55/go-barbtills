/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	colorkwingt "barbtils/internal/colorKwingT"
	"fmt"

	"github.com/spf13/cobra"
)

// colorHelperCmd represents the colorHelper command
var colorHelperCmd = &cobra.Command{
	Use:     "colorHelper",
	Aliases: []string{"ch", "color"},
	Short:   "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// kvCmd represents the kv command
var getColorCmd = &cobra.Command{
	Use:   "name",
	Short: "get color name from hex",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(colorkwingt.GetColorNameFromHexCode(args[0]))
	},
}

func init() {
	RootCmd.AddCommand(colorHelperCmd)
	colorHelperCmd.AddCommand(getColorCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// colorHelperCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// colorHelperCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
