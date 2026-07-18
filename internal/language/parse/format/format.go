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
// Comments are preserved: the tokenizer captures them into a side list, the
// parser carries them on ast.File.Comments, and the printer interleaves them
// with the tree by source line (see print_comment.go). Placement is approximate
// in a few cases documented there, but no comment is dropped.
//
// Known approximation: operator spacing. gofmt varies spacing by precedence
// ("a*b + c*d"); this printer uses a single space around every binary operator.
// That is a documented refinement, not a correctness bug.
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

	// Trailing blank lines are always removed, leaving a single terminating
	// newline (or nothing, for empty output).
	out := strings.TrimRight(p.buf.String(), " \t\n")
	if out == "" {
		return "", nil
	}

	return out + "\n", nil
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

	// comments is the source's comment list (from ast.File.Comments), and ci is
	// a monotonic cursor into it. Together they let the printer interleave
	// comments with the tree as it walks nodes in source order. See
	// print_comment.go.
	comments []ast.Comment
	ci       int
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

	p.comments = file.Comments
	p.ci = 0

	// wrote tracks whether any output has been produced yet, so the first line
	// is not preceded by a separator. sep emits a line break before an item,
	// optionally as a blank line.
	wrote := false

	sep := func(blank bool) {
		if wrote {
			p.newline()

			if blank {
				p.write("\n")
				p.write(p.indentation())
			}
		}

		wrote = true
	}

	for i, decl := range file.Decls {
		// A blank line separates this declaration from the previous output; it is
		// consumed by whichever item (leading comment or the declaration itself)
		// is emitted first for this decl.
		blank := i > 0 && needFileBlank(file.Decls[i-1], decl)

		for p.ci < len(p.comments) && p.comments[p.ci].Line < decl.Pos().Line {
			sep(blank)
			blank = false

			p.writeComment(p.comments[p.ci].Text)

			p.ci++
		}

		sep(blank)
		p.printStmt(decl)
		p.emitTrailingComment(decl.Pos().Line)
	}

	// Any comments after the last declaration (e.g. a trailing file footer).
	for p.hasPendingComments() {
		sep(false)
		p.writeComment(p.comments[p.ci].Text)
		
		p.ci++
	}

	if wrote {
		p.write("\n")
	}
}

// needFileBlank decides whether a blank line separates two adjacent top-level
// items. It applies the same statement-layout rules used inside blocks, plus
// gofmt's convention of a blank line between top-level declarations — except
// that a run of "var" declarations is kept grouped (a blank follows the group,
// not each member).
func needFileBlank(prev, cur ast.Node) bool {
	// Consecutive var declarations stay grouped.
	if prev.Kind() == ast.KindVarDecl && cur.Kind() == ast.KindVarDecl {
		return false
	}

	if blankBetweenStmts(prev, cur) {
		return true
	}

	return isDeclaration(prev) || isDeclaration(cur)
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
