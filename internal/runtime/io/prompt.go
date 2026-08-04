package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// This package is itself called "io", so Go's own io and errors packages
	// are given distinct names here to avoid the collision.
	goErrors "errors"
	goIO "io"

	"github.com/chzyer/readline"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// maxHistorySize is the maximum number of lines to retain in
// the persistent history file of command line input.
const maxHistorySize = 100

// ReaderInstance is the readline Instance used for console input.
var consoleReader *readline.Instance

// This mutex serializes access to the console reader since it is not
// inherently thread-safe.
var consoleLock sync.Mutex

// passwordPromptPrefix is the string prefix you can put in the prompt
// string for a call to the Ego prompt() function to cause it to suppress
// keyboard echo for the input. The text after this prefix, if any, is used
// as the prompt text.
const passwordPromptPrefix = "password~"

// prompt implements the io.prompt() function, which uses the console
// reader. This cannot reside in the runtime/io package, because it depends on
// the console reader function.
func prompt(symbols *symbols.SymbolTable, args data.List) (any, error) {
	var (
		text   string
		prompt string
	)

	if args.Len() > 0 {
		prompt = data.String(args.Get(0))
	}

	if strings.HasPrefix(prompt, passwordPromptPrefix) {
		text = ui.PromptPassword(prompt[len(passwordPromptPrefix):])
	} else {
		// The reason the read stopped is deliberately discarded here. This is
		// the Ego language's own prompt() function, and an Ego program that
		// calls it today gets an empty string back when the input ends. Making
		// it return an error instead would change the meaning of existing Ego
		// scripts, which is a language decision rather than a bug fix, so the
		// behavior is left exactly as it was.
		text, _ = ReadConsoleText(prompt)
	}

	text = strings.TrimSuffix(text, "\n")

	return text, nil
}

// Errors reported by ReadConsoleText to say why a read produced no more input.
//
// These are declared here, rather than callers testing for the readline
// library's own error values, so that nothing outside this file needs to know
// which library does the reading.
var (
	// ErrEndOfInput means there is no more input and there never will be: the
	// user pressed Ctrl-D, or input was redirected from something that ran
	// out. A loop that keeps prompting must stop when it sees this, or it
	// will ask forever and never get an answer.
	ErrEndOfInput = goErrors.New("end of console input")

	// ErrInterrupted means the user pressed Ctrl-C at the prompt, abandoning
	// the line they were typing.
	//
	// Note that this only happens while the console is waiting for input.
	// While an Ego program is actually running, the readline library is not
	// in control of the terminal, so Ctrl-C arrives as an ordinary interrupt
	// signal and is handled by the bytecode interpreter's signal watcher
	// instead, which stops the running program (see bytecode/run.go).
	ErrInterrupted = goErrors.New("console input interrupted")
)

// ReadConsoleText reads a line of text from the user's console.
//
// The second return value says why the read stopped, and is nil in the
// ordinary case of the user typing a line and pressing Enter. It is
// ErrEndOfInput or ErrInterrupted when the user is trying to end the session
// rather than supply a line. Anything that reads in a loop must check it:
// previously this function reported both of those as an empty line, so
// pressing Ctrl-D at an Ego prompt did nothing at all and the only way out of
// the interactive console was to type "exit".
func ReadConsoleText(prompt string) (string, error) {
	var (
		b           strings.Builder
		useReadLine = settings.GetBool(defs.UseReadlineSetting)
		reading     = true
		line        = 1
	)

	// If readline has been explicitly disabled for some reason, do a more primitive input operation.
	if !useReadLine {
		for reading {
			text, err := ui.PromptLine(prompt)
			if err != nil {
				// Any text read before the input ended is still worth
				// returning; the end will be reported on the next call.
				if b.Len() > 0 {
					return b.String(), nil
				}

				return "", ErrEndOfInput
			}

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

		return b.String(), nil
	}

	// Nope, let's use readline. If we have never initialized
	// the reader, let's do so now (in a thread-safe fashion)
	consoleLock.Lock()
	defer consoleLock.Unlock()

	if consoleReader == nil {
		historyFile := settings.Get(defs.ConsoleHistorySetting)
		if historyFile == "" {
			homeDir, _ := os.UserHomeDir()
			historyFile = filepath.Join(homeDir, settings.ProfileDirectory, "ego-commands.txt")
		}

		// If the history file does not yet exist, create it now with owner-only
		// read/write permissions (0600) so it is secured from the start rather
		// than inheriting whatever umask readline would use.
		if _, err := os.Stat(historyFile); os.IsNotExist(err) {
			if f, err := os.OpenFile(historyFile, os.O_CREATE|os.O_WRONLY, 0600); err == nil {
				f.Close()
			}
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

		return string(b), nil
	}

	// Set the prompt string and do the read.
	consoleReader.SetPrompt(prompt)

	result, err := consoleReader.Readline()

	// Translate the readline library's two "the user is not giving us a line"
	// outcomes into this package's own errors.
	//
	// io.EOF is Ctrl-D on an empty line, or redirected input running out.
	// readline.ErrInterrupt is Ctrl-C typed at the prompt. Because readline
	// puts the terminal into raw mode while it is reading, Ctrl-C arrives as
	// an ordinary character for readline to interpret rather than as a signal,
	// which is why it shows up here as a return value instead of waking the
	// interpreter's signal watcher.
	if err != nil {
		switch {
		case goErrors.Is(err, goIO.EOF):
			return "", ErrEndOfInput

		case goErrors.Is(err, readline.ErrInterrupt):
			return "", ErrInterrupted

		default:
			// Any other failure is also a failure to produce a line, and a
			// caller that keeps asking would spin, so it is reported as the
			// end of the input as well.
			ui.Log(ui.AppLogger, "app.console.error", ui.A{
				"error": err})

			return "", ErrEndOfInput
		}
	}

	return result + "\n", nil
}
