package format

// This file adds comment reproduction to the formatter. Comments live on
// ast.File.Comments in source order (the tokenizer captures them into a side
// list; see tokenizer/comment.go). The printer holds that list plus a monotonic
// cursor and interleaves comments with the tree by source line:
//
//   - Leading comments (a comment on a line before a statement) are emitted on
//     their own line, at the statement's indentation, just above it.
//   - A trailing comment (a comment on the same line a statement begins) is
//     emitted inline after the statement.
//   - Comments left inside a block after its last statement are flushed just
//     before the closing brace.
//
// Because the cursor only moves forward and containers are visited in source
// order, every comment is emitted exactly once and none are lost.
//
// Known rough edges (comments are always preserved, only their placement is
// approximate): a comment on the closing-brace line of a multi-line construct,
// or an inline block comment preceding a statement, may be moved to its own
// line; comments inside a struct/interface type body are not yet interleaved
// with the fields and surface at the next flush point. These are placement
// refinements, not losses. Search "TRIVIA:" for the broader follow-up notes.

// emitLeadingComments writes every pending comment positioned on a line before
// beforeLine, each on its own line at the current indentation.
func (p *printer) emitLeadingComments(beforeLine int) {
	for p.ci < len(p.comments) && p.comments[p.ci].Line < beforeLine {
		p.newline()
		p.write(p.comments[p.ci].Text)
		p.ci++
	}
}

// emitTrailingComment writes a single pending comment that sits on the given
// line, inline (after two spaces). It is used right after a statement to catch a
// comment that trailed it on the same source line.
func (p *printer) emitTrailingComment(line int) {
	if p.ci < len(p.comments) && p.comments[p.ci].Line == line {
		p.write("  " + p.comments[p.ci].Text)
		p.ci++
	}
}

// hasPendingComments reports whether any comments remain to be emitted.
func (p *printer) hasPendingComments() bool {
	return p.ci < len(p.comments)
}
