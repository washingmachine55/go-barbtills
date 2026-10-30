package encryption

import (
	"fmt"
	"os"
	"syscall"

	"github.com/zalando/go-keyring"

	"golang.org/x/term"
)

const serviceName = "Login"

var accountName = os.Getenv("USER")

func GetUserOSKeyringToken() (string, error) {
	// 1. Try to get password from keyring
	secret, err := keyring.Get(serviceName, accountName)
	if err == nil {
		return secret, nil
	}

	// 2. Prompt user securely if not found
	fmt.Print("Enter your Password to store in Keyring: ")
	byteToken, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println() // Print newline after hidden input

	token := string(byteToken)

	// 3. Save to keyring
	err = keyring.Set(serviceName, accountName, token)
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not save to keyring: %v\n", err)
	} else {
		fmt.Println("🚀 Token securely saved!")
	}

	return token, nil
}
