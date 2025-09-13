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
func doIntToRoman(s *symbols.SymbolTable, args data.List) (any, error) {
	input, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.ErrInvalidInteger.In("Itor")
	}

	if input < 1 || input > 3999 {
		return nil, errors.ErrInvalidRomanRange.In("Itor")
	}

	roman, err := romannumeral.IntToString(input)

	return roman, err
}

// Ego function that converts a Roman numeral string to an integer.
func doRomanToInt(s *symbols.SymbolTable, args data.List) (any, error) {
	input := strings.TrimSpace(strings.ToUpper(data.String(args.Get(0))))
	if len(input) == 0 {
		return 0, nil
	}

	roman, err := romannumeral.StringToInt(input)
	if err != nil {
		return nil, errors.ErrInvalidRomanNumeral.In("Itor")
	}

	return roman, err
}
