package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)


// Now implements time.Now().
func Now(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t := time.Now()

	return MakeTime(&t, s), nil
}

// Parse implements the time.Parse()(time.Time, error) function.
func Parse(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])
	fmt := basicLayout

	if len(args) > 1 {
		fmt = data.String(args[1])
	}

	t, err := time.Parse(fmt, str)
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	return data.List(MakeTime(&t, s), nil), nil
}

// ParseDuration implements the time.ParseDuration(d string)(time.Duration, error) function.
func ParseDuration(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])

	t, err := time.ParseDuration(str)
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	return data.List(int64(t), nil), nil
}

// TimeFormat implements time.Format().
func Format(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Format()")
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := data.String(args[0])

	return t.Format(layout), nil
}

// SleepUntil implements time.SleepUntil().
func SleepUntil(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, errors.ErrArgumentCount.In("Sleep()")
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	d := time.Until(*t)
	time.Sleep(d)

	return d.String(), nil
}

// String implements t.String().
func String(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, errors.ErrArgumentCount.In("String()")
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := basicLayout

	return t.Format(layout), nil
}

// Since implements time.Since().
func Since(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Since()")
	}

	// Get the time value stored in the argument
	t, err := getTimeV(args[0])
	if err != nil {
		return nil, err
	}

	// Calculate duration and return as a string.
	duration := time.Since(*t)

	return int64(duration), nil
}

// getTime looks in the symbol table for the "this" receiver, and
// extracts the time value from it.
func getTime(symbols *symbols.SymbolTable) (*time.Time, error) {
	if t, ok := symbols.Get(defs.ThisVariable); ok {
		return getTimeV(t)
	}

	return nil, errors.ErrNoFunctionReceiver.In("time function")
}

// getTimeV extracts a time.Time value from an Ego time
// object, by looking in the [time] member.
func getTimeV(timeV interface{}) (*time.Time, error) {
	if m, ok := timeV.(*data.Struct); ok {
		if tv, ok := m.Get("time"); ok {
			if tp, ok := tv.(*time.Time); ok {
				return tp, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("time function")
}

// Make a time object with the given time value.
func MakeTime(t *time.Time, s *symbols.SymbolTable) interface{} {
	Initialize(s)

	r := data.NewStruct(timeType)
	_ = r.Set("time", t)

	r.SetReadonly(true)

	return r
}
