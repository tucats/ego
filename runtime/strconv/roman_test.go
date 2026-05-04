package strconv

import (
	"fmt"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Helper: call doIntToRoman and extract (string, error) from the returned data.List.
func callItor(t *testing.T, input int) (string, error) {
	t.Helper()

	s := symbols.NewSymbolTable("testing")
	result, err := doIntToRoman(s, data.NewList(input))
	if err != nil {
		return "", err
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("doIntToRoman(%d): result is not data.List: %T", input, result)
	}

	if list.Len() != 2 {
		t.Fatalf("doIntToRoman(%d): expected 2-element list, got %d", input, list.Len())
	}

	if listErr, _ := list.Get(1).(error); listErr != nil {
		return "", listErr
	}

	roman, _ := list.Get(0).(string)

	return roman, nil
}

func Test_doIntToRoman_returnsCorrectRomanNumeralForInput1(t *testing.T) {
	got, err := callItor(t, 1)
	if err != nil {
		t.Errorf("doIntToRoman(1) error = %v", err)
	}

	if got != "I" {
		t.Errorf("doIntToRoman(1) got = %v, want I", got)
	}
}

func TestDoIntToRoman_ShouldReturnCorrectRomanNumeralForInput4(t *testing.T) {
	got, err := callItor(t, 4)
	if err != nil {
		t.Errorf("doIntToRoman(4) error = %v", err)

		return
	}

	if got != "IV" {
		t.Errorf("doIntToRoman(4) got = %v, want IV", got)
	}
}

func Test_doIntToRoman_58(t *testing.T) {
	got, err := callItor(t, 58)
	if err != nil {
		t.Errorf("doIntToRoman(58) error: %v", err)
	}

	if got != "LVIII" {
		t.Errorf("doIntToRoman(58) got: %v, want: LVIII", got)
	}
}

func TestDoIntToRoman_1994(t *testing.T) {
	got, err := callItor(t, 1994)
	if err != nil {
		t.Fatalf("doIntToRoman(1994) error = %v", err)
	}

	if got != "MCMXCIV" {
		t.Errorf("doIntToRoman(1994) got = %v, want MCMXCIV", got)
	}
}

func Test_doIntToRoman_3999(t *testing.T) {
	got, err := callItor(t, 3999)
	if err != nil {
		t.Errorf("doIntToRoman(3999) error = %v", err)
	}

	if got != "MMMCMXCIX" {
		t.Errorf("doIntToRoman(3999) got = %v, want MMMCMXCIX", got)
	}
}

func Test_doIntToRoman_InvalidZero(t *testing.T) {
	_, err := callItor(t, 0)
	if err == nil {
		t.Error("Expected error not reported for input 0")
	}

	expectedError := errors.ErrInvalidRomanRange.In("Itor")
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestDoIntToRoman_OutOfRange(t *testing.T) {
	_, err := callItor(t, 4000)
	if err == nil {
		t.Errorf("Expected error for input 4000, got nil")
	} else if err.Error() != errors.ErrInvalidRomanRange.In("Itor").Error() {
		t.Errorf("Expected error %v, got %v", errors.ErrInvalidRomanRange.In("Itor"), err)
	}
}

func Test_doIntToRoman_InvalidNegative(t *testing.T) {
	_, err := callItor(t, -1)
	if err == nil {
		t.Error("Expected error for input -1, but got nil")
	} else if err.Error() != errors.ErrInvalidRomanRange.In("Itor").Error() {
		t.Errorf("Expected error %v, but got %v", errors.ErrInvalidRomanRange.In("Itor"), err)
	}
}

func Test_doIntToRoman_MultipleConsecutiveIdenticalRomanNumerals(t *testing.T) {
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
			got, err := callItor(t, tt.input)
			if err != nil {
				t.Errorf("doIntToRoman(%d) error = %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("doIntToRoman(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
