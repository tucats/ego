// Package format renders an Ego AST (github.com/tucats/ego/internal/language/parse/ast)
// back into canonical Ego source text. It follows the same formatting rules as
// Go's gofmt where they apply to Ego:
//
//   - Tab indentation, one tab per block-nesting level.
//   - An opening "{" stays on the line of the construct it belongs to.
//   - One statement per line.
//   - A single space around binary operators and after commas; no space just
//     inside "(", "[", or "{".
//   - A blank line between top-level declarations of a full program.
//
// The typical entry point is Source, which parses source text and reprints it
// canonically. Node and File print an already-built AST.
//
// TRIVIA: Because the tokenizer discards comments (see ast/node.go), the
// formatter cannot yet reproduce them; comments are lost on a format round-trip.
// Restoring them is the follow-up tracked by the "TRIVIA:" breadcrumbs across
// the parse packages. A second known approximation is operator spacing: gofmt
// varies spacing by precedence ("a*b + c*d"); this printer uses a single space
// around every binary operator. Both are documented refinements, not
// correctness bugs.
package format

import (
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse"
	"github.com/tucats/ego/internal/language/parse/ast"
)

// Source parses Ego source text and returns its canonical formatting. When bare
// is true the input is treated as a statement fragment (parse.ParseStatements);
// otherwise it is treated as a complete program (parse.ParseProgram).
func Source(source string, bare bool) (string, error) {
	var (
		file *ast.File
		err  error
	)

	if bare {
		file, err = parse.ParseStatements(source)
	} else {
		file, err = parse.ParseProgram(source)
	}

	if err != nil {
		return "", err
	}

	return File(file)
}

// File renders a parsed file to canonical source text.
func File(file *ast.File) (string, error) {
	p := &printer{}

	p.printFile(file)

	if p.err != nil {
		return "", p.err
	}

	return p.buf.String(), nil
}

// Node renders a single AST node to source text (without a trailing newline).
// It is useful for formatting an isolated expression or statement.
func Node(node ast.Node) (string, error) {
	p := &printer{}

	p.printStmt(node)

	if p.err != nil {
		return "", p.err
	}

	return p.buf.String(), nil
}

// printer accumulates formatted output. indent is the current block-nesting
// depth; err records the first failure (an unsupported node), after which all
// further writes are suppressed.
type printer struct {
	buf    strings.Builder
	indent int
	err    error
}

// write appends raw text to the output (unless an error has been recorded).
func (p *printer) write(s string) {
	if p.err == nil {
		p.buf.WriteString(s)
	}
}

// newline writes a line break followed by the current indentation.
func (p *printer) newline() {
	p.write("\n")
	p.write(strings.Repeat("\t", p.indent))
}

// fail records the first formatting error for an unsupported node.
func (p *printer) fail(node ast.Node) {
	if p.err == nil {
		p.err = errors.New(errors.ErrInvalidType).Context("format: " + kindName(node))
	}
}

// kindName returns a printable kind label for an error, tolerating a nil node.
func kindName(node ast.Node) string {
	if node == nil {
		return "<nil>"
	}

	return node.Kind().String()
}

// printFile renders the top-level declarations/statements of a file. For a full
// program, consecutive top-level declarations are separated by a blank line, as
// gofmt does; for a bare fragment the statements are printed one per line with
// no extra blank lines, matching the body of a function.
func (p *printer) printFile(file *ast.File) {
	if file == nil {
		return
	}

	for i, decl := range file.Decls {
		if i > 0 {
			p.newline()

			if !file.Bare {
				// A blank line between top-level declarations of a program.
				p.write("\n")
				p.write(strings.Repeat("\t", p.indent))
			}
		}

		p.printStmt(decl)
	}

	if len(file.Decls) > 0 {
		p.write("\n")
	}
}
