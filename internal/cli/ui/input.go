package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

// Prompt prints a prompt string, and gets input from the console.
// The line endings are removed and the remainder of the input is
// returned as a string.
//
// This form cannot tell the difference between the user pressing Enter on an
// empty line and the input ending: both come back as an empty string. Callers
// that loop until the user is finished should use PromptLine instead, which
// says which of the two happened.
func Prompt(p string) string {
	text, _ := PromptLine(p)

	return text
}

// PromptLine is Prompt, except that it also reports why the read stopped.
//
// The returned error is io.EOF when there is no more input to be had -- the
// user pressed Ctrl-D, or input was redirected from a file that ran out. That
// distinction matters to anything that reads in a loop: without it, the end of
// the input looks exactly like a blank line, and the loop asks again, forever.
func PromptLine(p string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	if !IsConsolePipe() {
		fmt.Printf("%s", p)
	}

	// If not a terminal, no input is possible. Report that as end-of-input,
	// because it is: no amount of asking again will produce anything.
	if !readline.IsTerminal(0) || !readline.IsTerminal(int(os.Stdin.Fd())) {
		return "", io.EOF
	}

	buffer, err := reader.ReadString('\n')

	// Remove any extra line endings (CRLF or LF).
	buffer = strings.Replace(buffer, "\r\n", "", -1)
	buffer = strings.Replace(buffer, "\n", "", -1)

	// ReadString reports io.EOF both when the input ends with nothing after
	// the last line ending, and when it ends in the middle of an unterminated
	// final line. Only the first is really "there is nothing here"; if some
	// text did come back, hand it to the caller now and let the *next* call
	// report the end.
	if err != nil && buffer != "" {
		err = nil
	}

	return buffer, err
}

// PromptLinePassword prompts the user with a string prompt, and then
// allows the user to enter confidential information such as a password
// without it being echoed on the terminal. The value entered is returned
// as a string. It differs from PromptPassword in that it will return
// an io.EOF err if the user pressed control-D or otherwise ends the
// input.
func PromptLinePassword(p string) (string, error) {
	if !IsConsolePipe() {
		fmt.Print(p)
	}

	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	password := string(bytePassword)

	fmt.Println() // it's necessary to add a new line after user's input

	return password, err
}

// PromptPassword prompts the user with a string prompt, and then
// allows the user to enter confidential information such as a password
// without it being echoed on the terminal. The value entered is returned
// as a string.
func PromptPassword(p string) string {
	password, _ := PromptLinePassword(p)

	return password
}

// IsConsolePipe detects if the console (stdin) is a pipe versus a real device. This
// is used to manage prompts, etc.
func IsConsolePipe() bool {
	fi, _ := os.Stdin.Stat() // get the FileInfo struct describing the standard input.

	isPipe := (fi.Mode() & os.ModeCharDevice) == 0

	Log(AppLogger, "app.console.pipe", A{
		"flag": isPipe})

	return isPipe
}
