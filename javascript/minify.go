// Package javascript provides utilities for working with JavaScript source code.
// Currently it includes a Minify function that compresses JavaScript by removing
// comments, collapsing whitespace, and renaming local declarations to shorter names.
package javascript

import (
	"fmt"
	"strings"
)

// tokenKind identifies the syntactic category of a JavaScript token.
type tokenKind int

const (
	tkWhitespace  tokenKind = iota
	tkLineComment           // // ...
	tkBlockComment          // /* ... */
	tkString                // 'x' or "x"
	tkTemplate              // `x`
	tkRegex                 // /pat/flags
	tkNumber                // 0, 3.14, 0xff
	tkIdentifier            // foo, _x, $y
	tkPunct                 // operators and punctuation
)

type jsToken struct {
	kind  tokenKind
	value string
}

// reserved contains JavaScript keywords and common globals that must never be renamed.
var reserved = map[string]bool{
	"arguments": true, "as": true, "async": true, "await": true,
	"break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true,
	"else": true, "export": true, "extends": true, "false": true, "finally": true,
	"for": true, "from": true, "function": true, "get": true,
	"if": true, "import": true, "in": true, "instanceof": true,
	"let": true, "new": true, "null": true, "of": true,
	"return": true, "set": true, "static": true, "super": true, "switch": true,
	"this": true, "throw": true, "true": true, "try": true,
	"typeof": true, "undefined": true, "var": true, "void": true,
	"while": true, "with": true, "yield": true,
	// common globals
	"Array": true, "Boolean": true, "console": true, "Date": true,
	"document": true, "Error": true, "eval": true, "Function": true,
	"Infinity": true, "JSON": true, "Map": true, "Math": true,
	"NaN": true, "Number": true, "Object": true, "Promise": true,
	"Proxy": true, "Reflect": true, "RegExp": true, "Set": true,
	"String": true, "Symbol": true, "TypeError": true, "WeakMap": true,
	"WeakRef": true, "WeakSet": true, "window": true, "globalThis": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
	"decodeURI": true, "encodeURI": true, "decodeURIComponent": true,
	"encodeURIComponent": true, "Uint8Array": true,
	// common property names that must not be touched
	"constructor": true, "length": true, "prototype": true,
	"toString": true, "valueOf": true, "hasOwnProperty": true,
}

// Minify accepts JavaScript source code and returns a minified version.
// It strips comments, collapses whitespace, and renames locally declared
// identifiers (var/let/const and function parameters) to compact names
// such as a, b, …, z, a1, b1, etc.
func Minify(src []byte) []byte {
	tokens := tokenize(src)
	tokens = stripComments(tokens)
	tokens = renameLocals(tokens)

	return emit(tokens)
}

// ── tokenizer ────────────────────────────────────────────────────────────────

func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == '$'
}

func isIdentCont(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// isRegexContext returns true when a '/' at the current position should be
// interpreted as the start of a regex literal rather than a division operator.
// It inspects the last non-whitespace token already collected.
func isRegexContext(prev []jsToken) bool {
	for i := len(prev) - 1; i >= 0; i-- {
		t := prev[i]
		if t.kind == tkWhitespace {
			continue
		}

		if t.kind == tkIdentifier {
			switch t.value {
			case "return", "typeof", "instanceof", "in", "of",
				"new", "delete", "throw", "void", "case":
				return true
			}

			return false
		}

		if t.kind == tkPunct {
			switch t.value {
			case "=", "(", "[", "!", "&", "&&", "|", "||",
				"?", ":", ",", ";", "{", "}", "=>", "??":
				return true
			}

			return false
		}

		return false
	}

	return true // start of file
}

// tokenize converts raw JavaScript source into a flat slice of tokens.
func tokenize(src []byte) []jsToken {
	var out []jsToken

	i, n := 0, len(src)

	for i < n {
		b := src[i]

		// ── whitespace ────────────────────────────────────────────────
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			j := i + 1
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
				j++
			}

			out = append(out, jsToken{tkWhitespace, string(src[i:j])})
			i = j

			continue
		}

		// ── line comment ──────────────────────────────────────────────
		if b == '/' && i+1 < n && src[i+1] == '/' {
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}

			out = append(out, jsToken{tkLineComment, string(src[i:j])})
			i = j

			continue
		}

		// ── block comment ─────────────────────────────────────────────
		if b == '/' && i+1 < n && src[i+1] == '*' {
			j := i + 2
			for j+1 < n && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}

			if j+1 < n {
				j += 2
			}

			out = append(out, jsToken{tkBlockComment, string(src[i:j])})
			i = j

			continue
		}

		// ── string literals ───────────────────────────────────────────
		if b == '\'' || b == '"' {
			quote := b

			j := i + 1
			for j < n {
				if src[j] == '\\' {
					j += 2

					continue
				}

				if src[j] == quote {
					j++

					break
				}

				j++
			}

			out = append(out, jsToken{tkString, string(src[i:j])})
			i = j

			continue
		}

		// ── template literal ──────────────────────────────────────────
		if b == '`' {
			j := i + 1
			for j < n {
				if src[j] == '\\' {
					j += 2

					continue
				}

				if src[j] == '`' {
					j++

					break
				}

				j++
			}
			
			out = append(out, jsToken{tkTemplate, string(src[i:j])})
			i = j

			continue
		}

		// ── regex literal ─────────────────────────────────────────────
		if b == '/' && isRegexContext(out) {
			j := i + 1
			inClass := false

			for j < n {
				if src[j] == '\\' {
					j += 2

					continue
				}
				
				if src[j] == '[' {
					inClass = true
				} else if src[j] == ']' {
					inClass = false
				} else if src[j] == '/' && !inClass {
					j++
					for j < n && isIdentCont(src[j]) { // flags
						j++
					}

					break
				}

				j++
			}

			out = append(out, jsToken{tkRegex, string(src[i:j])})
			i = j

			continue
		}

		// ── number ────────────────────────────────────────────────────
		if isDigit(b) || (b == '.' && i+1 < n && isDigit(src[i+1])) {
			j := i + 1
			for j < n {
				c := src[j]
				if isDigit(c) || c == '.' || c == '_' ||
					c == 'e' || c == 'E' ||
					c == 'x' || c == 'X' ||
					c == 'b' || c == 'B' ||
					c == 'o' || c == 'O' ||
					(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
					j++

					continue
				}

				if (c == '+' || c == '-') && j > 0 && (src[j-1] == 'e' || src[j-1] == 'E') {
					j++

					continue
				}

				break
			}

			out = append(out, jsToken{tkNumber, string(src[i:j])})
			i = j

			continue
		}

		// ── identifier / keyword ──────────────────────────────────────
		if isIdentStart(b) {
			j := i + 1
			for j < n && isIdentCont(src[j]) {
				j++
			}

			out = append(out, jsToken{tkIdentifier, string(src[i:j])})
			i = j

			continue
		}

		// ── multi-character operators ─────────────────────────────────
		if i+2 < n {
			three := string(src[i : i+3])
			switch three {
			case "===", "!==", ">>>", "**=", ">>=", "<<=", "&&=", "||=", "??=":
				out = append(out, jsToken{tkPunct, three})
				i += 3

				continue
			}
		}

		if i+1 < n {
			two := string(src[i : i+2])
			switch two {
			case "==", "!=", ">=", "<=", "&&", "||", "++", "--",
				"+=", "-=", "*=", "/=", "%=", "**", ">>", "<<",
				"??", "=>", "?.":
				out = append(out, jsToken{tkPunct, two})
				i += 2

				continue
			}
		}

		// single-character punctuation / operator
		out = append(out, jsToken{tkPunct, string(src[i : i+1])})
		i++
	}

	return out
}

// ── comment removal ───────────────────────────────────────────────────────────

func stripComments(tokens []jsToken) []jsToken {
	out := make([]jsToken, 0, len(tokens))

	for _, t := range tokens {
		if t.kind != tkLineComment && t.kind != tkBlockComment {
			out = append(out, t)
		}
	}

	return out
}

// ── local identifier renaming ────────────────────────────────────────────────

// nameGen returns a function that yields successive short names:
// a, b, …, z, a1, b1, …, z1, a2, etc.
func nameGen() func() string {
	n := 0

	const letters = "abcdefghijklmnopqrstuvwxyz"

	return func() string {
		defer func() { n++ }()

		if n < 26 {
			return string(letters[n])
		}

		cycle := (n - 26) / 26
		idx := (n - 26) % 26

		return fmt.Sprintf("%c%d", letters[idx], cycle+1)
	}
}

// collectLocals scans tokens for declarations made with var/let/const and for
// function parameter lists.  It returns a set of identifier names that are
// candidates for renaming.
func collectLocals(tokens []jsToken) map[string]bool {
	locals := map[string]bool{}
	n := len(tokens)

	for i := 0; i < n; i++ {
		t := tokens[i]
		if t.kind != tkIdentifier {
			continue
		}

		switch t.value {
		case "var", "let", "const":
			// Collect identifier(s) in this declaration.
			// Handles: var a, b, c = …
			// Handles simple destructuring: var {a, b} or var [a, b]
			i++
			for i < n {
				i = skipWhitespace(tokens, i)
				if i >= n {
					break
				}

				cur := tokens[i]
				if cur.kind == tkIdentifier && !reserved[cur.value] {
					locals[cur.value] = true
					i++
				} else if cur.kind == tkPunct && (cur.value == "{" || cur.value == "[") {
					// Simple destructuring: collect identifiers until matching close
					depth := 1
					i++

					for i < n && depth > 0 {
						if tokens[i].kind == tkPunct {
							switch tokens[i].value {
							case "{", "[":
								depth++
							case "}", "]":
								depth--
							}
						}

						if tokens[i].kind == tkIdentifier && depth > 0 && !reserved[tokens[i].value] {
							locals[tokens[i].value] = true
						}

						i++
					}
				} else {
					break
				}
				// skip optional '= expr' or type annotation up to ',' or ';' or end-of-decl
				i = skipWhitespace(tokens, i)
				if i >= n {
					break
				}

				if tokens[i].kind == tkPunct && tokens[i].value == "=" {
					// skip initializer
					i = skipInitializer(tokens, i+1)
				}

				i = skipWhitespace(tokens, i)
				if i >= n || tokens[i].kind != tkPunct || tokens[i].value != "," {
					break
				}

				i++ // consume ','
			}

		case "function":
			// function [name] ( params )
			i++

			i = skipWhitespace(tokens, i)
			if i >= n {
				break
			}
			// optional function name
			if tokens[i].kind == tkIdentifier && !reserved[tokens[i].value] {
				locals[tokens[i].value] = true
				i++
				i = skipWhitespace(tokens, i)
			}
			// collect parameter list
			if i < n && tokens[i].kind == tkPunct && tokens[i].value == "(" {
				i++
				i = collectParams(tokens, i, locals)
			}
		}
	}

	return locals
}

// skipWhitespace advances i past any whitespace tokens and returns the new index.
func skipWhitespace(tokens []jsToken, i int) int {
	for i < len(tokens) && tokens[i].kind == tkWhitespace {
		i++
	}

	return i
}

// skipInitializer advances past a variable initializer expression, stopping
// before a comma (at depth 0) or a semicolon/newline that ends the declaration.
func skipInitializer(tokens []jsToken, i int) int {
	depth := 0

	for i < len(tokens) {
		t := tokens[i]
		if t.kind == tkPunct {
			switch t.value {
			case "(", "[", "{":
				depth++

			case ")", "]", "}":
				if depth == 0 {
					return i
				}

				depth--

			case ",", ";":
				if depth == 0 {
					return i
				}
			}
		}

		i++
	}

	return i
}

// collectParams collects parameter identifiers from a function's parameter list,
// starting after the opening '(' (i points to the first token inside the parens).
func collectParams(tokens []jsToken, i int, locals map[string]bool) int {
	depth := 1
	for i < len(tokens) && depth > 0 {
		t := tokens[i]
		if t.kind == tkPunct {
			switch t.value {
			case "(":
				depth++

			case ")":
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}

		if t.kind == tkIdentifier && !reserved[t.value] {
			locals[t.value] = true
		}

		i++
	}

	return i
}

// renameLocals collects locally declared identifiers and rewrites them to
// shorter names, leaving property accesses (identifiers preceded by '.') and
// all string/template contents unchanged.
func renameLocals(tokens []jsToken) []jsToken {
	locals := collectLocals(tokens)
	if len(locals) == 0 {
		return tokens
	}

	// Collect every identifier already present in the source so that generated
	// names do not collide with existing ones.
	existing := map[string]bool{}

	for _, t := range tokens {
		if t.kind == tkIdentifier {
			existing[t.value] = true
		}
	}

	// Build rename map: original → short name
	next := nameGen()
	rename := map[string]string{}

	for name := range locals {
		short := next()
		for existing[short] {
			short = next()
		}

		existing[short] = true
		rename[name] = short
	}

	// Apply rename, but skip identifiers that are accessed as properties.
	out := make([]jsToken, len(tokens))
	copy(out, tokens)

	for i, t := range out {
		if t.kind != tkIdentifier {
			continue
		}

		if short, ok := rename[t.value]; ok {
			// Check if this identifier is the right-hand side of a '.' access.
			prev := prevNonWS(out, i)
			if prev >= 0 && out[prev].kind == tkPunct && out[prev].value == "." {
				continue
			}

			out[i].value = short
		}
	}

	return out
}

// prevNonWS returns the index of the nearest non-whitespace token before i,
// or -1 if none exists.
func prevNonWS(tokens []jsToken, i int) int {
	for j := i - 1; j >= 0; j-- {
		if tokens[j].kind != tkWhitespace {
			return j
		}
	}

	return -1
}

// ── output reconstruction ─────────────────────────────────────────────────────

// needsSep returns true when a space must be inserted between two adjacent
// token values to prevent them from merging into a different token.
func needsSep(left, right string) bool {
	if left == "" || right == "" {
		return false
	}

	l := left[len(left)-1]
	r := right[0]

	// Two identifier/keyword/number characters would merge without a space.
	if isIdentCont(l) && isIdentCont(r) {
		return true
	}
	// Prevent ++ becoming +++ or -- becoming ---
	if l == '+' && r == '+' {
		return true
	}

	if l == '-' && r == '-' {
		return true
	}

	// Prevent / followed by * producing a block-comment start.
	if l == '/' && r == '*' {
		return true
	}

	return false
}

// emit reconstructs the token stream into a compact []byte, collapsing all
// whitespace to nothing (or a single space where tokens would otherwise merge).
func emit(tokens []jsToken) []byte {
	var sb strings.Builder

	lastVal := ""

	for _, t := range tokens {
		if t.kind == tkWhitespace {
			continue
		}

		if needsSep(lastVal, t.value) {
			sb.WriteByte(' ')
		}

		sb.WriteString(t.value)
		lastVal = t.value
	}

	return []byte(sb.String())
}
