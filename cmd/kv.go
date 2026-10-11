/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"barbtils/internal/encryption"
	l "barbtils/internal/logger"
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// kvCmd represents the kv command
var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 || len(args) == 0 {
			l.Fatal("Key must be a single string")
		}
		pairToAppend(args[0])
	},
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 || len(args) == 0 {
			l.Fatal("Key must be a single string")
		}
		readKvFile(args[0])
	},
}

func init() {
	RootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(addCmd)
	kvCmd.AddCommand(getCmd)
}

func readFullKvFile(s string) {

	f, err := os.OpenFile("kv.txt", os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	for scannerFile.Scan() {
		// scanner.Text() returns the current line as a string (excluding \n)
		line := scannerFile.Text()
		fmt.Println(line)
	}

	// 5. Check for errors that occurred during scanning
	if err := scannerFile.Err(); err != nil {
		log.Fatalf("error during scanning: %s", err)
	}

}

func readKvFile(input string) {

	input = strings.TrimSpace(input)

	f, err := os.OpenFile("kv.txt", os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	for scannerFile.Scan() {
		rawLine := strings.TrimSpace(scannerFile.Text())

		lines, err := encryption.Decrypt([]byte(rawLine))
		if err != nil {
			fmt.Errorf("Could not Decryupt: %v", err)
		}

		if strings.HasPrefix(string(lines), input) {
			after, _ := strings.CutPrefix(string(lines), input+":")
			fmt.Printf("%v\n", after)
		}
	}

	if err := scannerFile.Err(); err != nil {
		log.Fatalf("error during scanning: %s", err)
	}

}

func writeKvPair(keyInput string) string {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})

	var valueInput string
	scannerInput := bufio.NewScanner(os.Stdin)
	scannerInput.Split(bufio.ScanLines)
	go func() {
		l.Printf("Enter the value for the key of %s: ", keyInput)

		for scannerInput.Scan() {
			valueInput = scannerInput.Text()
			close(done)
		}

		if err := scannerInput.Err(); err != nil {
			l.Errorf("Error reading input: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\nReceived shutdown signal, exiting...")
		os.Stdin.Close()
		os.Exit(1)
	case <-done:
		fmt.Println("Scanner finished reading input.")
	}

	kvPairParsed := fmt.Sprintf("%s:%s", keyInput, valueInput)

	return kvPairParsed
}

func pairToAppend(s string) {

	kvPair := writeKvPair(s)

	f, err := os.OpenFile("kv.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewWriter(f)

	byte, err := encryption.Encrypt([]byte(kvPair))
	if err != nil {
		fmt.Errorf("Encryption err: %v\n", err)
	}

	_, err = scannerFile.WriteString(string(byte) + "\n")
	if err != nil {
		fmt.Errorf("err: %v\n", err)
	}

	err = scannerFile.Flush()
	if err != nil {
		fmt.Errorf("Error flushing buffer:", err)
		return
	}
}
