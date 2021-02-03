package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// ReaderInstance is the readline Instance used for console input
var consoleReader *readline.Instance
var consoleLock sync.Mutex

// ReadConsoleText reads a line of text from the user's console.
func ReadConsoleText(prompt string) string {
	useReadLine := persistence.GetBool(defs.UseReadline)

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
	if consoleReader == nil {
		historyFile := persistence.Get("ego.console.history")
		if historyFile == "" {
			historyFile = filepath.Join(os.TempDir(), "ego-commands.txt")
		}

		consoleReader, _ = readline.NewEx(&readline.Config{
			Prompt:            prompt,
			HistoryFile:       historyFile,
			HistorySearchFold: true,
			HistoryLimit:      100,
		})
	}
	consoleLock.Unlock()

	if len(prompt) > 1 && prompt[:1] == "~" {
		b, _ := consoleReader.ReadPassword(prompt[1:])

		return string(b)
	}
	// Set the prompt string and do the read. We ignore errors.
	consoleReader.SetPrompt(prompt)

	result, _ := consoleReader.Readline()

	return result + "\n"
}
