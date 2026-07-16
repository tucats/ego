package commands

import (
	"io"
	"os"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/parse/format"
)

// FmtAction is the command handler for "ego fmt". It parses Ego source into an
// abstract syntax tree and re-emits it in canonical form (the same pipeline a
// future editor integration would use). With no file arguments it reads source
// from standard input and writes the result to standard output.
//
// Options:
//
//	--write  (-w)   rewrite each named file in place instead of printing
//	--ast    (-a)   print the parsed syntax tree instead of formatted source
//	--fragment      parse as a bare statement sequence (REPL/@test style)
//	--program       parse as a complete program (package/func at top level)
//
// When neither --fragment nor --program is given, the input is parsed as a
// program and, if that fails, retried as a statement fragment.
//
// Invoked by:
//
//	Verb: ego fmt [file...]
func FmtAction(c *cli.Context) error {
	writeInPlace := c.Boolean("write")
	dumpAST := c.Boolean("ast")
	fragment := c.Boolean("fragment")
	program := c.Boolean("program")

	fileNames := c.FindGlobal().Parameters

	// With no file arguments, format standard input to standard output.
	// (--write has no meaning for stdin and is ignored.)
	if len(fileNames) == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return errors.New(err)
		}

		out, err := renderSource(string(src), fragment, program, dumpAST)
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

		out, err := renderSource(string(src), fragment, program, dumpAST)
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

// renderSource parses source according to the mode flags and returns either its
// canonical formatting or, when dumpAST is true, a textual dump of the AST.
func renderSource(source string, fragment, program, dumpAST bool) (string, error) {
	file, err := parseSource(source, fragment, program)
	if err != nil {
		return "", err
	}

	if dumpAST {
		return ast.Dump(file), nil
	}

	return format.File(file)
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
