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
// It always strips comments and collapses whitespace. When shortenNames is
// true it also renames locally declared identifiers (var/let/const and
// function parameters) to compact names such as a, b, …, z, a1, b1, etc.
// Pass false to preserve original identifier names — useful when source maps
// or human-readable output is still needed.
func Minify(src []byte, shortenNames bool) []byte {
	tokens := tokenize(src)
	tokens = stripComments(tokens)

	if shortenNames {
		tokens = renameLocals(tokens)
	}

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
					c == 'n' || // BigInt literal suffix, e.g. 0n, 123n, 0x1Fn
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
			case "===", "!==", ">>>", "**=", ">>=", "<<=", "&&=", "||=", "??=", "...":
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
// function parameter lists.  It returns two sets of identifier names:
//
//	locals    — names that are safe to rename: declarations made inside a
//	            braced block, plus function parameters, which are local to
//	            their function wherever that function was declared.
//	fileScope — names bound at file scope: var/let/const, function and class
//	            declarations that appear outside any braced block.
//
// Nothing in fileScope may be renamed, for two independent reasons:
//
//  1. A file-scope name is reachable from outside the file the minifier can
//     see — from an inline onclick/onchange attribute in the HTML, or from
//     another <script> sharing the same global scope. Renaming one silently
//     breaks a caller that was never in the minifier's hands.
//  2. Each asset is minified independently (see the assets handler), so two
//     files renaming on their own counters would not agree on the result.
//
// Function declaration names were already left out of the rename for reason 1,
// but that protection held only while no *other* declaration shared the name:
// the rename map is keyed by name alone, so a parameter called "showSettings"
// anywhere in the file would drag a top-level showSettings() into the rename
// along with it. Returning the file-scope set explicitly, and subtracting it
// in renameLocals, closes that hole.
func collectLocals(tokens []jsToken) (map[string]bool, map[string]bool) {
	locals := map[string]bool{}
	fileScope := map[string]bool{}
	n := len(tokens)
	depth := 0

	for i := 0; i < n; i++ {
		t := tokens[i]

		// Track braced-block nesting: a declaration is at file scope exactly
		// when no '{' encloses it. Only braces are counted — parentheses and
		// brackets do not introduce a scope that matters here.
		//
		// This depends on the loop never stepping over a brace, which is why
		// the two places below that skip a region hand back an index one short
		// of where they stopped: the loop's own i++ then lands exactly on the
		// token they stopped at, rather than one past it.
		if t.kind == tkPunct {
			switch t.value {
			case "{":
				depth++

			case "}":
				if depth > 0 {
					depth--
				}
			}

			continue
		}

		if t.kind != tkIdentifier {
			continue
		}

		// Declarations outside every brace are off-limits; anything deeper is
		// a genuine local and may be renamed.
		target := locals
		if depth == 0 {
			target = fileScope
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
					target[cur.value] = true
					i++
				} else if cur.kind == tkPunct && (cur.value == "{" || cur.value == "[") {
					// Simple destructuring: collect identifiers until matching
					// close. `nest` is this pattern's own bracket counter and
					// is deliberately not the enclosing `depth`: the pattern is
					// balanced, so it leaves the block nesting unchanged.
					nest := 1
					i++

					for i < n && nest > 0 {
						if tokens[i].kind == tkPunct {
							switch tokens[i].value {
							case "{", "[":
								nest++
							case "}", "]":
								nest--
							}
						}

						if tokens[i].kind == tkIdentifier && nest > 0 && !reserved[tokens[i].value] {
							target[tokens[i].value] = true
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

			// Step back one so the loop's i++ lands on the token the
			// declaration stopped at rather than one past it. That token may
			// be the '}' closing the enclosing block, and skipping it would
			// leave `depth` permanently too deep.
			i--

		case "function", "class":
			// function [name] ( params )   /   class [name] { … }
			//
			// The declared name is never a rename candidate: it is either at
			// file scope, where it is recorded as off-limits below, or nested,
			// where it is still reachable by name from anywhere in its
			// enclosing scope. Only the parameter names are truly local.
			i++

			i = skipWhitespace(tokens, i)
			if i >= n {
				break
			}

			// Record the optional declared name. Recording it is what stops a
			// same-named local elsewhere in the file from dragging it into the
			// rename map; see the note on this function.
			if tokens[i].kind == tkIdentifier && !reserved[tokens[i].value] {
				if depth == 0 {
					fileScope[tokens[i].value] = true
				}

				i++
				i = skipWhitespace(tokens, i)
			}

			// collect parameter list ("class" has none, so this is skipped for it)
			if i < n && tokens[i].kind == tkPunct && tokens[i].value == "(" {
				i++
				// Same step-back as above: collectParams returns the index
				// after the ')', and the token there is very often the '{'
				// that opens the body.
				i = collectParams(tokens, i, locals) - 1
			}
		}
	}

	return locals, fileScope
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
	locals, fileScope := collectLocals(tokens)

	// Anything bound at file scope is removed outright, even when the same
	// name is also declared as a local somewhere: the rename map is keyed by
	// name, so renaming it for the local's sake would rename the file-scope
	// binding too. Losing a rename costs a few bytes; renaming a name another
	// file or an inline HTML handler refers to breaks the page.
	for name := range fileScope {
		delete(locals, name)
	}

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

	// Apply rename. Build a new result slice to allow token insertion when
	// expanding ES6 shorthand properties.
	//
	// A context stack tracks the innermost bracket so we can distinguish object
	// literals '{' from parameter lists '(' and array literals '['. Object-literal
	// rules (key protection, shorthand expansion) only apply inside '{'.
	result := make([]jsToken, 0, len(tokens))

	var ctxStack []byte // innermost bracket: '{', '(', or '['

	for i, t := range tokens {
		// Maintain the context stack on every bracket token.
		if t.kind == tkPunct {
			switch t.value {
			case "{", "(", "[":
				ctxStack = append(ctxStack, t.value[0])
			case "}", ")", "]":
				if len(ctxStack) > 0 {
					ctxStack = ctxStack[:len(ctxStack)-1]
				}
			}
		}

		if t.kind != tkIdentifier {
			result = append(result, t)

			continue
		}

		short, ok := rename[t.value]
		if !ok {
			result = append(result, t)

			continue
		}

		prevIdx := prevNonWS(result, len(result))
		nextIdx := nextNonWS(tokens, i)

		// Skip identifiers on the right side of a '.' or '?.' (property read,
		// e.g. obj.foo or obj?.foo). '?.' is tokenized as a single two-character
		// punctuation token (see the multi-character operator list in tokenize()),
		// so it must be checked alongside the plain '.' case — otherwise a
		// property name following optional chaining is mistaken for a bare
		// identifier and renamed if it happens to match an unrelated local
		// variable name elsewhere in the file.
		if prevIdx >= 0 && result[prevIdx].kind == tkPunct &&
			(result[prevIdx].value == "." || result[prevIdx].value == "?.") {
			result = append(result, t)

			continue
		}

		// Object-literal rules only apply when the immediately enclosing bracket is '{'.
		// This prevents false positives in parameter lists '(a, b)' and arrays '[a, b]'.
		inObjCtx := len(ctxStack) > 0 && ctxStack[len(ctxStack)-1] == '{'

		// Within an object literal, also require the previous non-WS token to be
		// '{' or ',' to avoid misidentifying ternary values (a ? x : y) or return
		// statements without semicolons (return x}) as property positions.
		prevIsBraceOrComma := prevIdx >= 0 && result[prevIdx].kind == tkPunct &&
			(result[prevIdx].value == "{" || result[prevIdx].value == ",")

		if inObjCtx && prevIsBraceOrComma {
			nextIsColon := nextIdx >= 0 && tokens[nextIdx].kind == tkPunct && tokens[nextIdx].value == ":"
			nextIsClose := nextIdx >= 0 && tokens[nextIdx].kind == tkPunct &&
				(tokens[nextIdx].value == "," || tokens[nextIdx].value == "}")

			// Skip explicit property keys: {key: value}. The key must not be renamed.
			if nextIsColon {
				result = append(result, t)

				continue
			}

			// Expand ES6 shorthand properties: {ident} / {ident, ...}.
			// Renaming the token alone would change the property key seen by callers.
			// Expand to {ident: short} to preserve the key while renaming the value.
			if nextIsClose {
				result = append(result, t)                                   // key: original name
				result = append(result, jsToken{kind: tkPunct, value: ":"}) // ':'
				result = append(result, jsToken{kind: tkIdentifier, value: short})
				
				continue
			}
		}

		result = append(result, jsToken{kind: tkIdentifier, value: short})
	}

	return result
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

// nextNonWS returns the index of the nearest non-whitespace token after i,
// or -1 if none exists.
func nextNonWS(tokens []jsToken, i int) int {
	for j := i + 1; j < len(tokens); j++ {
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
