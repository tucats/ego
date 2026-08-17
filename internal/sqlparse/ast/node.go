// Package ast defines the public, extensible Abstract Syntax Tree (AST) node
// types produced by parsing a single SQL statement. The tree is produced by
// the sibling package github.com/tucats/ego/internal/sqlparse, and is meant
// to be consumed by tooling that needs a structural view of SQL text rather
// than a raw string — most immediately a canonical SQL formatter, and later
// additional tools that inspect or rewrite statements (e.g. rewriting table
// names, extracting referenced columns, or building an execution plan).
//
// # Design goals
//
//   - Dialect aware, not dialect specific. The parser accepts sqlite3 and
//     PostgreSQL source text and produces the same node types for constructs
//     both dialects share. Where a construct is dialect-specific (e.g.
//     PostgreSQL's RETURNING clause, which sqlite3 also now supports) it is
//     still represented with a plain node; it is the parser's job to decide
//     whether the construct is legal for the requested Dialect, not the
//     node type's.
//
//   - Public and extensible. Node is an open interface (it has no unexported
//     "sealing" method), so packages outside sqlparse/ast may implement
//     additional node types and have them participate in generic traversal
//     via Walk. The Kind space reserves a range (see KindUserBase) for such
//     external nodes.
//
//   - Cheap dispatch. Every node reports a Kind, a small integer tag that
//     lets consumers switch on node type without reflection or large type
//     switches.
//
//   - Syntax only. The parser does not validate function names, table
//     existence, or column types — those are extension points that vary by
//     backend and by installed extensions. The tree records what was
//     written, not what it means.
//
// # For readers new to Go
//
// This package leans on three Go idioms that are worth naming explicitly if
// you haven't run into them before:
//
//  1. Interfaces are satisfied implicitly. Node (below) is just a list of
//     methods. Any type — anywhere, in any package — that happens to define
//     all of those methods automatically "is a" Node. There is no "implements
//     Node" declaration anywhere; the compiler works it out from the method
//     set. This is why external packages can add their own node types (see
//     "Public and extensible" above) without this package knowing about them.
//
//  2. Embedding stands in for inheritance. Go has no class hierarchies. What
//     it has is embedding: a struct can name another struct as a field with
//     no field name (see BaseNode below), and the outer struct then "inherits"
//     the inner one's methods — Go calls this "method promotion". Every
//     concrete node type (Ident, BinaryExpr, SelectStmt, ...) embeds BaseNode
//     purely so it gets working Pos()/End() methods for free instead of
//     writing the same four lines in every type. It's composition, not
//     subclassing: BaseNode doesn't know or care what embeds it.
//
//  3. An unexported method can "seal" an interface. Statement (below) embeds
//     Node and adds one more method, statementNode(), whose name starts with
//     a lowercase letter. Because unexported identifiers are only visible
//     within the package that declares them, no type outside this ast
//     package can implement statementNode() — and therefore no type outside
//     this package can satisfy Statement, even though it's a perfectly
//     ordinary, otherwise-public interface. This is the standard Go trick for
//     restricting "which of my types can play this role" without a runtime
//     check.
package ast

// Position identifies a location in the original source text. Line and
// Column are both 1-based. A zero Position (both fields 0) means the
// location is unknown — typically because the node was constructed
// synthetically rather than parsed from source.
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
// which then participate in generic traversal through Children (and
// therefore Walk). Consumers that need to recognize a specific node type do
// so with a Go type switch or by comparing Kind.
//
// Every concrete node type below (Ident, BinaryExpr, SelectStmt, and so on)
// satisfies Node simply by having these five methods — nothing declares that
// relationship explicitly. See "For readers new to Go" in the package doc
// comment above if that's unfamiliar.
type Node interface {
	// Pos returns the position of the first token of the node.
	Pos() Position

	// End returns the position just past the last token of the node. It is
	// used by tooling (such as a formatter) that needs the source span of a
	// node. For nodes where the end is not tracked, End returns the same
	// value as Pos.
	End() Position

	// Kind returns the node's kind tag for cheap dispatch.
	Kind() Kind

	// Children returns the node's direct child nodes in source order. It
	// never returns nil entries; a node with no children returns an empty
	// (or nil) slice. Generic traversal (Walk) is built entirely on this
	// method, so a correct Children implementation is all an external node
	// type needs to be fully traversable.
	Children() []Node

	// String returns a short, human-readable description of the node,
	// primarily for debugging and test output. It is not a
	// source-reconstruction; that is the job of the formatter.
	String() string
}

// Statement is the marker interface implemented by every node that can be
// the root of a parsed SQL statement (as opposed to a sub-expression such as
// a WHERE clause fragment). Parse returns a Statement.
type Statement interface {
	Node

	// statementNode is unexported so that only types defined in this package
	// can be top-level statements; sub-expressions and clauses remain plain
	// Node values.
	statementNode()
}

// BaseNode is an embeddable helper that supplies Pos/End storage and their
// accessor methods. Node implementations embed it (as an unnamed field —
// see "Embedding stands in for inheritance" in the package doc comment) to
// avoid repeating position bookkeeping in every node type. A type that
// embeds BaseNode picks up Pos(), End(), and SetSpan() automatically and so
// only has to write its own Kind(), Children(), and String(). External node
// types may embed it too.
type BaseNode struct {
	Start  Position
	Finish Position
}

// Pos returns the node's start position. This has a value receiver ("b
// BaseNode", not "b *BaseNode") because reading two ints is cheap and the
// method never needs to modify the node — see SetSpan below for the
// pointer-receiver counterpart that does need to modify it.
func (b BaseNode) Pos() Position { return b.Start }

// End returns the node's end position, falling back to the start position
// when no distinct end was recorded.
func (b BaseNode) End() Position {
	if b.Finish.IsValid() {
		return b.Finish
	}

	return b.Start
}

// SetSpan records the start and end positions of the node. It takes a
// pointer receiver ("b *BaseNode") because, unlike Pos and End, it mutates
// the struct — a value receiver would only modify a throwaway copy. Every
// parser function that finishes building a node calls n.SetSpan(start,
// p.here()) as its last step before returning; see sqlparse/parser.go.
func (b *BaseNode) SetSpan(start, end Position) {
	b.Start = start
	b.Finish = end
}

// BaseStmt is embedded (in addition to BaseNode) by every statement-level
// node to satisfy the Statement marker interface. Embedding BaseNode inside
// BaseStmt, and then BaseStmt inside e.g. SelectStmt, chains the method
// promotion two levels deep: SelectStmt ends up with Pos()/End()/SetSpan()
// from BaseNode and statementNode() from BaseStmt, without writing any of
// them itself.
type BaseStmt struct {
	BaseNode
}

// statementNode has an empty body — it exists only so that any type
// embedding BaseStmt satisfies the Statement interface's unexported method
// requirement. Nothing ever calls it directly.
func (BaseStmt) statementNode() {}

// Dialect selects the SQL dialect a source string is parsed as. Most syntax
// is shared between sqlite3 and PostgreSQL; the dialect only changes which
// dialect-specific constructs (if any) are accepted or how a small number of
// ambiguous forms are interpreted.
type Dialect int

const (
	// DialectSQLite parses source as sqlite3 SQL.
	DialectSQLite Dialect = iota

	// DialectPostgreSQL parses source as PostgreSQL SQL.
	DialectPostgreSQL
)

// String returns the name of the dialect.
func (d Dialect) String() string {
	switch d {
	case DialectSQLite:
		return "sqlite3"
	case DialectPostgreSQL:
		return "postgres"
	default:
		return "Dialect(" + itoa(int(d)) + ")"
	}
}
