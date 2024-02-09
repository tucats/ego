package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// now implements time.New().
func now(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t := time.Now()

	return makeTime(&t, s), nil
}

// parseTime implements the time.Parse()(time.Time, error) function.
func parseTime(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	str := data.String(args.Get(0))
	fmt := basicLayout

	if args.Len() > 1 {
		fmt = data.String(args.Get(1))
	}

	t, err := time.Parse(fmt, str)
	if err != nil {
		return data.NewList(nil, err), errors.New(err)
	}

	return data.NewList(makeTime(&t, s), nil), nil
}

// sleepUntil implements time.sleepUntil().
func sleepUntil(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	d := time.Until(*t)
	time.Sleep(d)

	return d.String(), nil
}

// sinceTime implements time.Since().
func sinceTime(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Get the time value stored in the argument
	t, err := getTimeV(args.Get(0))
	if err != nil {
		return nil, err
	}

	// Calculate duration and return as a string.
	duration := time.Since(*t)
	d := data.NewStruct(durationType)
	_ = d.Set("duration", duration)

	return d, nil
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

// unix implements the time.Unix() function.
func unix(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	sec := data.Int64(args.Get(0))
	nsec := data.Int64(args.Get(1))

	t := time.Unix(sec, nsec)

	return makeTime(&t, s), nil
}

// Make a time object with the given time value.
func makeTime(t *time.Time, s *symbols.SymbolTable) interface{} {
	Initialize(s)

	r := data.NewStruct(timeType)
	_ = r.Set("time", t)

	r.SetReadonly(true)

	return r
}
