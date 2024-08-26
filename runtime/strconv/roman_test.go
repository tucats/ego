package strconv

import (
	"fmt"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Test_doIntToRoman_returnsCorrectRomanNumeralForInput1(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(1)

	want := "I"
	got, err := doIntToRoman(s, args)

	if err != nil {
		t.Errorf("doIntToRoman() error = %v", err)
	}

	if got != want {
		t.Errorf("doIntToRoman() got = %v, want %v", got, want)
	}
}
func TestDoIntToRoman_ShouldReturnCorrectRomanNumeralForInput4(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(4)

	want := "IV"
	got, err := doIntToRoman(s, args)

	if err != nil {
		t.Errorf("doIntToRoman() error = %v", err)
		return
	}

	if got != want {
		t.Errorf("doIntToRoman() got = %v, want %v", got, want)
	}
}
func Test_doIntToRoman_58(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(58)

	want := "LVIII"
	got, err := doIntToRoman(s, args)

	if err != nil {
		t.Errorf("doIntToRoman(58) error: %v", err)
	}

	if got.(string) != want {
		t.Errorf("doIntToRoman(58) got: %v, want: %v", got, want)
	}
}
func TestDoIntToRoman_1994(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(1994)

	want := "MCMXCIV"
	got, err := doIntToRoman(s, args)

	if err != nil {
		t.Fatalf("doIntToRoman() error = %v", err)
	}

	if got != want {
		t.Errorf("doIntToRoman() got = %v, want %v", got, want)
	}
}
func Test_doIntToRoman_3999(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(3999)

	want := "MMMCMXCIX"
	got, err := doIntToRoman(s, args)

	if err != nil {
		t.Errorf("doIntToRoman() error = %v", err)
	}

	if got != want {
		t.Errorf("doIntToRoman() got = %v, want %v", got, want)
	}
}
func Test_doIntToRoman_InvalidZero(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(0)

	_, err := doIntToRoman(s, args)

	if err == nil {
		t.Error("Expected error not reported for input 0")
	}

	expectedError := errors.ErrInvalidRomanRange.In("Itor")
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}
func TestDoIntToRoman_OutOfRange(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(4000)

	_, err := doIntToRoman(s, args)

	if err == nil {
		t.Errorf("Expected error for input 4000, got nil")
	} else if err != errors.ErrInvalidRomanRange.In("Itor") {
		t.Errorf("Expected error %v, got %v", errors.ErrInvalidRomanRange.In("Itor"), err)
	}
}
func Test_doIntToRoman_InvalidNegative(t *testing.T) {
	s := symbols.NewSymbolTable("testing")
	args := data.NewList(-1)

	_, err := doIntToRoman(s, args)

	if err == nil {
		t.Error("Expected error for input -1, but got nil")
	} else if err != errors.ErrInvalidRomanRange.In("Itor") {
		t.Errorf("Expected error %v, but got %v", errors.ErrInvalidRomanRange.In("Itor"), err)
	}
}
func Test_doIntToRoman_MultipleConsecutiveIdenticalRomanNumerals(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		input int
		want  string
	}{
		{input: 4, want: "IV"},
		{input: 9, want: "IX"},
		{input: 14, want: "XIV"},
		{input: 40, want: "XL"},
		{input: 90, want: "XC"},
		{input: 140, want: "CXL"},
		{input: 400, want: "CD"},
		{input: 900, want: "CM"},
		{input: 1400, want: "MCD"},
		{input: 3999, want: "MMMCMXCIX"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%d", tt.input), func(t *testing.T) {
			got, err := doIntToRoman(s, data.NewList(tt.input))
			if err != nil {
				t.Errorf("doIntToRoman() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("doIntToRoman() = %v, want %v", got, tt.want)
			}
		})
	}
}
