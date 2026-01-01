package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var hashCmd = &cobra.Command{
	Use:   "hash-password",
	Short: "Generate bcrypt hash for a password",
	Long: `Generate a bcrypt hash for use with AUTH_PASSWORD_HASH configuration.

Example usage:
  ./whatsapp hash-password                              # Interactive prompt
  ./whatsapp hash-password --password="mysecretpassword"  # Non-interactive

The generated hash can be used in your .env file:
  AUTH_PASSWORD_HASH=$2a$10$...`,
	Run: hashPassword,
}

var passwordFlag string

func init() {
	rootCmd.AddCommand(hashCmd)
	hashCmd.Flags().StringVar(&passwordFlag, "password", "", "Password to hash (if not provided, will prompt securely)")
}

func hashPassword(cmd *cobra.Command, args []string) {
	var password string

	if passwordFlag != "" {
		password = passwordFlag
	} else {
		// Prompt for password securely
		fmt.Print("Enter password: ")

		// Try to read password without echo
		if term.IsTerminal(int(syscall.Stdin)) {
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println() // Add newline after password input
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			password = string(bytePassword)
		} else {
			// Fallback for non-terminal input (e.g., piped input)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			password = strings.TrimSpace(input)
		}
	}

	if password == "" {
		fmt.Fprintf(os.Stderr, "Error: Password cannot be empty\n")
		os.Exit(1)
	}

	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Password hash generated successfully!")
	fmt.Println("\nAdd this to your .env file:")
	fmt.Printf("AUTH_PASSWORD_HASH=%s\n", string(hash))
	fmt.Println("\nOr use the --auth-password-hash flag:")
	fmt.Printf("./whatsapp rest --auth-password-hash=\"%s\"\n", string(hash))
}
