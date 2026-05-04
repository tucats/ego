// Converts between integers and Roman Numeral strings.
//
// Currently only supports Roman Numerals without viniculum (1-3999) and will throw an error for
// numbers outside of that range. See here for details on viniculum:
// https://en.wikipedia.org/wiki/Roman_numerals#Large_numbers
package strconv

import (
	"strings"

	"github.com/brandenc40/romannumeral"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Ego function that converts an integer to a Roman numeral string.
// Returns a data.List of (string, error) so callers can use two-value
// assignment: r, err := strconv.Itor(n)
func doIntToRoman(s *symbols.SymbolTable, args data.List) (any, error) {
	input, err := data.Int(args.Get(0))
	if err != nil {
		return data.NewList(nil, errors.ErrInvalidInteger.In("Itor")), nil
	}

	if input < 1 || input > 3999 {
		return data.NewList(nil, errors.ErrInvalidRomanRange.In("Itor")), nil
	}

	roman, romanErr := romannumeral.IntToString(input)
	if romanErr != nil {
		return data.NewList(nil, romanErr), nil
	}

	return data.NewList(roman, nil), nil
}

// Ego function that converts a Roman numeral string to an integer.
// Returns a data.List of (int, error) so callers can use two-value
// assignment: n, err := strconv.Rtoi(s)
func doRomanToInt(s *symbols.SymbolTable, args data.List) (any, error) {
	input := strings.TrimSpace(strings.ToUpper(data.String(args.Get(0))))
	if len(input) == 0 {
		return data.NewList(0, nil), nil
	}

	roman, err := romannumeral.StringToInt(input)
	if err != nil {
		return data.NewList(nil, errors.ErrInvalidRomanNumeral.In("Rtoi")), nil
	}

	return data.NewList(roman, nil), nil
}
