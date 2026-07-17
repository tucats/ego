package ast

// Comment is a single source comment carried alongside the tree. Comments are
// not attached to individual nodes; instead the whole set for a file is held on
// File.Comments in source order, and consumers (the formatter) correlate them
// with nodes by source position. This mirrors how the tokenizer captures them
// (a side list keyed by position) and keeps the node types free of comment
// bookkeeping.
type Comment struct {
	// Text is the comment exactly as written, including the leading "//" or the
	// surrounding "/*" and "*/".
	Text string

	// Line and Column are the 1-based position of the comment's first character.
	Line   int
	Column int

	// Block is true for a "/* ... */" comment, false for a "//" line comment.
	Block bool
}
