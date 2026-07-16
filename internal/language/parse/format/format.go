// Package format renders an Ego AST (github.com/tucats/ego/internal/language/parse/ast)
// back into canonical Ego source text. It follows the same formatting rules as
// Go's gofmt where they apply to Ego:
//
//   - Space indentation, DefaultIndentWidth (4) spaces per block-nesting level
//     by default; the width is configurable via Options.IndentWidth.
//   - An opening "{" stays on the line of the construct it belongs to.
//   - One statement per line.
//   - A single space around binary operators and after commas; no space just
//     inside "(", "[", or "{".
//   - A blank line between top-level declarations of a full program.
//
// The typical entry point is Source, which parses source text and reprints it
// canonically. Node and File print an already-built AST. Each has a
// ...WithOptions counterpart that takes formatting Options.
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

// DefaultIndentWidth is the number of spaces per indentation level used when
// Options.IndentWidth is not set (zero or negative).
const DefaultIndentWidth = 4

// Options controls formatting behavior.
type Options struct {
	// IndentWidth is the number of spaces emitted per block-nesting level. A
	// value of zero or less selects DefaultIndentWidth.
	IndentWidth int
}

// indentUnit returns the string emitted for one indentation level.
func (o Options) indentUnit() string {
	width := o.IndentWidth
	if width <= 0 {
		width = DefaultIndentWidth
	}

	return strings.Repeat(" ", width)
}

// Source parses Ego source text and returns its canonical formatting with
// default options. When bare is true the input is treated as a statement
// fragment (parse.ParseStatements); otherwise it is treated as a complete
// program (parse.ParseProgram).
func Source(source string, bare bool) (string, error) {
	return SourceWithOptions(source, bare, Options{})
}

// SourceWithOptions is Source with explicit formatting options.
func SourceWithOptions(source string, bare bool, opts Options) (string, error) {
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

	return FileWithOptions(file, opts)
}

// File renders a parsed file to canonical source text with default options.
func File(file *ast.File) (string, error) {
	return FileWithOptions(file, Options{})
}

// FileWithOptions is File with explicit formatting options.
func FileWithOptions(file *ast.File, opts Options) (string, error) {
	p := newPrinter(opts)

	p.printFile(file)

	if p.err != nil {
		return "", p.err
	}

	return p.buf.String(), nil
}

// Node renders a single AST node to source text (without a trailing newline)
// using default options. It is useful for formatting an isolated expression or
// statement.
func Node(node ast.Node) (string, error) {
	return NodeWithOptions(node, Options{})
}

// NodeWithOptions is Node with explicit formatting options.
func NodeWithOptions(node ast.Node, opts Options) (string, error) {
	p := newPrinter(opts)

	p.printStmt(node)

	if p.err != nil {
		return "", p.err
	}

	return p.buf.String(), nil
}

// printer accumulates formatted output. indent is the current block-nesting
// depth; indentUnit is the string emitted per level; err records the first
// failure (an unsupported node), after which all further writes are suppressed.
type printer struct {
	buf        strings.Builder
	indent     int
	indentUnit string
	err        error
}

// newPrinter creates a printer configured from opts.
func newPrinter(opts Options) *printer {
	return &printer{indentUnit: opts.indentUnit()}
}

// write appends raw text to the output (unless an error has been recorded).
func (p *printer) write(s string) {
	if p.err == nil {
		p.buf.WriteString(s)
	}
}

// indentation returns the whitespace for the current nesting depth.
func (p *printer) indentation() string {
	return strings.Repeat(p.indentUnit, p.indent)
}

// newline writes a line break followed by the current indentation.
func (p *printer) newline() {
	p.write("\n")
	p.write(p.indentation())
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

// printFile renders the top-level declarations/statements of a file. Following
// gofmt, a blank line separates top-level declarations (and functions) from
// their neighbors, but consecutive plain statements — which Ego, unlike Go,
// permits at the top level — are printed one per line with no blank line
// between them.
func (p *printer) printFile(file *ast.File) {
	if file == nil {
		return
	}

	for i, decl := range file.Decls {
		if i > 0 {
			p.newline()

			if isDeclaration(file.Decls[i-1]) || isDeclaration(decl) {
				p.write("\n")
				p.write(p.indentation())
			}
		}

		p.printStmt(decl)
	}

	if len(file.Decls) > 0 {
		p.write("\n")
	}
}

// isDeclaration reports whether a top-level node is a declaration or function
// definition, which gofmt sets off from its neighbors with a blank line.
func isDeclaration(node ast.Node) bool {
	switch node.Kind() {
	case ast.KindPackageDecl, ast.KindImportDecl, ast.KindConstDecl,
		ast.KindTypeDecl, ast.KindVarDecl, ast.KindFuncDecl:
		return true
	default:
		return false
	}
}
