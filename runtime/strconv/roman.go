// Converts between integers and Roman Numeral strings.
//
// Currently only supports Roman Numerals without viniculum (1-3999) and will throw an error for
// numbers outside of that range. See here for details on viniculum:
// https://en.wikipedia.org/wiki/Roman_numerals#Large_numbers
package strconv

import (
	"bytes"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

const (
	_maxRoman = 3999
	_minRoman = 1
)

var (
	InvalidRomanNumeral = errors.ErrInvalidRomanNumeral
	IntegerOutOfBounds  = errors.ErrInvalidRomanRange
)

// numeral describes the value and symbol of a single roman numeral
type numeral struct {
	val int
	sym []byte
}

// _numerals are all unique numerals ordered from largest to smallest
var _numerals = []numeral{
	{1000, []byte("M")},
	{900, []byte("CM")},
	{500, []byte("D")},
	{400, []byte("CD")},
	{100, []byte("C")},
	{90, []byte("XC")},
	{50, []byte("L")},
	{40, []byte("XL")},
	{10, []byte("X")},
	{9, []byte("IX")},
	{5, []byte("V")},
	{4, []byte("IV")},
	{1, []byte("I")},
}

// lookup arrays used for converting from an int to a roman numeral extremely quickly.
// method from here: https://rosettacode.org/wiki/Roman_numerals/Encode#Go
var (
	m0 = []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX"}
	m1 = []string{"", "X", "XX", "XXX", "XL", "L", "LX", "LXX", "LXXX", "XC"}
	m2 = []string{"", "C", "CC", "CCC", "CD", "D", "DC", "DCC", "DCCC", "CM"}
	m3 = []string{"", "M", "MM", "MMM"}
)

// intToRomanString converts an integer value to a roman numeral string. An error is
// returned if the integer is not between 1 and 3999.
func intToRomanString(input int) (string, error) {
	if outOfBounds(input) {
		return "", IntegerOutOfBounds
	}
	return intToRoman(input), nil
}

// intToRomanBytes converts an integer value to a roman numeral byte array. An error is
// returned if the integer is not between 1 and 3999.
func intToRomanBytes(input int) ([]byte, error) {
	str, err := intToRomanString(input)
	return []byte(str), err
}

// outOfBounds checks to ensure an input value is valid for roman numerals without the need of
// vinculum (used for values of 4,000 and greater)
func outOfBounds(input int) bool {
	return input < _minRoman || input > _maxRoman
}

func intToRoman(n int) string {
	// This is efficient in Go. The 4 operands are evaluated,
	// then a single allocation is made of the exact size needed for the result.
	return m3[n%1e4/1e3] + m2[n%1e3/1e2] + m1[n%100/10] + m0[n%10]
}

// romanStringToInt converts a roman numeral string to an integer. Roman numerals for numbers
// outside of the range 1 to 3,999 will return an error. Empty strings will return 0
// with no error thrown.
func romanStringToInt(input string) (int, error) {
	return romanBytesToInt([]byte(input))
}

// romanBytesToInt converts a roman numeral byte array to an integer. Roman numerals for numbers
// outside of the range 1 to 3,999 will return an error. Nil or empty []byte will return 0
// with no error thrown.
func romanBytesToInt(input []byte) (int, error) {
	if input == nil || len(input) == 0 {
		return 0, nil
	}
	if output, ok := romanToInt(input); ok {
		return output, nil
	}
	return 0, InvalidRomanNumeral
}

func romanToInt(input []byte) (int, bool) {
	var output int
	for _, rom := range _numerals {
		for bytes.HasPrefix(input, rom.sym) {
			output += rom.val
			input = input[len(rom.sym):]
		}
	}
	// If we are still left with input string values then the
	// input was invalid and the bool is returned as False
	return output, len(input) == 0
}

// Ego function that converts an integer to a Roman numeral string.
func doIntToRoman(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	input := int(data.Int(args.Get(0)))
	roman, err := intToRomanString(input)

	return roman, err
}

// Ego function that converts a Roman numeral string to an integer.
func doRomanToInt(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	input := strings.ToUpper(data.String(args.Get(0)))
	if len(strings.TrimSpace(input)) == 0 {
		return 0, nil
	}

	roman, err := romanStringToInt(input)

	return roman, err
}
