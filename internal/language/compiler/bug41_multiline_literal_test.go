package compiler

// Tests for the BUG-41 fix: a struct/map/array literal that spans multiple
// lines failed to compile with "invalid list" whenever an element value
// itself ended in "}" or "]" (most commonly a nested literal) and was the
// last element before the enclosing literal's closing brace/bracket, with
// no trailing comma.
//
// Root cause: the tokenizer inserts a synthetic ";" at the end of any
// source line whose last token is one that would end a Go statement -
// including "}" and "]" - following Go's own automatic semicolon insertion
// rules (see tokenizer.splitLines). The list-parsing loops for struct, map,
// and array literals (parseStruct/compileArrayInitializer in expr_atom.go,
// and parseArrayInitializer/parseMapInitializer/structInitializeByOrderedList/
// structInitializeByName/compileEmbeddedInitializer in initializer.go) only
// recognized "," or the closing bracket at that point, so the stray ";"
// produced "invalid list" instead of being silently skipped.
//
// The fix adds Compiler.skipSyntheticSemicolons(), called at both list-loop
// checkpoints (before testing for the closing bracket, and before testing
// for the "," separator) in every one of the loops named above.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestBUG41MultilineLiteral(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The exact reproducer from docs/ISSUES.md BUG-41.
			name: "untyped map literal with nested map value, no trailing comma",
			text: `
				x := {
					"a": { "b": "c" }
				}
				result := x["a"]["b"] == "c"
			`,
			want: true,
		},
		{
			// docs/LANGUAGE.md's own "@localization" example structure.
			name: "untyped map literal, three levels, no trailing commas",
			text: `
				x := {
					"en": {
						"hello": {
							"msg": "hi"
						}
					}
				}
				result := x["en"]["hello"]["msg"] == "hi"
			`,
			want: true,
		},
		{
			name: "untyped array literal, last element on its own line, no trailing comma",
			text: `
				x := []int{
					1,
					2
				}
				result := len(x) == 2 && x[0] == 1 && x[1] == 2
			`,
			want: true,
		},
		{
			name: "nested typed array literal, no trailing comma before outer close",
			text: `
				x := [][]int{
					[]int{1, 2},
					[]int{
						3,
						4
					}
				}
				result := len(x) == 2 && x[0][0] == 1 && x[1][1] == 4
			`,
			want: true,
		},
		{
			name: "typed map literal, no trailing comma before close",
			text: `
				m := map[string]int{
					"a": 1,
					"b": 2
				}
				result := m["a"] == 1 && m["b"] == 2
			`,
			want: true,
		},
		{
			name: "typed struct literal, ordered fields, no trailing comma",
			text: `
				type Point struct { X int; Y int }
				p := Point{
					1,
					2
				}
				result := p.X == 1 && p.Y == 2
			`,
			want: true,
		},
		{
			name: "typed struct literal, named fields, no trailing comma",
			text: `
				type Point struct { X int; Y int }
				p := Point{
					X: 1,
					Y: 2
				}
				result := p.X == 1 && p.Y == 2
			`,
			want: true,
		},
		{
			// Nested struct-typed field value ends the line with "}"; this is
			// the typed-struct analogue of the original bug report and
			// exercises structInitializeByName's embedded compileInitializer
			// recursion.
			name: "typed struct literal with nested struct field, no trailing comma",
			text: `
				type Point struct { X int; Y int }
				type Line struct { A Point; B Point }
				l := Line{
					A: Point{
						5,
						6
					},
					B: Point{7, 8}
				}
				result := l.A.X == 5 && l.A.Y == 6 && l.B.X == 7 && l.B.Y == 8
			`,
			want: true,
		},
		{
			// Regression guard: a literal that already had trailing commas
			// (the documented Go-style workaround) must keep working.
			name: "literal with trailing commas still parses (unaffected by fix)",
			text: `
				x := []int{
					1,
					2,
				}
				result := len(x) == 2 && x[0] == 1 && x[1] == 2
			`,
			want: true,
		},
		{
			// Regression guard: single-line literals are unaffected.
			name: "single-line nested literal still parses (unaffected by fix)",
			text: `
				x := { "a": { "b": "c" } }
				result := x["a"]["b"] == "c"
			`,
			want: true,
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

// TestBUG41GenuineMissingCommaStillErrors is a regression guard: the fix must
// only skip synthetic ";" tokens the tokenizer inserted at a line break. A
// genuinely malformed literal missing its comma on a single line (so no
// synthetic ";" is involved) must still be rejected.
func TestBUG41GenuineMissingCommaStillErrors(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "array literal missing comma on one line",
			text: `x := []int{1 2}`,
		},
		{
			name: "map literal missing comma on one line",
			text: `x := { "a": 1 "b": 2 }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			err := RunString(tt.name, s, tt.text)
			if errors.Nil(err) {
				t.Fatalf("expected a compile error for malformed literal %q, got none", tt.text)
			}

			if !errors.Equals(err, errors.ErrInvalidList) {
				t.Errorf("expected ErrInvalidList, got %v", err)
			}
		})
	}
}
