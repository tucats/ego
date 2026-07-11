package fmt

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

func Test_scanner(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		format string
		want   []any
		err    error
	}{
		// --- integer verbs ---
		{
			name:   "Single integer",
			data:   "35",
			format: "%d",
			want:   []any{35},
			err:    nil,
		},
		{
			name:   "Integer with leading spaces",
			data:   "   42",
			format: "%d",
			want:   []any{42},
			err:    nil,
		},
		{
			name:   "Integer width limits characters consumed",
			data:   "12345",
			format: "%3d",
			want:   []any{123},
			err:    nil,
		},
		{
			name:   "Two integers",
			data:   "10 20",
			format: "%d %d",
			want:   []any{10, 20},
			err:    nil,
		},
		{
			name:   "Invalid integer",
			data:   "thirty",
			format: "%d",
			want:   []any{},
			err:    errors.ErrScanExpectedInteger,
		},
		{
			name:   "Negative integer",
			data:   "-35",
			format: "%d",
			want:   []any{-35},
			err:    nil,
		},
		{
			name:   "Bare minus sign with no digits is an error",
			data:   "- 5",
			format: "%d",
			want:   []any{},
			err:    errors.ErrScanExpectedInteger,
		},

		// --- hexadecimal verb ---
		{
			name:   "hexadecimal int",
			data:   "DEADBEEF",
			format: "%x",
			want:   []any{3735928559},
			err:    nil,
		},
		{
			name:   "two hexadecimal ints with widths",
			data:   "DEAD BEEF",
			format: "%4x %4x",
			want:   []any{57005, 48879},
			err:    nil,
		},
		{
			name:   "lowercase hexadecimal",
			data:   "ff",
			format: "%x",
			want:   []any{255},
			err:    nil,
		},

		// --- binary verb ---
		{
			name:   "binary int",
			data:   "1101",
			format: "%b",
			want:   []any{13},
			err:    nil,
		},
		{
			name:   "binary int with width",
			data:   "11010011",
			format: "%4b",
			want:   []any{13},
			err:    nil,
		},

		// --- octal verb ---
		{
			name:   "octal integer",
			data:   "17",
			format: "%o",
			want:   []any{15},
			err:    nil,
		},
		{
			name:   "octal integer with width",
			data:   "0755",
			format: "%4o",
			want:   []any{493},
			err:    nil,
		},
		{
			name:   "invalid octal digit stops scan",
			data:   "89",
			format: "%o",
			want:   []any{},
			err:    errors.ErrScanExpectedInteger,
		},

		// --- float verb ---
		{
			name:   "Single float64",
			data:   "3.14",
			format: "%f",
			want:   []any{3.14},
			err:    nil,
		},
		{
			name:   "Integer-looking float",
			data:   "42",
			format: "%f",
			want:   []any{42.0},
			err:    nil,
		},
		{
			name:   "Negative float",
			data:   "-3.14",
			format: "%f",
			want:   []any{-3.14},
			err:    nil,
		},
		{
			name:   "Negative float with width",
			data:   "-3.14extra",
			format: "%5f",
			want:   []any{-3.14},
			err:    nil,
		},
		{
			name:   "Float width clamped to available data",
			data:   "3.1",
			format: "%8f",
			want:   []any{3.1},
			err:    nil,
		},
		{
			name:   "Bare minus sign with no digits is an error",
			data:   "-",
			format: "%f",
			want:   []any{},
			err:    errors.ErrInvalidValue,
		},
		{
			name:   "Invalid float",
			data:   "abc",
			format: "%f",
			want:   []any{},
			err:    errors.ErrInvalidValue,
		},

		// --- string verb ---
		{
			name:   "Single string",
			data:   "Tom",
			format: "%s",
			want:   []any{"Tom"},
			err:    nil,
		},
		{
			name:   "String skips leading spaces",
			data:   "   Tom",
			format: "%s",
			want:   []any{"Tom"},
			err:    nil,
		},
		{
			name:   "String stops at whitespace",
			data:   "Hello World",
			format: "%s",
			want:   []any{"Hello"},
			err:    nil,
		},
		{
			name:   "Two strings",
			data:   "Hello World",
			format: "%s %s",
			want:   []any{"Hello", "World"},
			err:    nil,
		},
		{
			name:   "String with width",
			data:   " 12 34 567 ",
			format: "%5s%d",
			want:   []any{"12 34", 567},
			err:    nil,
		},

		// --- boolean verb ---
		{
			name:   "boolean true",
			data:   "so true",
			format: "so %t",
			want:   []any{true},
			err:    nil,
		},
		{
			name:   "boolean false",
			data:   "result false",
			format: "result %t",
			want:   []any{false},
			err:    nil,
		},
		{
			name:   "standalone boolean true",
			data:   "true",
			format: "%t",
			want:   []any{true},
			err:    nil,
		},
		{
			name:   "invalid boolean",
			data:   "maybe",
			format: "%t",
			want:   []any{},
			err:    errors.ErrInvalidBooleanValue,
		},

		// --- literal matching ---
		{
			name:   "Const and string",
			data:   "Name Tom",
			format: "Name %s",
			want:   []any{"Tom"},
			err:    nil,
		},
		{
			name:   "Const, string, int",
			data:   "name Tom age 44",
			format: "name %s age %d",
			want:   []any{"Tom", 44},
			err:    nil,
		},
		{
			name:   "Const mismatch stops scan with a mismatch error",
			data:   "Hello Tom",
			format: "Name %s",
			want:   []any{},
			err:    errors.ErrScanMismatch,
		},

		// --- invalid format verbs ---
		{
			name:   "Unknown format verb",
			data:   "hello",
			format: "%q",
			want:   []any{},
			err:    errors.ErrInvalidFormatVerb,
		},
		{
			name:   "Truncated format (bare % at end)",
			data:   "42",
			format: "%",
			want:   []any{},
			err:    errors.ErrInvalidFormatVerb,
		},

		// --- data exhausted before format ends (BUG-52) ---
		{
			name:   "Data exhausted before second verb",
			data:   "42",
			format: "%d %d",
			want:   []any{42},
			err:    errors.ErrScanEOF,
		},
		{
			name:   "Empty data with integer format",
			data:   "",
			format: "%d",
			want:   []any{},
			err:    errors.ErrScanEOF,
		},
		{
			// BUG-52 primary reproducer: fmt.Sscanf("5", "%d %d", &c, &d)
			// must report a non-nil error (real Go: io.EOF) instead of
			// silently returning nil.
			name:   "BUG-52: second verb runs out of data entirely",
			data:   "5",
			format: "%d %d",
			want:   []any{5},
			err:    errors.ErrScanEOF,
		},
		{
			// BUG-52 secondary reproducer: fmt.Sscanf("wrong 35", "age %d", &a)
			// must report a non-nil error (real Go: "input does not match
			// format") instead of silently returning nil.
			name:   "BUG-52: literal text mismatch",
			data:   "wrong 35",
			format: "age %d",
			want:   []any{},
			err:    errors.ErrScanMismatch,
		},
		{
			name:   "BUG-52: data runs out mid-literal is treated as EOF",
			data:   "wro",
			format: "wrong %d",
			want:   []any{},
			err:    errors.ErrScanEOF,
		},
		{
			name:   "BUG-52: genuine integer parse error propagates",
			data:   "5 abc",
			format: "%d %d",
			want:   []any{5},
			err:    errors.ErrScanExpectedInteger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, e := scanner(tt.data, tt.format)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scanner() got = %v, want %v", got, tt.want)
			}

			if !errors.Equals(e, tt.err) {
				t.Errorf("scanner() got1 = %v, want %v", e, tt.err)
			}
		})
	}
}

// Test_stringScanFormat_BUG52 exercises fmt.Sscanf's wrapper (stringScanFormat)
// end-to-end with the two BUG-52 reproducers, verifying that the returned
// count and error match Go's partial-scan semantics: values scanned before
// the failure are still assigned to their pointers, and the error return is
// non-nil rather than silently swallowed.
func Test_stringScanFormat_BUG52(t *testing.T) {
	st := symbols.NewSymbolTable("test")

	t.Run("second verb runs out of data entirely", func(t *testing.T) {
		var c, d any = 0, 0

		result, err := stringScanFormat(st, data.NewList("5", "%d %d", &c, &d))
		if err == nil {
			t.Fatal("expected a non-nil error")
		}

		list, ok := result.(data.List)
		if !ok {
			t.Fatalf("expected result to be a data.List, got %T", result)
		}

		if n, _ := list.GetInt(0); n != 1 {
			t.Errorf("expected count 1, got %v", list.Get(0))
		}

		if !errors.Equals(err, errors.ErrScanEOF) {
			t.Errorf("expected ErrScanEOF, got %v", err)
		}

		if c != 5 {
			t.Errorf("expected c == 5, got %v", c)
		}

		if d != 0 {
			t.Errorf("expected d to be left untouched at 0, got %v", d)
		}
	})

	t.Run("literal text mismatch", func(t *testing.T) {
		var a any = 0

		result, err := stringScanFormat(st, data.NewList("wrong 35", "age %d", &a))
		if err == nil {
			t.Fatal("expected a non-nil error")
		}

		list, ok := result.(data.List)
		if !ok {
			t.Fatalf("expected result to be a data.List, got %T", result)
		}

		if n, _ := list.GetInt(0); n != 0 {
			t.Errorf("expected count 0, got %v", list.Get(0))
		}

		if !errors.Equals(err, errors.ErrScanMismatch) {
			t.Errorf("expected ErrScanMismatch, got %v", err)
		}

		if a != 0 {
			t.Errorf("expected a to be left untouched at 0, got %v", a)
		}
	})
}
