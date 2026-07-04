package compiler

// Tests for the "eof=" option of the @compile directive (see
// compileBlockDirective and collectTokensUntilEOFMarker in directives.go).
//
// BACKGROUND FOR A NEW READER:
//
// @compile is a directive used inside Ego test programs to compile a
// fragment of Ego source code AT RUNTIME and inspect whether it compiled
// cleanly or produced an error. This lets a test assert things like "this
// exact syntax mistake produces error code X" without stopping the whole
// test run, by wrapping the @compile statement in a try/catch:
//
//	@compile block {
//	    fmt.Println("test" "value")   // missing comma - a deliberate mistake
//	} catch(e) {
//	    // e.Code() will be "list" here
//	}
//
// The traditional form above finds the end of its code by counting "{" and
// "}" tokens: every "{" adds one, every "}" subtracts one, and the block
// ends when the count returns to where it started. That is fine as long as
// the tested code has correctly balanced braces. But it makes it very hard
// to write a test whose whole POINT is that the braces in the tested code
// are WRONG (for example: "does the compiler give a sensible error when a
// closing brace is missing?"). A stray brace inside the test throws off the
// count, and the @compile directive ends up grabbing the wrong tokens - or
// leaves tokens behind that then confuse the OUTER compiler, producing a
// bewildering unrelated error instead of testing what was intended.
//
// The "eof=" option fixes this by using a plain text marker instead of
// brace-counting to find the end of the code to compile:
//
//	@compile eof="$EOF"
//	    fmt.Println(1,,2)
//	$EOF
//	catch(e) {
//	    // e.Code() will be "token.extra" here
//	}
//
// Everything between the @compile line and a line whose tokens spell out
// "$EOF" is handed to the sub-compiler untouched - braces and all - so code
// with intentionally mismatched braces can be tested cleanly. This file
// tests both the low-level token-matching helper (collectTokensUntilEOFMarker)
// and the full @compile eof=... directive end-to-end.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// spellings is a small test helper that turns a *tokenizer.Tokenizer into a
// plain []string of each token's text, so test expectations can be written
// as ordinary string slices instead of constructing Token values by hand.
func spellings(tok *tokenizer.Tokenizer) []string {
	result := make([]string, 0, len(tok.Tokens))
	for _, t := range tok.Tokens {
		result = append(result, t.Spelling())
	}

	return result
}

// TestCollectTokensUntilEOFMarker exercises the low-level token-matching
// helper directly, without going through the full @compile directive. Each
// test case supplies a fake "source" that has already been tokenized (as if
// the compiler were positioned right after the @compile eof="..." line) and
// checks both (a) whether the marker was found, and (b) exactly which
// tokens were collected as "the code" versus discarded as "the marker".
func TestCollectTokensUntilEOFMarker(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		marker     string
		wantFound  bool
		wantTokens []string
	}{
		{
			// The simplest case: the marker is a single identifier-like
			// token, and it appears exactly once, cleanly ending the code.
			name:       "single-token marker ends the code cleanly",
			source:     `x := 1 ; END`,
			marker:     "END",
			wantFound:  true,
			wantTokens: []string{"x", ":=", "1", ";"},
		},
		{
			// The marker as given in the feature request: "$EOF" does not
			// lex as one token (the tokenizer splits "$" from "EOF"), so
			// this proves the matcher correctly glues multiple tokens'
			// spellings together before comparing against the marker.
			name:       "multi-token marker ($ and EOF) is matched by concatenated spelling",
			source:     `fmt . Println ( 1 ) ; $ EOF`,
			marker:     "$EOF",
			wantFound:  true,
			wantTokens: []string{"fmt", ".", "Println", "(", "1", ")", ";"},
		},
		{
			// This is the headline feature: code containing mismatched /
			// extra braces must be collected verbatim (braces are not
			// treated specially at all), right up to the marker.
			name:       "mismatched braces in the code are passed through untouched",
			source:     `if true { fmt . Println ( "hi" ) } } END`,
			marker:     "END",
			wantFound:  true,
			// Note: a string token's Spelling() is its unescaped content
			// with the surrounding quotes already stripped off by the
			// tokenizer, so the expected token here is "hi", not `"hi"`.
			wantTokens: []string{"if", "true", "{", "fmt", ".", "Println", "(", "hi", ")", "}", "}"},
		},
		{
			// A "false start": the code contains a token that begins the
			// same way the marker does ("E" is the first letter of "END")
			// but the match doesn't complete, so those tokens must be
			// released as ordinary code, not silently dropped.
			name:       "false start (partial match that fails) is preserved as code",
			source:     `Envelope := 1 ; END`,
			marker:     "END",
			wantFound:  true,
			// NOTE: "Envelope" itself does not partially match "END" at the
			// token-spelling level (its first token is the whole identifier
			// "Envelope", which is not a prefix of "END"), so it is ordinary
			// code from the very first comparison. This case mainly proves
			// that unrelated tokens before the real marker are unaffected.
			wantTokens: []string{"Envelope", ":=", "1", ";"},
		},
		{
			// If the source runs out before the marker text is ever
			// completed, the caller must be told the marker was not found
			// (compileBlockDirective turns this into ErrMissingEOFMarker).
			name:      "marker never appears",
			source:    `x := 1 ; y := 2`,
			marker:    "END",
			wantFound: false,
		},
		{
			// The marker may appear as the very first thing in the source,
			// meaning zero lines of code were provided. This is unusual but
			// not an error at this layer -- it just means an empty program.
			name:       "marker with no preceding code at all",
			source:     `END`,
			marker:     "END",
			wantFound:  true,
			wantTokens: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a bare compiler whose token stream is exactly the
			// pre-tokenized "source" for this test case. We don't need a
			// real @compile directive here -- collectTokensUntilEOFMarker
			// only ever looks at c.t, so this is enough to drive it
			// directly and cheaply.
			c := New(t.Name()).WithTokens(tokenizer.New(tt.source, true))

			gotTokens, gotFound := c.collectTokensUntilEOFMarker(tt.marker)

			if gotFound != tt.wantFound {
				t.Fatalf("found = %v, want %v", gotFound, tt.wantFound)
			}

			if !tt.wantFound {
				// When the marker isn't found, the caller reports an error
				// and discards the result, so its exact contents don't
				// matter -- nothing further to check.
				return
			}

			gotSpellings := spellings(gotTokens)
			if !reflect.DeepEqual(gotSpellings, tt.wantTokens) {
				t.Errorf("collected tokens = %#v, want %#v", gotSpellings, tt.wantTokens)
			}
		})
	}
}

// TestCompileEOFDirective exercises the "eof=" option through the full
// @compile directive, end to end: real Ego source containing
// "@compile eof=\"...\" ... catch(e) { ... }", compiled and run for real via
// RunString, checking the resulting 'result' variable set by the test
// program body. This is the same style used by
// TestCompileBlockDirectiveDoesNotLeakUnusedVarsSetting above and by
// tests/directives/compile.ego at the Ego-language level.
func TestCompileEOFDirective(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The exact shape from the feature request: a single-token
			// marker line, with a deliberately malformed statement (an
			// extra comma) inside the tested code. The malformed statement
			// is an array literal rather than a fmt.Println call, so this
			// test doesn't depend on the "fmt" package being resolvable --
			// that's a separate, pre-existing detail of how block-mode
			// @compile fragments resolve package names that has nothing to
			// do with the eof= feature being tested here.
			name: "eof mode catches a compile error and reports its code",
			text: `
				var code string
				@compile block eof="$EOF"
					x := []int{1,,2}
				$EOF
				catch(e) {
					code = e.Code()
				}
				result := code
			`,
			want: "token.extra",
		},
		{
			// Clean code with no compile error at all: the catch block
			// must not run, matching the existing brace-delimited
			// behavior for a successful compile.
			name: "eof mode compiles clean code with no error",
			text: `
				failed := false
				@compile block eof="$EOF"
					x := 1 + 2
					y := x * 2
					_ = y
				$EOF
				catch(e) {
					failed = true
				}
				result := failed
			`,
			want: false,
		},
		{
			// The headline scenario: code with a missing closing brace.
			// Brace-counting mode cannot cleanly express this test (the
			// missing brace throws off where the block "ends"), but eof=
			// mode isolates the code by marker text alone, so the
			// intentionally-broken code reaches the sub-compiler exactly
			// as written and fails with a sensible, catchable error.
			name: "eof mode cleanly tests code with a missing closing brace",
			text: `
				var code string
				@compile block eof="$EOF"
					if true {
						fmt.Println("unbalanced")
				$EOF
				catch(e) {
					code = e.Code()
				}
				result := code
			`,
			// "block.end" is the error key for a missing '}' - see
			// errors.ErrMissingEndOfBlock / messages_en.txt "block.end".
			want: "block.end",
		},
		{
			// Extra (too many) closing braces are just as awkward for
			// brace-counting mode as a missing one; eof= handles this the
			// same way, by ignoring braces entirely and watching only for
			// the marker text.
			name: "eof mode cleanly tests code with an extra closing brace",
			text: `
				var code string
				@compile block eof="$EOF"
					fmt.Println("a")
					}
					fmt.Println("b")
				$EOF
				catch(e) {
					code = e.Code()
				}
				result := code
			`,
			want: "statement.not.found",
		},
		{
			// Other @compile flags (unused=, block, etc.) must keep working
			// when combined with eof=. This mirrors the pre-existing
			// "unused=false" tests for brace-delimited mode.
			name: "eof mode honors unused=false like brace mode does",
			text: `
				failed := false
				@compile unused=false eof="###"
					func scratch() {
						neverReferenced := 1
					}
				###
				catch(e) {
					failed = true
				}
				result := failed
			`,
			want: false,
		},
		{
			// The converse: unused=true (the default in most test
			// environments, but set explicitly here for clarity) must
			// still flag an unused variable inside eof= mode.
			name: "eof mode honors unused=true like brace mode does",
			text: `
				var code string
				@compile unused=true eof="###"
					func scratch() {
						neverReferenced := 1
					}
				###
				catch(e) {
					code = e.Code()
				}
				result := code
			`,
			want: "var.unused",
		},
		{
			// A marker made of more than one token ("$" then "EOF") must be
			// matched by concatenated spelling, exactly like the
			// TestCollectTokensUntilEOFMarker unit test above, but now
			// exercised through the real directive end to end.
			name: "eof mode supports a multi-token marker string",
			text: `
				failed := false
				@compile block eof="$EOF"
					y := 10
					z := y + 1
					_ = z
				$EOF
				catch(e) {
					failed = true
				}
				result := failed
			`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			result, found := s.Get("result")
			if !found {
				t.Fatal("program did not set 'result'")
			}

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestCompileEOFDirectiveMissingMarkerIsAnError is a regression guard: if
// the eof= marker text never appears anywhere in the rest of the source
// (e.g. a typo in the marker, or a forgotten terminator line), the @compile
// directive itself must fail to compile with a clear error rather than
// silently consuming the rest of the file or hanging. This is NOT a
// runtime/catchable error (there is no code to even attempt to run yet), so
// it is checked directly on the return value of RunString rather than via a
// try/catch inside the Ego program.
func TestCompileEOFDirectiveMissingMarkerIsAnError(t *testing.T) {
	program := `
		@compile eof="$NOPE"
			fmt.Println("x")
		$EOF
	`

	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	err := RunString(t.Name(), s, program)
	if errors.Nil(err) {
		t.Fatal("expected a compile error because the eof= marker never appears, got none")
	}

	if !errors.Equals(err, errors.ErrMissingEOFMarker) {
		t.Errorf("expected ErrMissingEOFMarker, got %v", err)
	}
}

// TestCompileEOFDirectiveRejectsEmptyMarker is a regression guard for a
// specific edge case: eof="" (an empty string) can never be matched by any
// non-empty run of tokens in a useful way -- treating it literally would
// mean the very first token comparison ("" + firstToken == ""?) never
// succeeds either, but allowing it invites confusing test-authoring
// mistakes, so compileBlockDirective rejects it up front with a clear
// error instead of attempting to compile with a nonsensical marker.
func TestCompileEOFDirectiveRejectsEmptyMarker(t *testing.T) {
	program := `
		@compile eof=""
			fmt.Println("x")
		$EOF
	`

	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	err := RunString(t.Name(), s, program)
	if errors.Nil(err) {
		t.Fatal("expected a compile error for an empty eof= marker, got none")
	}

	if !errors.Equals(err, errors.ErrInvalidValue) {
		t.Errorf("expected ErrInvalidValue, got %v", err)
	}
}
