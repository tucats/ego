package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompt prints a prompt string, and gets input from the console.
// The line endings are removed and the remainder of the input is
// returned as a string.
func Prompt(p string) string {
	reader := bufio.NewReader(os.Stdin)

	if !IsConsolePipe() {
		fmt.Printf("%s", p)
	}

	//Remove any extra line endings (CRLF or LF)
	buffer, _ := reader.ReadString('\n')
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

	bytePassword, _ := term.ReadPassword(int(os.Stdin.Fd()))
	password := string(bytePassword)

	fmt.Println() // it's necessary to add a new line after user's input

	return password
}

// IsConsolePipe detects if the console (stdin) is a pipe versus a real device. This
// is used to manage prompts, etc.
func IsConsolePipe() bool {
	fi, _ := os.Stdin.Stat() // get the FileInfo struct describing the standard input.

	isPipe := (fi.Mode() & os.ModeCharDevice) == 0

	Debug(AppLogger, "Console pipe: %v", isPipe)

	return isPipe
}
