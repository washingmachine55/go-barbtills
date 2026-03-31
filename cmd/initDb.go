/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

// initDbCmd represents the initDb command
var initDbCmd = &cobra.Command{
	Use:   "initDb",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. `,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("initDb called")
			initDB()
	},
}

func init() {
	RootCmd.AddCommand(initDbCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initDbCmd.PersistentFlags().String("foo", "", "A help for foo")
	initDbCmd.PersistentFlags().Bool("f", false, "Initialize Db for the first time")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initDbCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
