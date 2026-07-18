package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/parse/format"
	"github.com/tucats/ego/internal/language/parse/resolve"
)

// FmtAction is the command handler for "ego format source". It parses Ego source
// into an abstract syntax tree and re-emits it in canonical form (the same
// pipeline a future editor integration would use). With no file arguments it
// reads source from standard input and writes the result to standard output.
//
// Options:
//
//	--write     (-w)   rewrite each named file in place instead of printing
//	--ast       (-a)   print the parsed syntax tree instead of formatted source
//	--analysis         run internal/language/parse/resolve's symbol/scope
//	                   analysis and print any diagnostics.
//	--tabs      (-t)   number of spaces per indentation level (default 4)
//	--fragment         parse as a bare statement sequence (REPL/@test style)
//	--program          parse as a complete program (package/func at top level)
//
// When neither --fragment nor --program is given, the input is parsed as a
// program and, if that fails, retried as a statement fragment.
//
// --write has no effect when combined with --ast or --analysis (both are
// reports, not rewrites) and is ignored for standard input.
//
// Invoked by:
//
//	Verb: ego format source [file...]
func FmtAction(c *cli.Context) error {
	writeInPlace := c.Boolean("write")
	dumpAST := c.Boolean("ast")
	analysis := c.Boolean("analysis")
	fragment := c.Boolean("fragment")
	program := c.Boolean("program")

	// Indentation width in spaces; default to the formatter's default.
	opts := format.Options{}
	if tabs, found := c.Integer("tabs"); found {
		opts.IndentWidth = tabs
	}

	fileNames := c.FindGlobal().Parameters

	// With no file arguments, format standard input to standard output.
	// (--write has no meaning for stdin and is ignored.)
	if len(fileNames) == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return errors.New(err)
		}

		if analysis {
			out, err := analyzeSource(string(src), fragment, program, "-")
			if err != nil {
				return err
			}

			if len(out) > 0 {
				out = strings.TrimSuffix(out, "\n")
				if err := c.Output(out); err != nil {
					return err
				}

				return errors.ErrTerminatedWithErrors
			}
		}

		out, err := renderSource(string(src), fragment, program, dumpAST, opts)
		if err != nil {
			return err
		}

		return c.Output(out)
	}

	// Otherwise process each named file.
	for _, name := range fileNames {
		src, err := os.ReadFile(name)
		if err != nil {
			return errors.New(err)
		}

		if analysis {
			out, err := analyzeSource(string(src), fragment, program, name)
			if err != nil {
				return errors.New(err).Context(name)
			}

			if len(out) > 0 {
				out = strings.TrimSuffix(out, "\n")
				if err := c.Output(out); err != nil {
					return err
				}

				return errors.ErrTerminatedWithErrors
			}
		}

		out, err := renderSource(string(src), fragment, program, dumpAST, opts)
		if err != nil {
			return errors.New(err).Context(name)
		}

		if writeInPlace && !dumpAST {
			if err := os.WriteFile(name, []byte(out), 0644); err != nil {
				return errors.New(err).Context(name)
			}

			continue
		}

		if err := c.Output(out); err != nil {
			return err
		}
	}

	return nil
}

// analyzeSource parses source according to the mode flags and runs
// internal/language/parse/resolve's symbol/scope analysis over the result,
// returning a report of any diagnostics found, one per line, in the
// conventional linter format "label:line:col: [kind] message" -- label is
// the source file name, or "-" for standard input. label is not a
// guaranteed-accurate position for diagnostics anchored inside a directive
// argument (e.g. inside "@assert ..."): see the resolve package doc's
// "Known limitations" for why those report a synthetic position instead of
// their true location in source. When no diagnostics are found, a single
// confirmation line is returned instead of empty output.
func analyzeSource(source string, fragment, program bool, label string) (string, error) {
	file, err := parseSource(source, fragment, program)
	if err != nil {
		return "", err
	}

	_, diags := resolve.Resolve(file)
	if len(diags) == 0 {
		return "", nil
	}

	var b strings.Builder

	for _, d := range diags {
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", label, d.Pos.Line, d.Pos.Column, d.Message)
	}

	return b.String(), nil
}

// renderSource parses source according to the mode flags and returns either its
// canonical formatting (using opts) or, when dumpAST is true, a textual dump of
// the AST.
func renderSource(source string, fragment, program, dumpAST bool, opts format.Options) (string, error) {
	file, err := parseSource(source, fragment, program)
	if err != nil {
		return "", err
	}

	if dumpAST {
		return ast.Dump(file), nil
	}

	return format.FileWithOptions(file, opts)
}

// parseSource selects the parse entry point from the mode flags. With neither
// flag set it tries program mode first and falls back to fragment mode.
func parseSource(source string, fragment, program bool) (*ast.File, error) {
	switch {
	case fragment:
		return parse.ParseStatements(source)
	case program:
		return parse.ParseProgram(source)
	default:
		file, err := parse.ParseProgram(source)
		if err != nil {
			return parse.ParseStatements(source)
		}

		return file, nil
	}
}
