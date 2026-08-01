package debugger

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// showCommand dispatches the "show <subcommand>" family of debugger commands.
// Each sub-command displays a different view of the running program's state.
//
// Supported sub-commands:
//
//	show breaks / show breakpoints — list all active breakpoints
//	show symbols                   — dump the current symbol table
//	show line                      — reprint the current source line
//	show frames / show calls [n]   — display the call-frame stack (top n frames)
//	show scope                     — walk the symbol-table scope chain
//	show source [start[:end]]      — list source lines with indentation
//
// s is the symbol table at the current execution point.
// tokens is the full command tokenizer, positioned before "show".
// line is the line number the context is stopped at.
// c is the bytecode context (needed for frame and tokenizer access).
// sessionContext routes all output to the correct destination.
func showCommand(s *symbols.SymbolTable, tokens *tokenizer.Tokenizer, line int, c *bytecode.Context, sessionContext *session) error {
	var (
		err error
		t   = tokens.Peek(2) // the word immediately after "show"
		tx  = c.GetTokenizer()
	)

	switch t.Spelling() {
	case "package":
		// Perform the work of the "@package" command in the debugger.
		tokens.Advance(2)

		names := make([]string, 0, 1)

		for {
			if tokens.EndOfStatement() {
				break
			}

			t := tokens.Next()
			if t.IsIdentifier() {
				names = append(names, t.Spelling())
			} else {
				return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(1))
			}

			if tokens.IsNext(tokenizer.CommaToken) {
				continue
			}

			break
		}

		name := strings.Join(names, ",")

		showPackage(c, sessionContext, name)

	case "breaks", "breakpoints":
		// List every active breakpoint.
		showBreaks(sessionContext)

	case "symbols":
		// Dump the symbol table at the current scope.
		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		// symbols.SymbolTable.Format(true) produces a multi-line string with
		// one "name = value" entry per line, prefixed by type information when
		// the boolean argument is true.
		sessionContext.println(s.Format(true))

	case "line":
		// Reprint the source line the debugger is currently stopped at.
		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		text := tx.GetLine(line)
		sessionContext.printf("%s:\n\t%5d, %s\n", stepTo, line, text)

	case "frames", "calls":
		// Display the call-frame stack.  An optional numeric argument limits
		// the output to the top N frames; the special word "all" removes the
		// limit.  bytecode.ShowAllCallFrames is the sentinel for "unlimited".
		depth := bytecode.ShowAllCallFrames

		tx := tokens.Peek(3)
		if tx.IsNot(tokenizer.EndOfTokens) {
			if tokens.Peek(4).IsNot(tokenizer.EndOfTokens) {
				return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(4))
			}

			if tx.Spelling() != "all" {
				var e2 error

				depth, e2 = egostrings.Atoi(tx.Spelling())
				if e2 != nil {
					err = errors.ErrInvalidInteger
				}
			}
		}

		if err == nil {
			// FormatFrames returns a ready-to-print multi-line string.
			sessionContext.printf("%s", c.FormatFrames(depth))
		}

	case "scope":
		// Walk the symbol-table parent chain and print one summary line per
		// scope level — the scope name and how many symbols it contains.
		symbolTable := s
		depth := 0

		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		sessionContext.say("msg.debug.scope")

		for symbolTable != nil {
			idx := "local"

			if depth > 0 {
				// Use a right-aligned numeric depth label for non-local scopes.
				idx = fmt.Sprintf("%5d", depth)
			}

			sessionContext.printf("\t%s:  %s, %d %s\n",
				idx,
				symbolTable.Name,
				symbolTable.Size(),
				i18n.L("symbols"),
			)

			depth++
			symbolTable = symbolTable.Parent()
		}

	case "source":
		// DEBUGGER-SHOW-1: showSource now returns an error so that invalid
		// range arguments (e.g. "show source abc") are reported to the user
		// rather than silently ignored.
		err = showSource(tx, tokens, sessionContext)

	default:
		err = errors.ErrInvalidDebugCommand.Context("show " + t.Spelling())
	}

	return err
}

// Implementation of the "show package" debugger command. This calls the bytecode
// DumpPackage opcode to print the package information to the sessionContext
// output stream.  The name argument is a comma-separated list of package names
// to show; if name is empty, all packages are shown.
func showPackage(c *bytecode.Context, sessionContext *session, name string) error {
	var err error

	// We have to set up a call to the DumpPackage bytecode handler using the
	// current context and symbol table, and using the name as the operand
	// to the DumpPackage opcode.  The DumpPackage opcode will then print the
	// package information to the sessionContext output stream.

	if name == "" {
		err = bytecode.DumpPackagesByteCode(c, nil)
	} else {
		names := strings.Split(name, ",")
		for idx, n := range names {
			names[idx] = strings.TrimSpace(n)
		}

		sort.Strings(names)

		err = bytecode.DumpPackagesByteCode(c, names)
	}

	if err != nil {
		return err
	}

	text := c.GetOutput()
	text = strings.TrimSuffix(text, "\n") // remove trailing newline

	sessionContext.println(text)

	return nil
}
