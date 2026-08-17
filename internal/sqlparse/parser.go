// Package sqlparse converts a single SQL statement's source text into an
// Abstract Syntax Tree (AST) as defined in the sibling package
// github.com/tucats/ego/internal/sqlparse/ast. It is intended to be paired
// with future sibling packages that format an AST back to canonical SQL
// text and that operate on the tree (e.g. rewriting identifiers, extracting
// referenced tables/columns), the same way
// github.com/tucats/ego/internal/language/parse is paired with its
// ast/format/resolve siblings.
//
// The parser accepts both the sqlite3 and PostgreSQL dialects; Parse takes
// an ast.Dialect that selects which one governs the handful of constructs
// where the two disagree (see ast.Dialect). Most of the grammar — SELECT,
// INSERT, UPDATE, DELETE, CREATE/DROP/ALTER TABLE, CREATE/DROP INDEX,
// CREATE/DROP VIEW, and transaction control — is shared and dialect-neutral.
//
// This is a syntax-only parser: it does not validate that a referenced
// function, table, or column exists, or that a function is called with the
// right number or type of arguments — functions in particular may be
// dialect or user extensions the parser has no knowledge of. PRAGMA
// statements are not part of the supported grammar and are rejected with a
// syntax error (they are a sqlite3-specific configuration mechanism, not a
// statement in the usual sense).
//
// Syntax errors carry the offending token's line and column, in the same
// style as the Ego language parser (internal/language/parse): see
// errors.ErrSQLSyntax and friends in internal/errors/messages.go.
//
// # How this parser works
//
// This is a hand-written "recursive descent" parser. That name describes a
// specific, very common technique: the SQL grammar is a set of rules like
// "a SELECT statement is the keyword SELECT, then a list of result columns,
// then optionally a FROM clause, then optionally a WHERE clause, ...", and
// each such rule is written as one ordinary Go method on parser, named
// after the rule (parseSelectCore, parseResultColumn, parseFromClause, and
// so on, spread across parser.go, select.go, expr.go, dml.go, ddl.go, and
// txn.go). A rule's method calls the methods for the sub-rules it's made
// of, which is where "descent" comes from: parsing a SELECT statement
// descends into parsing its FROM clause, which descends into parsing a
// table reference, and so on down to individual tokens. There's no grammar
// file and no generated code (contrast with parser generators like yacc or
// ANTLR) — the grammar exists only as the shape of these Go functions
// calling each other, which is what makes it "hand-written".
//
// The parser reads from the flat token slice the lexer already produced
// (see lexer.go) through a small cursor, analogous to the lexer's own rune
// cursor one level down: cur() looks at the token under the cursor without
// moving it, peek(n) looks n tokens further ahead without moving anything,
// and next() reads the current token and advances. Every parseX method
// leaves the cursor sitting on the first token *after* whatever it just
// parsed, which is what lets the caller immediately continue parsing from
// where the callee left off.
//
// On top of that cursor, nearly every parsing function in this package is
// built from two families of helper, and recognizing this naming
// convention makes the rest of the package much easier to read:
//
//   - acceptX (acceptKeyword, acceptOp, acceptPunct, ...): "is the next
//     token an X? If so, consume it and report true; if not, consume
//     nothing and report false." No error is possible — acceptX is for
//     grammar elements that are genuinely optional, like the DISTINCT in
//     "SELECT DISTINCT ...". Calling code typically uses it in an
//     "if p.acceptKeyword(...)" or bare statement when it doesn't even need
//     to know whether it matched.
//
//   - expectX (expectKeyword, expectPunct, expectIdent, ...): "the grammar
//     requires an X at exactly this point." It consumes the token and
//     returns nil on success, or — if the token isn't there — leaves the
//     cursor alone and returns a syntax error. This is for grammar elements
//     that are mandatory once you've committed to a rule: once parser code
//     has decided it's looking at a CAST expression (because it saw the
//     keyword CAST), the opening "(" that must follow is parsed with
//     expectPunct("("), not acceptPunct, because its absence is a genuine
//     syntax error rather than a sign that some other rule should be tried
//     instead.
//
// Errors are plain Go values, not exceptions: every parseX method returns
// (result, error), and the standard "if err != nil { return nil, err }"
// after each sub-call is how a failure deep in the recursion unwinds back
// out to the top-level Parse call — there's no panic/recover in the normal
// path. See syntaxError below for how the error itself is put together.
//
// Because this parser never backtracks (it never "un-reads" a token after
// deciding what to do with it), any point where the grammar could go more
// than one way from the same starting keyword needs enough lookahead to
// decide up front, without consuming anything, which rule applies.
// isKeywordAt(offset, word) exists for exactly this: for example "IS" is
// ambiguous — plain "IS", "IS NULL", "IS NOT ...", and "IS DISTINCT FROM"
// all start with the same token — so the code parsing it peeks ahead before
// committing to which shape it's building (see the "is" case in
// parseComparisonExpr, expr.go).
package sqlparse

import (
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

// parser holds the state for a single parse. It is not safe for concurrent
// use; construct a new parser (via newParser) per source string.
type parser struct {
	toks    []token
	pos     int
	dialect ast.Dialect
}

// Parse parses source as a single SQL statement in the given dialect and
// returns its AST. On a syntax error the returned error carries the
// offending token's line and column (see package doc).
func Parse(source string, dialect ast.Dialect) (ast.Statement, error) {
	p, err := newParser(source, dialect)
	if err != nil {
		return nil, err
	}

	stmt, err := p.parseStatement()
	if err != nil {
		return nil, err
	}

	// A single optional trailing terminator is tolerated.
	p.acceptPunct(";")

	if !p.atEnd() {
		return nil, p.syntaxError("end of statement")
	}

	return stmt, nil
}

func newParser(source string, dialect ast.Dialect) (*parser, error) {
	toks, err := newLexer(source, dialect).tokenize()
	if err != nil {
		return nil, err
	}

	return &parser{toks: toks, dialect: dialect}, nil
}

// --- token stream primitives ---.
//
// cur/peek/next below are the parser's cursor over the token slice — see
// "How this parser works" in the package doc comment for the bigger
// picture. p.pos is the index of the next unconsumed token, exactly like
// lexer.pos is the index of the next unconsumed rune in lexer.go.

// cur returns the token under the cursor without consuming it. It's just a
// readable shorthand for peek(0), used constantly throughout this package
// (e.g. every isKeyword/acceptX check starts by looking at p.cur()).
func (p *parser) cur() token {
	return p.peek(0)
}

// peek returns the token offset positions past the cursor without moving
// the cursor. peek(0) is the same as cur(); peek(1) is one token further
// ahead, and so on. Running off either end of the token slice yields a
// synthetic tokEOF token rather than panicking, so callers never need a
// separate bounds check before peeking.
func (p *parser) peek(offset int) token {
	i := p.pos + offset
	if i < 0 || i >= len(p.toks) {
		return token{kind: tokEOF}
	}

	return p.toks[i]
}

// next returns the token under the cursor and moves the cursor one token
// forward (except at EOF, where there's nowhere further to advance to).
// This is the only place that actually consumes a token; every acceptX
// helper below is ultimately built on a call to next().
func (p *parser) next() token {
	t := p.cur()
	if t.kind != tokEOF {
		p.pos++
	}

	return t
}

func (p *parser) atEnd() bool {
	return p.cur().kind == tokEOF
}

func (p *parser) here() ast.Position {
	t := p.cur()

	return ast.Position{Line: t.line, Column: t.col}
}

// isKeyword reports whether the current token is the (unquoted) keyword
// spelling word, without consuming it.
func (p *parser) isKeyword(word string) bool {
	return p.cur().is(word)
}

// isKeywordAt reports whether the token offset positions ahead is the
// keyword word, without consuming anything. Used for short lookahead when a
// leading keyword needs a following word to disambiguate (e.g. "IS NOT").
func (p *parser) isKeywordAt(offset int, word string) bool {
	return p.peek(offset).is(word)
}

// acceptKeyword consumes and returns true if the current token is the
// keyword word; otherwise it leaves the position unchanged and returns
// false. See "acceptX" in the package doc comment for the general pattern.
func (p *parser) acceptKeyword(word string) bool {
	if p.isKeyword(word) {
		p.next()

		return true
	}

	return false
}

// acceptOp is acceptKeyword's counterpart for operators ("=", "||", "+", ...).
func (p *parser) acceptOp(op string) bool {
	if p.cur().isOp(op) {
		p.next()

		return true
	}

	return false
}

// acceptPunct is identical to acceptOp — token.isOp treats operators and
// punctuation ("(", ")", ",", ".", ";") as the same kind of check, matching
// on exact text — but call sites use whichever name reads better for what
// they're matching (acceptPunct("(") vs. acceptOp("=")). It's a naming
// convenience, not a different behavior; don't be surprised to find the two
// are interchangeable if you go looking.
func (p *parser) acceptPunct(s string) bool {
	if p.cur().isOp(s) {
		p.next()

		return true
	}

	return false
}

// expectKeyword is acceptKeyword's mandatory counterpart: see "expectX" in
// the package doc comment. strings.ToUpper(word) is passed to syntaxError so
// the error names the keyword the way a SQL author would recognize it
// ("expected FROM"), even though the parser's own comparison is
// case-insensitive and word is written lowercase at every call site.
func (p *parser) expectKeyword(word string) error {
	if p.acceptKeyword(word) {
		return nil
	}

	return p.syntaxError(strings.ToUpper(word))
}

// expectPunct is acceptPunct's mandatory counterpart.
func (p *parser) expectPunct(s string) error {
	if p.acceptPunct(s) {
		return nil
	}

	return p.syntaxError("'" + s + "'")
}

// expectOp is acceptOp's mandatory counterpart.
func (p *parser) expectOp(s string) error {
	if p.acceptOp(s) {
		return nil
	}

	return p.syntaxError("'" + s + "'")
}

// expectIdent consumes and returns the current token's text if it is an
// identifier (bare or quoted); otherwise it returns a syntax error. Keyword
// spellings are accepted here too — SQL keywords are not fully reserved, so
// e.g. a column or table can be named "type" or "value" in most contexts.
func (p *parser) expectIdent() (string, error) {
	t := p.cur()
	if t.kind != tokIdent {
		return "", p.syntaxError("identifier")
	}

	p.next()

	return t.text, nil
}

// syntaxError builds a located ErrSQLSyntax describing the current token as
// unexpected, optionally naming what was expected instead. Every expectX
// helper above funnels through this on failure, which is why every syntax
// error in this package reads "unexpected <token>, expected <thing>" — see
// e.g. TestSyntaxErrorPosition in parser_test.go for what the resulting
// message and located error look like from the caller's side.
//
// errors.New(...).Context(...).At(line, col) is this codebase's shared
// error-construction API (package internal/errors, used the same way by the
// Ego language parser in internal/language/parse): New wraps a predefined
// error identity (ErrSQLSyntax), Context attaches the human-readable detail
// message, and At attaches the source position. The result still satisfies
// the standard library's error interface, so ordinary "if err != nil"
// checks work on it everywhere else in the codebase; callers that want the
// extra detail use the internal/errors API to get at it (see
// TestParsePragmaRejected in parser_test.go, which checks the error
// identity with ee.Equal(...) rather than comparing message strings).
func (p *parser) syntaxError(expected string) error {
	t := p.cur()
	msg := "unexpected " + describeToken(t)

	if expected != "" {
		msg += ", expected " + expected
	}

	return errors.New(errors.ErrSQLSyntax).Context(msg).At(t.line, t.col)
}

// describeToken renders a token for a syntax error message, e.g. "'select'"
// for an identifier/keyword or "number '42'" for a numeric literal.
func describeToken(t token) string {
	switch t.kind {
	case tokEOF:
		return "end of input"
	case tokString:
		return "string literal"
	case tokBlob:
		return "blob literal"
	case tokNumber:
		return "number '" + t.text + "'"
	case tokPlaceholder:
		return "placeholder '" + t.text + "'"
	case tokIdent:
		return "'" + t.text + "'"
	default:
		return "'" + t.text + "'"
	}
}

// --- statement dispatch ---.

// parseStatement is the top of the recursive descent: it looks at the
// statement's leading keyword — without consuming it, via isKeyword — to
// decide which statement grammar applies, then hands off entirely to that
// grammar's own parse method (e.g. parseSelectStatement in select.go),
// which consumes the keyword itself as its first step. One keyword is
// always enough to tell SQL statement types apart, so no lookahead beyond
// isKeyword is needed here, unlike the ambiguous cases inside expression
// parsing (see expr.go).
func (p *parser) parseStatement() (ast.Statement, error) {
	switch {
	case p.isKeyword("with"):
		return p.parseSelectStatement()
	case p.isKeyword("select"):
		return p.parseSelectStatement()
	case p.isKeyword("insert"):
		return p.parseInsertStatement()
	case p.isKeyword("update"):
		return p.parseUpdateStatement()
	case p.isKeyword("delete"):
		return p.parseDeleteStatement()
	case p.isKeyword("create"):
		return p.parseCreateStatement()
	case p.isKeyword("drop"):
		return p.parseDropStatement()
	case p.isKeyword("alter"):
		return p.parseAlterTableStatement()
	case p.isKeyword("begin"):
		return p.parseBeginStatement()
	case p.isKeyword("commit") || p.isKeyword("end"):
		return p.parseCommitStatement()
	case p.isKeyword("rollback"):
		return p.parseRollbackStatement()
	case p.isKeyword("savepoint"):
		return p.parseSavepointStatement()
	case p.isKeyword("release"):
		return p.parseReleaseStatement()
	case p.isKeyword("pragma"):
		t := p.cur()

		return nil, errors.New(errors.ErrSQLPragmaNotSupported).At(t.line, t.col)
	default:
		return nil, p.syntaxError("a SQL statement")
	}
}

// parseSchemaQualifiedName parses "Name" or "Schema.Name", both bare or
// quoted identifiers.
func (p *parser) parseSchemaQualifiedName() (schema, name string, err error) {
	first, err := p.expectIdent()
	if err != nil {
		return "", "", err
	}

	if p.acceptPunct(".") {
		second, err := p.expectIdent()
		if err != nil {
			return "", "", err
		}

		return first, second, nil
	}

	return "", first, nil
}

// parseTableRef parses "Name" or "Schema.Name" into a spanned *ast.TableRef.
// It does not parse a trailing alias — callers that accept one (FROM-clause
// items, INSERT/UPDATE/DELETE targets) do so separately via optionalAlias.
func (p *parser) parseTableRef() (*ast.TableRef, error) {
	start := p.here()

	schema, name, err := p.parseSchemaQualifiedName()
	if err != nil {
		return nil, err
	}

	ref := &ast.TableRef{Schema: schema, Name: name}
	ref.SetSpan(start, p.here())

	return ref, nil
}

// parseQualifiedTableName parses "Name" or "Schema.Name" and joins them into
// a single "Schema.Name" string. It exists alongside parseTableRef above for
// AST fields (foreign-key REFERENCES targets — see ColumnReferences.Table
// and TableForeignKey.RefTable in ddl.go) that record a target table as a
// plain string rather than as a full *ast.TableRef, since a REFERENCES
// target never takes an alias or the other things TableRef carries.
func (p *parser) parseQualifiedTableName() (string, error) {
	schema, name, err := p.parseSchemaQualifiedName()
	if err != nil {
		return "", err
	}

	if schema != "" {
		return schema + "." + name, nil
	}

	return name, nil
}
