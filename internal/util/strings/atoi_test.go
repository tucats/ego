package egostrings_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/tucats/ego/internal/errors"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// TestAtoi_Decimal verifies plain decimal parsing, which is delegated
// straight through to strconv.Atoi. This covers positive, negative, zero,
// and whitespace-padded values, plus the documented int(-3.9)-style
// zero-value-on-error contract.
func TestAtoi_Decimal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "simple positive", input: "42", want: 42},
		{name: "simple negative", input: "-7", want: -7},
		{name: "zero", input: "0", want: 0},
		{name: "leading whitespace", input: "   42", want: 42},
		{name: "trailing whitespace", input: "42   ", want: 42},
		{name: "surrounding whitespace", input: "  -15  ", want: -15},
		{name: "explicit plus sign", input: "+8", want: 8},
		{name: "empty string is an error", input: "", want: 0, wantErr: true},
		{name: "whitespace only is an error", input: "   ", want: 0, wantErr: true},
		{name: "non-numeric garbage", input: "abc", want: 0, wantErr: true},
		{name: "decimal point is not an integer", input: "3.14", want: 0, wantErr: true},
		{name: "trailing garbage after digits", input: "42abc", want: 0, wantErr: true},
		{name: "internal whitespace is invalid", input: "4 2", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_Hexadecimal exercises the "0x"/"0X" radix prefix path. The
// prefix check is case-insensitive, but the digits themselves must still
// be valid base-16 digits regardless of case.
func TestAtoi_Hexadecimal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "lowercase prefix, uppercase digits", input: "0x1F", want: 31},
		{name: "uppercase prefix, uppercase digits", input: "0X1F", want: 31},
		{name: "lowercase prefix, lowercase digits", input: "0xff", want: 255},
		{name: "uppercase prefix, lowercase digits", input: "0XFF", want: 255},
		{name: "zero value", input: "0x0", want: 0},
		{name: "whitespace padded", input: "  0x1F  ", want: 31},
		// strconv.ParseInt(base 16) itself accepts a sign after the value
		// portion is isolated from the "0x" prefix by Atoi, so a negative
		// hex literal is parsed as a negative magnitude.
		{name: "negative hex value", input: "0x-1F", want: -31},
		{name: "invalid hex digit", input: "0xZZ", want: 0, wantErr: true},
		{name: "empty digits after prefix", input: "0x", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_Octal exercises the "0o"/"0O" radix prefix path (base 8).
func TestAtoi_Octal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "lowercase prefix", input: "0o17", want: 15},
		{name: "uppercase prefix", input: "0O17", want: 15},
		{name: "zero value", input: "0o0", want: 0},
		{name: "whitespace padded", input: "  0o17  ", want: 15},
		{name: "max single octal digit", input: "0o7", want: 7},
		{name: "invalid octal digit (8 is not valid in base 8)", input: "0o8", want: 0, wantErr: true},
		{name: "empty digits after prefix", input: "0o", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_Binary exercises the "0b"/"0B" radix prefix path (base 2).
func TestAtoi_Binary(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "lowercase prefix", input: "0b1010", want: 10},
		{name: "uppercase prefix", input: "0B1010", want: 10},
		{name: "zero value", input: "0b0", want: 0},
		{name: "all ones nibble", input: "0b1111", want: 15},
		{name: "whitespace padded", input: "  0b1010  ", want: 10},
		{name: "invalid binary digit", input: "0b1012", want: 0, wantErr: true},
		{name: "empty digits after prefix", input: "0b", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_RuneLiteral verifies the special-cased "'x'" rune-literal
// syntax, including the documented ordering requirement (rune-literal
// detection happens BEFORE the whole string is lowercased), multi-byte
// UTF-8 runes, and the ErrInvalidRune failure mode when the quoted
// content is not exactly one rune.
func TestAtoi_RuneLiteral(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "uppercase letter", input: "'A'", want: 65},
		// Confirms the doc comment claim that the rest of the string is
		// NOT lowercased, so 'A' stays 65 rather than becoming 'a' (97).
		{name: "case is preserved, not folded to lowercase", input: "'A'", want: 65},
		{name: "lowercase letter", input: "'a'", want: 97},
		{name: "digit character", input: "'5'", want: 53},
		{name: "space character", input: "' '", want: 32},
		// Multi-byte UTF-8 rune: the euro sign is U+20AC.
		{name: "multi-byte UTF-8 rune", input: "'€'", want: 0x20AC},
		{name: "whitespace padded", input: "  'A'  ", want: 65},
		{name: "empty quotes is invalid rune", input: "''", want: 0, wantErr: true},
		{name: "multiple characters is invalid rune", input: "'AB'", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}

				if !errors.Equals(err, errors.ErrInvalidRune) {
					t.Errorf("Atoi(%q) error = %v, want an ErrInvalidRune", tt.input, err)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_SingleQuoteEdgeCases checks strings that involve a single quote
// character but do not qualify as a two-or-more-character rune literal, to
// make sure the "len(s) > 1" guard in the rune-detection branch is exercised
// correctly and such inputs fall through to (and fail) ordinary decimal
// parsing rather than panicking on a slice-index error.
func TestAtoi_SingleQuoteEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "single quote character alone", input: "'"},
		{name: "single quote is only whitespace-trimmed content", input: "  '  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)
			if err == nil {
				t.Fatalf("Atoi(%q) expected an error, got value %d", tt.input, got)
			}

			if got != 0 {
				t.Errorf("Atoi(%q) = %d, want 0 on error", tt.input, got)
			}
		})
	}
}

// TestAtoi_ShortStringRadixDetection targets the min(len(s), 2) guard used
// by the radix-prefix checks. Strings shorter than 2 characters must not
// panic when sliced for prefix comparison, and single-character strings
// that happen to match part of a prefix must still be treated as plain
// decimal input.
func TestAtoi_ShortStringRadixDetection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "single digit zero", input: "0", want: 0},
		{name: "single digit nonzero", input: "5", want: 5},
		{name: "single character letter is invalid decimal", input: "x", want: 0, wantErr: true},
		{name: "two-char decimal that is not a radix prefix", input: "42", want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Atoi(%q) expected an error, got none", tt.input)
				}
			} else if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_LossOfPrecision verifies the overflow guard that rejects parsed
// int64 values which fall outside the range of the platform's native int
// type. On 64-bit platforms (where int is 64 bits), math.MinInt/MaxInt equal
// the int64 bounds, so this exercises the boundary condition using values
// derived directly from math.MinInt64/MaxInt64 via hexadecimal literals,
// which are guaranteed to be representable as int64 regardless of platform
// int width.
func TestAtoi_LossOfPrecision(t *testing.T) {
	// On a 64-bit platform int and int64 share the same range, so the
	// overflow branch in Atoi is unreachable via ParseInt(..., 64) since
	// ParseInt itself would already reject anything wider than int64.
	// We still confirm the boundary values at the edge of int64 succeed,
	// and document the intended failure mode for 32-bit platforms.
	if strconv.IntSize < 64 {
		t.Skip("loss-of-precision boundary test assumes a 64-bit platform int")
	}

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "max int64 hex value fits", input: "0x7FFFFFFFFFFFFFFF", want: math.MaxInt64},
		{name: "min int64 hex value (negative) fits", input: "0x-8000000000000000", want: math.MinInt64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := egostrings.Atoi(tt.input)
			if err != nil {
				t.Fatalf("Atoi(%q) unexpected error: %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAtoi_LossOfPrecisionError confirms that when strconv.ParseInt would
// overflow int64 itself (e.g. a hex literal wider than 64 bits), Atoi
// reports a parse error (from strconv) rather than the ErrLossOfPrecision
// path — since ParseInt's own range check triggers first for values that
// cannot even be represented in int64. This documents the real failure mode
// distinct from the dedicated ErrLossOfPrecision branch in the source,
// which can only be reached on platforms where int is narrower than int64.
func TestAtoi_LossOfPrecisionError(t *testing.T) {
	// A 17-hex-digit value exceeds 64 bits and cannot be parsed by
	// strconv.ParseInt(..., 16, 64) at all, so this returns strconv's own
	// range error, not errors.ErrLossOfPrecision.
	got, err := egostrings.Atoi("0xFFFFFFFFFFFFFFFFF")
	if err == nil {
		t.Fatalf("Atoi() expected an error for a hex value wider than 64 bits, got %d", got)
	}

	if got != 0 {
		t.Errorf("Atoi() = %d, want 0 on error", got)
	}
}

// TestAtoi_ErrorValueIsAlwaysZero is a cross-cutting sanity check (covering
// every recognized literal format) that confirms the documented contract:
// on any error, the returned int is always 0, never a partial or
// out-of-range value.
func TestAtoi_ErrorValueIsAlwaysZero(t *testing.T) {
	badInputs := []string{
		"",
		"not a number",
		"0xZZ",
		"0o9",
		"0b2",
		"''",
		"'ab'",
		"0xFFFFFFFFFFFFFFFFF",
	}

	for _, input := range badInputs {
		t.Run(input, func(t *testing.T) {
			got, err := egostrings.Atoi(input)
			if err == nil {
				t.Fatalf("Atoi(%q) expected an error, got none", input)
			}

			if got != 0 {
				t.Errorf("Atoi(%q) = %d, want 0 alongside error", input, got)
			}
		})
	}
}
