package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// passwordPromptPrefix is the string prefix you can put in the prompt
// string for a call to the Ego prompt() function to cause it to suppress
// keyboard echo for the input. The text after this prefix, if any, is used
// as the prompt text.
const passwordPromptPrefix = "password~"

// Prompt implements the io.Prompt() function, which uses the console
// reader. This cannot reside in the runtime/io package, because it depends on
// the console reader function.
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 1 {
		return nil, errors.ErrArgumentCount
	}

	prompt := ""
	if len(args) > 0 {
		prompt = data.String(args[0])
	}

	var text string
	if strings.HasPrefix(prompt, passwordPromptPrefix) {
		text = ui.PromptPassword(prompt[len(passwordPromptPrefix):])
	} else {
		text = ReadConsoleText(prompt)
	}

	text = strings.TrimSuffix(text, "\n")

	return text, nil
}

// maxHistorySize is the maximum number of lines to retain in
// the persistent history file of command line input.
const maxHistorySize = 100

// ReaderInstance is the readline Instance used for console input.
var consoleReader *readline.Instance
var consoleLock sync.Mutex

// ReadConsoleText reads a line of text from the user's console.
func ReadConsoleText(prompt string) string {
	useReadLine := settings.GetBool(defs.UseReadline)

	// If readline has been explicitly disabled for some reason,
	// do a more primitive input operation.
	// TODO this entire functionality could probably be moved
	// into ui.Prompt() at some point.
	if !useReadLine {
		var b strings.Builder

		reading := true
		line := 1

		for reading {
			text := ui.Prompt(prompt)
			if len(text) == 0 {
				break
			}

			line = line + 1

			if text[len(text)-1:] == "\\" {
				text = text[:len(text)-1]
				prompt = fmt.Sprintf("ego[%d]> ", line)
			} else {
				reading = false
			}

			b.WriteString(text)
			b.WriteString("\n")
		}

		return b.String()
	}

	// Nope, let's use readline. IF we have never initialized
	// the reader, let's do so now (in a threadsafe fashion)
	consoleLock.Lock()
	defer consoleLock.Unlock()

	if consoleReader == nil {
		historyFile := settings.Get(defs.ConsoleHistorySetting)
		if historyFile == "" {
			homeDir, _ := os.UserHomeDir()
			historyFile = filepath.Join(homeDir, settings.ProfileDirectory, "ego-commands.txt")
		}

		consoleReader, _ = readline.NewEx(&readline.Config{
			Prompt:            prompt,
			HistoryFile:       historyFile,
			HistorySearchFold: true,
			HistoryLimit:      maxHistorySize,
		})
	}

	if len(prompt) > 1 && prompt[:1] == "~" {
		b, _ := consoleReader.ReadPassword(prompt[1:])

		return string(b)
	}
	// Set the prompt string and do the read. We ignore errors.
	consoleReader.SetPrompt(prompt)

	result, _ := consoleReader.Readline()

	return result + "\n"
}
