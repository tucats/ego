package time

import (
	"fmt"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Names of the months of the year, indexed 0..11 (January == index 0). Month
// values are 1-based (January == 1) to match Go's time.Month, so lookups
// subtract one. @tomcole later this should be drawn from localized strings.
var monthNames = []string{
	"January",
	"February",
	"March",
	"April",
	"May",
	"June",
	"July",
	"August",
	"September",
	"October",
	"November",
	"December",
}

// month implements t.Month(). It wraps the native time.Month result as a Month
// scalar so the returned value carries the Month type (for comparison, String()
// dispatch, and reflect.Type), mirroring weekday() above.
func month(s *symbols.SymbolTable, args data.List) (any, error) {
	var result *data.Scalar

	t, err := getTime(s)
	if err == nil {
		result = data.NewScalar(TimeMonthType, int(t.Month()))
	}

	return result, err
}

// monthString implements Month.String().
func monthString(s *symbols.SymbolTable, args data.List) (any, error) {
	monthValue, err := getMonth(s)
	if err != nil {
		return nil, err
	}

	// Match Go's time.Month.String(): an out-of-range value produces a
	// placeholder string (e.g. "%!Month(13)") rather than an error, so String()
	// never fails on a bad value.
	if monthValue < 1 || monthValue > 12 {
		return fmt.Sprintf("%%!Month(%d)", monthValue), nil
	}

	return monthNames[monthValue-1], nil
}

// getMonth looks in the symbol table for the "this" receiver and extracts the
// integer month value from it.
func getMonth(symbols *symbols.SymbolTable) (int, error) {
	// See if we can get a receiver value
	if t, ok := symbols.Get(defs.ThisVariable); ok {
		// If the receiver is a simple int, then we just return that value.
		if intValue, ok := t.(int); ok {
			return intValue, nil
		}

		// If the receiver is a scalar typed as a Month, then extract the
		// integer value and return it.
		if monthValue, ok := t.(*data.Scalar); ok {
			if monthValue.Type().Name() == "Month" {
				return data.Int(monthValue.Value())
			}
		}

		// Not a usable receiver type
		return 0, errors.ErrInvalidType.Context(fmt.Sprint(t)).In("Month function")
	}

	// Couldn't use the receiver
	return 0, errors.ErrNoFunctionReceiver.In("Month function")
}

// date implements time.Date(). It is an Ego wrapper around Go's time.Date so
// the month argument can be supplied as a Month scalar (or a plain int): the
// value is unwrapped and reconstructed as a time.Month here, which the direct
// native call could not do (reflection would reject an int where a time.Month
// is required).
func date(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() != 8 {
		return nil, errors.ErrArgumentCount.Context(args.Len())
	}

	values := make([]int, 7)

	for i := 0; i < 7; i++ {
		v, err := data.Int(args.Get(i))
		if err != nil {
			return nil, err
		}

		values[i] = v
	}

	loc, ok := args.Get(7).(*time.Location)
	if !ok || loc == nil {
		return nil, errors.ErrInvalidType.Context("location").In("Date")
	}

	result := time.Date(values[0], time.Month(values[1]), values[2],
		values[3], values[4], values[5], values[6], loc)

	return result, nil
}
