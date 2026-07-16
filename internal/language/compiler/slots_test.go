package compiler

// Unit tests for the slot-eligibility predicate (docs/SLOTS.md Section 5.1,
// Section 11 Q2). These mirror the style of the Finding 4/8/11 predicate tests
// (needsOwnScope, etc.): each case feeds a complete "{...}" function body to
// functionBodyIsSlotEligible with the tokenizer positioned at the body's
// opening brace, and asserts the verdict.

import (
	"testing"

	"github.com/tucats/ego/internal/language/tokenizer"
)

// slotEligible positions a fresh Compiler's tokenizer at the opening "{" of
// body and returns the predicate's verdict. seed lists the function's
// parameter / named-return / receiver names (the declarations that live
// outside the body braces).
func slotEligible(t *testing.T, seed []string, body string) bool {
	t.Helper()

	c := &Compiler{}
	c.t = tokenizer.New(body, true)

	seedNames := map[string]bool{}
	for _, n := range seed {
		seedNames[n] = true
	}

	// The predicate expects the mark to sit AT the body's opening brace, so we
	// do NOT consume it here.
	return c.functionBodyIsRegisterEligible(seedNames)
}

func TestFunctionBodyIsSlotEligible(t *testing.T) {
	tests := []struct {
		name string
		seed []string
		body string
		want bool
	}{
		{
			name: "empty body is eligible",
			body: `{}`,
			want: true,
		},
		{
			name: "plain locals and assignments, no literals",
			body: `{ x := 1; y := x + 2; x = y * 2 }`,
			want: true,
		},
		{
			name: "uses only params and globals",
			seed: []string{"a", "b"},
			body: `{ return a + b + fmt.Len(a) }`,
			want: true,
		},
		{
			name: "nested if/for blocks with their own locals are fine",
			body: `{ for i := 0; i < 10; i++ { if i > 5 { z := i * 2; total = total + z } } }`,
			want: true,
		},
		{
			name: "go statement disqualifies",
			body: `{ x := 1; go worker(x) }`,
			want: false,
		},
		{
			name: "defer statement disqualifies",
			body: `{ f := open(); defer f.Close() }`,
			want: false,
		},
		{
			name: "closure capturing an enclosing local disqualifies",
			body: `{ n := 5; g := func() int { return n + 1 }; return g() }`,
			want: false,
		},
		{
			name: "closure capturing a parameter disqualifies",
			seed: []string{"limit"},
			body: `{ g := func() int { return limit }; return g() }`,
			want: false,
		},
		{
			name: "closure capturing a var-declared local disqualifies",
			body: `{ var count int; count = 3; h := func() int { return count }; return h() }`,
			want: false,
		},
		{
			name: "closure using only its own params does not disqualify",
			body: `{ sort.Slice(data, func(i, j int) bool { return i < j }) }`,
			want: true,
		},
		{
			name: "closure referencing only globals/packages does not disqualify",
			body: `{ run(func() int { return fmt.Answer })  }`,
			want: true,
		},
		{
			name: "nested closure capturing enclosing name disqualifies",
			body: `{ n := 1; outer(func() { inner(func() int { return n }) }) }`,
			want: false,
		},
		{
			name: "defer inside a non-capturing closure does not disqualify enclosing",
			body: `{ f := func() { defer cleanup() }; register(f) }`,
			want: true,
		},
		{
			name: "go inside a non-capturing closure does not disqualify enclosing",
			body: `{ register(func() { go background() }) }`,
			want: true,
		},
		{
			name: "closure with inline struct in signature bails to ineligible",
			body: `{ apply(func() struct { X int } { return z }) }`,
			want: false,
		},
		{
			name: "empty-body closure captures nothing",
			seed: []string{"p"},
			body: `{ register(func() {}) }`,
			want: true,
		},
		{
			name: "not a block is ineligible",
			body: `x := 1`,
			want: false,
		},
		{
			name: "discard name is not treated as a captured local",
			body: `{ _ = 5; run(func() { _ = 9 }) }`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slotEligible(t, tt.seed, tt.body)
			if got != tt.want {
				t.Errorf("functionBodyIsSlotEligible() = %v, want %v (body: %s)", got, tt.want, tt.body)
			}
		})
	}
}
