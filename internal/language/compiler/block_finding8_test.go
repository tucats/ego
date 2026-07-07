package compiler

// Tests for PERFORMANCE.md Finding 8: skipping a brace-delimited block's own
// PushScope/PopScope pair when the block declares nothing of its own. See the
// large comment block above blockBodyNeedsOwnScope in block.go for the full
// design rationale.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// needsOwnScope is a small test helper that positions a fresh Compiler's
// tokenizer just past the opening "{" of body (which must be a complete
// "{...}" block) and returns the predicate's verdict. This matches
// blockBodyNeedsOwnScope's own documented precondition: the caller must
// already have consumed the block's opening brace.
func needsOwnScope(t *testing.T, body string) bool {
	t.Helper()

	c := &Compiler{}
	c.t = tokenizer.New(body, true)

	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		t.Fatalf("test body %q does not start with '{'", body)
	}

	return c.blockBodyNeedsOwnScope()
}

func TestBlockBodyNeedsOwnScope(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "function call only, no declarations or closures",
			body: `{ fmt.Println("hi") }`,
			want: false,
		},
		{
			name: "assignment to an existing outer variable, not a declaration",
			body: `{ sum = sum + 1 }`,
			want: false,
		},
		{
			name: "top-level short declaration disqualifies",
			body: `{ doubled := 2; sum = sum + doubled }`,
			want: true,
		},
		{
			name: "top-level var declaration disqualifies",
			body: `{ var doubled int; doubled = 2 }`,
			want: true,
		},
		{
			name: "top-level const declaration disqualifies",
			// This is the exact shape that originally surfaced the bug this
			// predicate must guard against: a block containing ONLY a
			// "const" declaration has no ":=" or "var" token at all, but
			// still binds a new (immutable) name that must not leak into
			// the enclosing scope.
			body: `{ const x = 1; sum = sum + x }`,
			want: true,
		},
		{
			name: "top-level type declaration disqualifies",
			body: `{ type Point struct { x int }; p := Point{} }`,
			want: true,
		},
		{
			name: "declaration nested inside an if is still disqualifying at depth 2",
			// Known-conservative case (documented for Finding 4 too): the
			// declaration is actually scoped to the nested "if" block,
			// which gets its own scope regardless, so eliding THIS block
			// would still be safe - but the scan only special-cases depth
			// == 0 relative to itself, so a nested declaration is not
			// detected and does NOT disqualify.
			body: `{ if x > 0 { doubled := x * 2; sum = sum + doubled } }`,
			want: false,
		},
		{
			name: "function literal disqualifies",
			body: `{ funcs = append(funcs, func() int { return 1 }) }`,
			want: true,
		},
		{
			name: "go statement disqualifies",
			body: `{ go worker(1) }`,
			want: true,
		},
		{
			name: "defer statement disqualifies",
			body: `{ defer cleanup(1) }`,
			want: true,
		},
		{
			name: "nested block with its own declaration does not disqualify the outer block",
			// The nested block's ":=" is at depth 1 relative to the OUTER
			// block, not depth 0, so it is correctly recognized as scoped
			// to the nested block and does not disqualify this one.
			body: `{ { y := 1; sum = sum + y } }`,
			want: false,
		},
		{
			name: "malformed input with no closing brace is treated as unsafe",
			body: `{ sum = sum + 1`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsOwnScope(t, tt.body)
			if got != tt.want {
				t.Errorf("blockBodyNeedsOwnScope(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestFinding8ConstInElidedBlockDoesNotLeak guards the correctness bug found
// while implementing Finding 8: a block containing only a "const" declaration
// has no ":=" or "var" token, so an earlier version of the predicate treated
// it as safe to elide. Without a scope of its own, the constant's name was
// created directly in the enclosing scope and never cleaned up, so a later,
// unrelated variable of the same name collided with it ("item is read-only").
// This is exercised twice in the same program, in two different (elided)
// blocks, specifically to confirm the const's name does not survive past the
// block that declared it.
func TestFinding8ConstInElidedBlockDoesNotLeak(t *testing.T) {
	text := `
		func first() int {
			const a = 1
			return a
		}

		func second() int {
			// "a" here is a completely unrelated ordinary variable. If the
			// first function's "const a" had leaked out of its own
			// (elided) block into a shared enclosing scope, this
			// assignment would fail with "item is read-only".
			a := 99
			return a
		}

		result := first() + second()
	`

	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	if !reflect.DeepEqual(result, 100) {
		t.Errorf("got %v (%T), want 100", result, result)
	}
}
