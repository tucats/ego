package fmt

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
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
			err:    errors.ErrInvalidValue,
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
			err:    errors.ErrInvalidValue,
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
			err:    errors.ErrInvalidValue,
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
			name:   "Const mismatch stops scan without error",
			data:   "Hello Tom",
			format: "Name %s",
			want:   []any{},
			err:    nil,
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

		// --- data exhausted before format ends ---
		{
			name:   "Data exhausted before second verb",
			data:   "42",
			format: "%d %d",
			want:   []any{42},
			err:    nil,
		},
		{
			name:   "Empty data with integer format",
			data:   "",
			format: "%d",
			want:   []any{},
			err:    nil,
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
