package time

import (
	"fmt"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Names of the days of the week. @tomcole later this should be
// drawn from localized strings.

var dayNames = []string{
	"Sunday",
	"Monday",
	"Tuesday",
	"Wednesday",
	"Thursday",
	"Friday",
	"Saturday",
}

// weekday implements t.Weekday().
func weekday(s *symbols.SymbolTable, args data.List) (any, error) {
	var result *data.Scalar

	t, err := getTime(s)
	if err == nil {
		dayOfWeek := int(t.Weekday())

		result = data.NewScalar(TimeWeekdayType, dayOfWeek)
	}

	return result, err
}

func weekdayString(s *symbols.SymbolTable, args data.List) (any, error) {
	weekDay, err := getWeekday(s)
	if err != nil {
		return nil, err
	}

	// Match Go's time.Weekday.String(): an out-of-range value produces a
	// placeholder string (e.g. "%!Weekday(9)") rather than an error, so
	// String() never fails on a bad value. This keeps the method's single
	// string return value honest -- it always returns a string, never a throw.
	if weekDay < 0 || weekDay > 6 {
		return fmt.Sprintf("%%!Weekday(%d)", weekDay), nil
	}

	return dayNames[weekDay], nil
}

// getWeekday looks in the symbol table for the "this" receiver, and
// extracts the time.Weekday value from it.
func getWeekday(symbols *symbols.SymbolTable) (int, error) {
	// See if we can get a receiver value
	if t, ok := symbols.Get(defs.ThisVariable); ok {
		// If the receiver is a simple int, then we just
		// return that value.
		if intValue, ok := t.(int); ok {
			return intValue, nil
		}

		// If the receiver is a scalar typed as a Weekday,
		// then extract the integer value and return it.
		if weekdayValue, ok := t.(*data.Scalar); ok {
			if weekdayValue.Type().Name() == "Weekday" {
				return data.Int(weekdayValue.Value())
			}
		}

		// Not a usable receiver type
		return 0, errors.ErrInvalidType.Context(fmt.Sprint(t)).In("Weekday function")
	}

	// Couldn't use the receiver
	return 0, errors.ErrNoFunctionReceiver.In("Weekday function")
}
