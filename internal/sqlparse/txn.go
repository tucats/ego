package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file implements the transaction-control statements: BEGIN, COMMIT
// (and its "END" alias), ROLLBACK, SAVEPOINT, and RELEASE. These are the
// simplest statements in the grammar — no expressions, no nested clauses —
// so this file is a good place to see the package's basic parsing idioms in
// isolation if the bigger files (select.go, ddl.go) feel like a lot at
// once: acceptKeyword for optional pieces, expectIdent for mandatory names,
// and BaseNode's SetSpan(start, p.here()) as the last step of every
// function, recording the source span from where parsing began (start,
// captured before anything is consumed) to the cursor's position now that
// everything has been consumed.
//
// Several functions below use a tagless switch purely to try a short list
// of optional, mutually exclusive keywords and keep whichever one matched —
// or none, silently, if none did. Unlike the ASC/DESC switch in
// common.go's parseOrderByTerm (which records *which* case matched, since
// ASC and DESC mean different things), the "TRANSACTION"/"WORK" switches
// below have empty case bodies: TRANSACTION and WORK are pure synonyms
// SQL allows for readability, and neither is retained on the AST node,
// so the switch's only job is to consume whichever one is present (if
// either) and otherwise do nothing.

// parseBeginStatement parses "BEGIN [DEFERRED|IMMEDIATE|EXCLUSIVE]
// [TRANSACTION|WORK] [name]".
func (p *parser) parseBeginStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "begin"

	stmt := &ast.BeginStmt{}

	switch {
	case p.acceptKeyword("deferred"):
		stmt.Mode = "DEFERRED"
	case p.acceptKeyword("immediate"):
		stmt.Mode = "IMMEDIATE"
	case p.acceptKeyword("exclusive"):
		stmt.Mode = "EXCLUSIVE"
	}

	switch {
	case p.acceptKeyword("transaction"):
	case p.acceptKeyword("work"):
	}

	// The trailing transaction name is optional and has no keyword of its
	// own to signal its presence — it's just a bare identifier, if there is
	// one. clauseStopKeywords (from common.go, normally used for alias
	// detection) is reused here for the same underlying reason: at
	// statement end, only a semicolon or end-of-input can legally follow,
	// so any of those stop keywords appearing here would mean something
	// went wrong elsewhere — but checking defensively costs nothing and
	// avoids ever misreading, say, a stray keyword as a transaction name.
	if p.cur().kind == tokIdent && !clauseStopKeywords[toLower(p.cur().text)] {
		stmt.Name = p.next().text
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseCommitStatement parses "COMMIT [TRANSACTION|WORK]" — also reached
// for "END [TRANSACTION|WORK]", PostgreSQL's synonym for COMMIT (see the
// "commit" || "end" case in parser.parseStatement, parser.go).
func (p *parser) parseCommitStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "commit" or "end"

	switch {
	case p.acceptKeyword("transaction"):
	case p.acceptKeyword("work"):
	}

	stmt := &ast.CommitStmt{}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseRollbackStatement parses "ROLLBACK [TRANSACTION|WORK]
// [TO [SAVEPOINT] name]".
func (p *parser) parseRollbackStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "rollback"

	switch {
	case p.acceptKeyword("transaction"):
	case p.acceptKeyword("work"):
	}

	stmt := &ast.RollbackStmt{}

	if p.acceptKeyword("to") {
		p.acceptKeyword("savepoint")

		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		stmt.To = name
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseSavepointStatement parses "SAVEPOINT name".
func (p *parser) parseSavepointStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "savepoint"

	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	stmt := &ast.SavepointStmt{Name: name}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseReleaseStatement parses "RELEASE [SAVEPOINT] name".
func (p *parser) parseReleaseStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "release"

	p.acceptKeyword("savepoint")

	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	stmt := &ast.ReleaseStmt{Name: name}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}
