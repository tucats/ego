package util

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoWriteHeaderBeforeWriteJSON enforces a rule that cannot be expressed in the Go
// type system but that silently corrupts responses when broken.
//
// util.WriteJSON now issues the response status itself, because it may first need to
// add a "Content-Encoding: gzip" header. WriteHeader() is the moment the status line
// and every header are flushed to the client; any header set after that call is
// discarded without complaint. So if a handler calls w.WriteHeader() and then calls
// WriteJSON, the compressed bytes still go out but the header telling the client they
// are compressed does not -- producing a response no client can parse, with nothing in
// any log to explain why. The failure appears only for payloads large enough to cross
// the compression threshold, which makes it exactly the kind of bug that escapes
// casual testing and shows up on real data.
//
// Eight such call sites existed before compression was added, and all were corrected.
// This test exists so that a ninth cannot be introduced unnoticed.
//
// Note on false positives: a WriteHeader on a mutually exclusive early-return error
// path is harmless, since the two calls can never both run. The check below therefore
// ignores any WriteHeader that is followed by a return statement inside the same
// block, which is the shape those error paths always take in this codebase.
func TestNoWriteHeaderBeforeWriteJSON(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("cannot locate the repository root: %v", err)
	}

	fileSet := token.NewFileSet()
	violations := 0

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			// Skip directories that hold no server code, so the scan stays quick.
			if name := entry.Name(); name == ".git" || name == "builds" || name == "tests" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}

			for _, offender := range findEarlyWriteHeader(fn) {
				violations++

				t.Errorf("%s: w.WriteHeader() at line %d precedes a util.WriteJSON() call "+
					"in the same function; remove it and let WriteJSON send the status, "+
					"or the Content-Encoding header will be silently dropped",
					filepath.Base(path), fileSet.Position(offender).Line)
			}

			return true
		})

		return nil
	})

	if walkErr != nil {
		t.Fatalf("failed to scan the source tree: %v", walkErr)
	}

	if violations == 0 {
		t.Log("no handler sends its status before calling WriteJSON")
	}
}

// findEarlyWriteHeader returns the positions of any WriteHeader calls in a function
// that come before a util.WriteJSON call and are not part of an early-return path.
func findEarlyWriteHeader(fn *ast.FuncDecl) []token.Pos {
	var (
		headers  []token.Pos
		writeAt  = token.NoPos
		offences []token.Pos
	)

	// Collect the position of every WriteHeader call, and the position of the first
	// WriteJSON call. Only their relative order in the source matters here.
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		switch selector.Sel.Name {
		case "WriteHeader":
			headers = append(headers, call.Pos())

		case "WriteJSON":
			// Confirm this is util.WriteJSON rather than some other WriteJSON.
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "util" && writeAt == token.NoPos {
				writeAt = call.Pos()
			}
		}

		return true
	})

	if writeAt == token.NoPos {
		return nil
	}

	for _, header := range headers {
		if header < writeAt && !isEarlyReturnPath(fn.Body, header) {
			offences = append(offences, header)
		}
	}

	return offences
}

// isEarlyReturnPath reports whether a WriteHeader call at the given position is part of
// an error path that returns immediately, and so can never be followed by a WriteJSON
// on the same execution path.
//
// The rule is deliberately narrow: the very next statement in the WriteHeader's own
// innermost block must be a return. That is the shape every error path in this codebase
// takes:
//
//	if err != nil {
//	    w.WriteHeader(http.StatusInternalServerError)
//	    return status
//	}
//
// An earlier, looser version of this check asked only whether *some* later statement in
// the block was a return. Because nearly every handler ends in a return, that suppressed
// essentially every finding and the test passed against deliberately broken code. When
// in doubt this version reports a violation, which is the safe direction to be wrong in:
// a false positive is a moment's inspection, a false negative is a corrupt response.
func isEarlyReturnPath(body *ast.BlockStmt, pos token.Pos) bool {
	innermost := innermostBlock(body, pos)
	if innermost == nil {
		return false
	}

	for i, statement := range innermost.List {
		if statement.Pos() <= pos && pos <= statement.End() {
			if i+1 < len(innermost.List) {
				_, isReturn := innermost.List[i+1].(*ast.ReturnStmt)

				return isReturn
			}

			// The WriteHeader is the last statement in its block. That only exits the
			// function if the block is the function body itself, which the caller has
			// already established is not the case (a WriteJSON follows it).
			return false
		}
	}

	return false
}

// innermostBlock finds the most deeply nested block statement that contains a position.
// Nesting depth matters: a WriteHeader inside an "if" belongs to that if's block, not to
// the enclosing function body, and it is the inner block whose next statement decides
// whether this is an early-return path.
func innermostBlock(body *ast.BlockStmt, pos token.Pos) *ast.BlockStmt {
	var best *ast.BlockStmt

	ast.Inspect(body, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}

		if block.Pos() <= pos && pos <= block.End() {
			// A block that contains the position and sits inside the best candidate
			// found so far is a closer fit.
			if best == nil || block.Pos() > best.Pos() {
				best = block
			}
		}

		return true
	})

	return best
}
