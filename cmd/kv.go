/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"barbtils/internal/encryption"
	l "barbtils/internal/logger"
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var kvFilePath string

// kvCmd represents the kv command
var kvCmd = &cobra.Command{
	Use:                   "kv",
	Short:                 "Create and consume Key-Value Pairs with AES-GCM Encryption and Decryption",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var addCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"a"},
	Short: `Create KV pair by inputting your Key.
	      Its value can be entered by Stdin after the command is invoked`,
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		kvFilePath = viper.GetString("kv.filePath")
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 || len(args) == 0 {
			l.Fatal("Key must be a single string")
		}
		if len(args) == 1 {
			userInput := strings.TrimSpace(args[0])
			if userInput == "" {
				l.Fatal("Key must not be an empty string")
			} else {
				pairToAppend(userInput)
			}
		}
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"del", "d"},
	Short:   `Delete KV pair by inputting your Key as an argument`,
	// DisableFlagParsing:    true,
	// DisableFlagsInUseLine: true,
	Args: cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		kvFilePath = viper.GetString("kv.filePath")
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 || len(args) == 0 {
			l.Fatal("Key must be a single string")
		}
		if len(args) == 1 {
			userInput := strings.TrimSpace(args[0])
			if userInput == "" {
				l.Fatal("Key must not be an empty string")
			} else {
				err := deleteExistingKey(userInput)
				if err != nil {
					err.Error()
				}
			}
		}
	},
}

var getCmd = &cobra.Command{
	Use:                   "get",
	Short:                 "Get KV pair by inputting your Key as an argument",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.RangeArgs(0, 1),
	PreRun: func(cmd *cobra.Command, args []string) {
		kvFilePath = viper.GetString("kv.filePath")
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			l.Fatal("Key must be a single string")
			// errors.New("Key must be a single string")
		}
		if len(args) == 0 {
			readFullKvFile()
		}
		if len(args) == 1 {
			readKvFile(args[0])
		}
	},
}

func init() {
	RootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(addCmd)
	kvCmd.AddCommand(getCmd)
	kvCmd.AddCommand(deleteCmd)

	// kvCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
	// 	_ = command.Flags().MarkHidden("config")     // Replace with your flag's name
	// 	_ = command.Flags().MarkHidden("debug")      // Replace with your flag's name
	// 	_ = command.Flags().MarkHidden("get_db_uri") // Replace with your flag's name
	// 	_ = command.Flags().MarkHidden("json")       // Replace with your flag's name
	// 	_ = command.Flags().MarkHidden("version")    // Replace with your flag's name
	// 	command.Parent().HelpFunc()(command, strings)
	// })
}

func initKvFile() (*os.File, *bufio.Scanner) {
	f, err := os.OpenFile(kvFilePath, os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	return f, scannerFile
}

func readFullKvFile() {
	f, err := os.OpenFile(kvFilePath, os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	for scannerFile.Scan() {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(scannerFile.Text()))
		if err != nil {
			l.Fatalf("corrupt line: %w", err)
		}
		lines, err := encryption.Decrypt(raw)
		if err != nil {
			fmt.Errorf("Could not Decrypt: %v", err)
		}

		key, _, _ := strings.Cut(string(lines), ":")

		fmt.Printf("key: %v\n", key)
	}

	// 5. Check for errors that occurred during scanning
	if err := scannerFile.Err(); err != nil {
		log.Fatalf("error during scanning: %s", err)
	}
}

func readKvFile(input string) {
	input = strings.TrimSpace(input)

	f, err := os.OpenFile(kvFilePath, os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	for scannerFile.Scan() {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(scannerFile.Text()))
		if err != nil {
			l.Fatalf("corrupt line: %w", err)
		}

		lines, err := encryption.Decrypt(raw)
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
	err := scanForExistingKey(keyInput)
	if err != nil {
		l.Fatal(err)
	}

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

	f, err := os.OpenFile(kvFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	scannerFile := bufio.NewWriter(f)

	byte, err := encryption.Encrypt([]byte(kvPair))
	if err != nil {
		fmt.Errorf("Encryption err: %v\n", err)
	}

	_, err = scannerFile.WriteString(base64.StdEncoding.EncodeToString(byte) + "\n")
	if err != nil {
		fmt.Errorf("err: %v\n", err)
	}

	err = scannerFile.Flush()
	if err != nil {
		fmt.Errorf("Error flushing buffer:", err)
		return
	}
}

func scanForExistingKey(input string) error {
	f, err := os.OpenFile(kvFilePath, os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
		return err
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	for scannerFile.Scan() {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(scannerFile.Text()))
		if err != nil {
			return fmt.Errorf("corrupt line: %w", err)
		}

		lines, err := encryption.Decrypt(raw)
		if err != nil {
			return fmt.Errorf("Could not Decrypt: %v", err)
		}

		key, _, found := strings.Cut(string(lines), ":")

		if found && key == input {
			return fmt.Errorf("Key [%s] already exists, please use a new key", key)
		}
	}

	if err := scannerFile.Err(); err != nil {
		log.Fatalf("error during scanning: %s", err)
		return err
	}

	return nil
}

func deleteExistingKey(input string) error {
	f, err := os.OpenFile(kvFilePath, os.O_RDONLY, 0644)
	if err != nil {
		l.Fatal(err)
		return err
	}
	defer f.Close()

	scannerFile := bufio.NewScanner(f)

	var newLines []string
	for scannerFile.Scan() {
		rawLine, err := base64.StdEncoding.DecodeString(strings.TrimSpace(scannerFile.Text()))

		if err != nil {
			return fmt.Errorf("corrupt line: %w", err)
		}

		lines, err := encryption.Decrypt(rawLine)
		if err != nil {
			return fmt.Errorf("Could not Decrypt: %v", err)
		}

		key, _, found := strings.Cut(string(lines), ":")

		if found && key != input {
			newLines = append(newLines, string(rawLine))
		}
	}

	if err := scannerFile.Err(); err != nil {
		log.Fatalf("error during scanning: %s", err)
		return err
	}

	// Overwrite the file with the filtered lines
	outFile, err := os.Create(kvFilePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return err
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	for _, line := range newLines {
		_, err := writer.WriteString(base64.StdEncoding.EncodeToString([]byte(line)) + "\n")
		if err != nil {
			fmt.Println("Error writing file:", err)
			return err
		}
	}
	writer.Flush()

	return nil
}
