package format

import "strings"

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
		p.writeComment(p.comments[p.ci].Text)
		p.ci++
	}
}

// emitTrailingComment writes a single pending comment that sits on the given
// line, inline (after two spaces). It is used right after a statement to catch a
// comment that trailed it on the same source line.
func (p *printer) emitTrailingComment(line int) {
	if p.ci < len(p.comments) && p.comments[p.ci].Line == line {
		p.write("  ")
		p.writeComment(p.comments[p.ci].Text)
		p.ci++
	}
}

// writeComment writes a comment, re-indenting the interior lines of a multi-line
// (block) comment to the current indentation. splitLines strips the original
// interior indentation during tokenization, so this restores a consistent block
// indent rather than leaving those lines at column zero.
//
// A "star comment" — one whose every continuation line begins with "*" — is
// aligned as gofmt does: each "*" sits one column to the right of the "/*"
// opener, so the stars line up beneath the "*" of "/*".
func (p *printer) writeComment(text string) {
	if !strings.Contains(text, "\n") {
		p.write(text)

		return
	}

	lines := strings.Split(text, "\n")
	indent := p.indentation()
	star := isStarComment(lines)

	// The first line is written at the cursor (already positioned by the
	// caller); continuation lines get their own indentation.
	p.write(lines[0])

	for _, line := range lines[1:] {
		p.write("\n")

		if star {
			// line already begins with "*" (splitLines trimmed leading space);
			// the extra space aligns it under the "*" of "/*".
			p.write(indent + " " + line)
		} else {
			p.write(indent + line)
		}
	}
}

// isStarComment reports whether a split block comment is a "star comment": every
// line after the first, ignoring leading whitespace, begins with "*". Such
// comments (including the closing "*/" line) get their stars aligned.
func isStarComment(lines []string) bool {
	if len(lines) < 2 {
		return false
	}

	for _, line := range lines[1:] {
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "*") {
			return false
		}
	}

	return true
}

// hasPendingComments reports whether any comments remain to be emitted.
func (p *printer) hasPendingComments() bool {
	return p.ci < len(p.comments)
}
