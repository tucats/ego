package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// Prompt prints a prompt string, and gets input from the console.
// The line endings are removed and the remainder of the input is
// returned as a string.
func Prompt(p string) string {
	reader := bufio.NewReader(os.Stdin)
	if !IsConsolePipe() {
		fmt.Printf("%s", p)
	}
	buffer, _ := reader.ReadString('\n')

	//Remove any extra line endings (CRLF or LF)
	buffer = strings.Replace(buffer, "\r\n", "", -1)
	buffer = strings.Replace(buffer, "\n", "", -1)

	return buffer
}

// PromptPassword prompts the user with a string prompt, and then
// allows the user to enter confidential information such as a password
// without it being echoed on the terminal. The value entered is returned
// as a string.
func PromptPassword(p string) string {
	if !IsConsolePipe() {
		fmt.Print(p)
	}
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))

	password := string(bytePassword)
	fmt.Println() // it's necessary to add a new line after user's input

	return password
}

// IsConsolePipe detects if the console (stdin) is a pipe versus a real device. This
// is used to manage prompts, etc.
func IsConsolePipe() bool {
	fi, _ := os.Stdin.Stat() // get the FileInfo struct describing the standard input.
	return (fi.Mode() & os.ModeCharDevice) == 0
}
