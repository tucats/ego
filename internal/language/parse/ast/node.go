// Package ast defines the public, extensible Abstract Syntax Tree (AST) node
// types for the Ego language. The tree is produced by the sibling package
// github.com/tucats/ego/internal/language/parse, and is intended to be consumed
// by tooling that needs a structural (rather than bytecode) view of Ego source
// — most immediately a canonical source formatter, and later additional
// compiler-support tools.
//
// # Design goals
//
//   - Faithful to the grammar. The node inventory maps directly onto the
//     productions in docs/SYNTAX.md. When the language grammar is extended, the
//     corresponding node type is added here and the parser gains one production.
//
//   - Public and extensible. Node is an open interface (it has no unexported
//     "sealing" method), so packages outside parse/ast may implement additional
//     node types and have them participate in generic traversal via Walk. The
//     Kind space reserves a range (see KindUserBase) for such external nodes.
//
//   - Cheap dispatch. Every node reports a Kind, a small integer tag that lets
//     consumers switch on node type without reflection or large type switches.
//
// # Comments / trivia (known gap)
//
// The Ego tokenizer is built on Go's scanner and discards comment tokens, so
// the AST produced today carries NO comment or whitespace trivia. This is an
// accepted limitation for the initial AST work. A future canonical formatter
// will need comments; when that work begins, the trail to follow is:
//
//   - internal/language/tokenizer/lexer.go — where Go's scanner is driven and
//     comments are currently dropped; comment tokens would have to be captured
//     here and threaded through the Tokenizer's token stream.
//   - This package — add a Comment node type and comment-attachment fields
//     (leading/trailing) on BaseNode so the formatter can re-emit them.
//   - internal/language/parse/parser.go — attach captured comment trivia to the
//     nearest node during parsing.
//
// Search the codebase for the string "TRIVIA:" to find every breadcrumb left
// for that follow-up task.
package ast

// Position identifies a location in the original source text. Line and Column
// are 1-based, matching tokenizer.Token.Location(). A zero Position (both
// fields 0) means the location is unknown — typically because the node was
// constructed synthetically rather than parsed from source.
type Position struct {
	Line   int
	Column int
}

// IsValid reports whether the position refers to a real source location.
func (p Position) IsValid() bool {
	return p.Line > 0
}

// Node is the interface implemented by every AST node. It is deliberately an
// open interface: any package may implement it to contribute new node types,
// which then participate in generic traversal through Children (and therefore
// Walk). Consumers that need to recognize a specific node type do so with a Go
// type switch or by comparing Kind.
type Node interface {
	// Pos returns the position of the first token of the node.
	Pos() Position

	// End returns the position just past the last token of the node. It is used
	// by tooling (such as a formatter) that needs the source span of a node. For
	// nodes where the end is not tracked, End returns the same value as Pos.
	End() Position

	// Kind returns the node's kind tag for cheap dispatch.
	Kind() Kind

	// Children returns the node's direct child nodes in source order. It never
	// returns nil entries; a node with no children returns an empty (or nil)
	// slice. Generic traversal (Walk) is built entirely on this method, so a
	// correct Children implementation is all an external node type needs to be
	// fully traversable.
	Children() []Node

	// String returns a short, human-readable description of the node, primarily
	// for debugging and test output. It is not a source-reconstruction; that is
	// the job of the (future) formatter.
	String() string
}

// BaseNode is an embeddable helper that supplies Pos/End storage and their
// accessor methods. Node implementations embed it to avoid repeating position
// bookkeeping. External node types may embed it too.
type BaseNode struct {
	Start  Position
	Finish Position
}

// Pos returns the node's start position.
func (b BaseNode) Pos() Position { return b.Start }

// End returns the node's end position, falling back to the start position when
// no distinct end was recorded.
func (b BaseNode) End() Position {
	if b.Finish.IsValid() {
		return b.Finish
	}

	return b.Start
}

// SetSpan records the start and end positions of the node. It is a convenience
// for parser code and external constructors.
func (b *BaseNode) SetSpan(start, end Position) {
	b.Start = start
	b.Finish = end
}
