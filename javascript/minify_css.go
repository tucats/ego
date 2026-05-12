package javascript

// MinifyCSS removes block comments and collapses unnecessary whitespace from
// CSS source. The following transformations are applied:
//
//   - /* ... */ block comments are stripped entirely.
//   - Runs of whitespace (spaces, tabs, newlines) are collapsed to a single
//     space, or to nothing when they border a CSS delimiter.
//   - Spaces are removed before and after { } ; , >.
//   - Spaces are removed after : (property separator), but preserved before :
//     so that descendant + pseudo-class selectors such as "a :hover" are not
//     accidentally collapsed to "a:hover" (different semantics).
//   - Trailing semicolons before } are dropped (redundant in CSS).
//   - Consecutive semicolons are collapsed to one.
//   - Whitespace inside quoted strings is preserved verbatim.
func MinifyCSS(src []byte) []byte {
	out := make([]byte, 0, len(src))
	i, n := 0, len(src)

	for i < n {
		c := src[i]

		// Block comment /* ... */ — discard entirely.
		if c == '/' && i+1 < n && src[i+1] == '*' {
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}

			i += 2 // skip closing */

			continue
		}

		// Quoted strings — copy verbatim, handling backslash escapes.
		if c == '"' || c == '\'' {
			out = append(out, c)

			i++
			for i < n && src[i] != c {
				if src[i] == '\\' && i+1 < n {
					out = append(out, src[i], src[i+1])
					i += 2

					continue
				}

				out = append(out, src[i])
				i++
			}

			if i < n {
				out = append(out, src[i]) // closing quote
				i++
			}

			continue
		}

		// Whitespace run — collapse to a single space, or nothing at a delimiter.
		if cssIsWS(c) {
			for i < n && cssIsWS(src[i]) {
				i++
			}

			// Drop the space if the next real character is a delimiter.
			nextIsDelim := i < n && cssIsDelim(src[i])

			// Drop the space if the previous output character is a delimiter or a colon.
			// (Handles both "color: red" → "color:red" and "a, b" → "a,b".)
			prevIsDelimOrColon := len(out) > 0 && (cssIsDelim(out[len(out)-1]) || out[len(out)-1] == ':')
			if !nextIsDelim && !prevIsDelimOrColon && len(out) > 0 {
				out = append(out, ' ')
			}

			continue
		}

		// Semicolons — collapse duplicates and drop the one before '}'.
		if c == ';' {
			// Consume any immediately following semicolons.
			for i+1 < n && src[i+1] == ';' {
				i++
			}

			// Scan ahead (past whitespace) to see if the next real token is '}'.
			j := i + 1
			for j < n && cssIsWS(src[j]) {
				j++
			}

			if j < n && src[j] == '}' {
				i++ // drop this redundant semicolon
				
				continue
			}

			out = append(out, c)
			i++

			continue
		}

		out = append(out, c)
		i++
	}

	// Trim any trailing space left by a final whitespace run.
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}

	return out
}

// cssIsWS reports whether b is a CSS whitespace character.
func cssIsWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// cssIsDelim reports whether b is a CSS delimiter around which spaces are
// always safe to remove: { } ; , >
// Note: ':' is intentionally excluded — spaces before ':' may be significant
// in descendant-combinator + pseudo-class selectors (e.g. "a :hover").
func cssIsDelim(b byte) bool {
	return b == '{' || b == '}' || b == ';' || b == ',' || b == '>'
}
