/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"barbtils/internal/encryption"
	l "barbtils/internal/logger"
	"bufio"
	"context"
	"errors"
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
	Use:                   "kv",
	Short:                 "Create and consume Key-Value Pairs with AES-GCM Encryption and Decryption",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
		// return fmt.Errorf("unknown subcommand %q", args[0])
	},
}

var addCmd = &cobra.Command{
	Use: "add",
	Short: `Create KV pair by inputting your Key.
	      Its value can be entered by Stdin after the command is invoked`,
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pairToAppend(args[0])
	},
}

var getCmd = &cobra.Command{
	Use:                   "get",
	Short:                 "Get KV pair by inputting your Key",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 || len(args) == 0 {
			// l.Fatal()
			errors.New("Key must be a single string")
		}
		readKvFile(args[0])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Help()
		return errors.New("Key must be a single string")
	},
}

func init() {
	RootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(addCmd)
	kvCmd.AddCommand(getCmd)

	kvCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		_ = command.Flags().MarkHidden("config")     // Replace with your flag's name
		_ = command.Flags().MarkHidden("debug")      // Replace with your flag's name
		_ = command.Flags().MarkHidden("get_db_uri") // Replace with your flag's name
		_ = command.Flags().MarkHidden("json")       // Replace with your flag's name
		_ = command.Flags().MarkHidden("version")    // Replace with your flag's name
		command.Parent().HelpFunc()(command, strings)
	})
}

func initKvFile() (*os.File, *bufio.Scanner) {
	f, err := os.OpenFile("kv.txt", os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	return f, scannerFile
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
			fmt.Errorf("Could not Decrypt: %v", err)
		}

		key, value, _ := strings.Cut(string(lines), ":")

		if key == input {
			fmt.Printf("%v\n", value)
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
