package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// parseDuration implements the time.parseDuration(d string)(time.Duration, error) function.
func parseDuration(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	str := data.String(args.Get(0))

	t, err := time.ParseDuration(str)
	if err != nil {
		return data.NewList(nil, err), errors.New(err)
	}

	d := data.NewStruct(durationType)
	_ = d.Set("duration", t)

	return data.NewList(d, nil), nil
}

func durationHours(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Hours(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationMinutes(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Minutes(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationSeconds(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Seconds(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationMilliseconds(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Milliseconds(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationMicroseconds(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Microseconds(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationNanoseconds(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)

	if duration != nil {
		return duration.Nanoseconds(), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func durationString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)
	withSpaces := false

	if args.Len() > 0 {
		withSpaces = data.Bool(args.Get(0))
	}

	if duration != nil {
		return util.FormatDuration(*duration, withSpaces), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func getDuration(s *symbols.SymbolTable) *time.Duration {
	if this, found := s.Get(defs.ThisVariable); found {
		if d, ok := this.(*data.Struct); ok {
			if v, found := d.Get("duration"); found {
				if duration := v.(time.Duration); ok {
					return &duration
				}
			}
		}
	}

	return nil
}

func getDurationV(value interface{}) *time.Duration {
	if d, ok := value.(*data.Struct); ok {
		if v, found := d.Get("duration"); found {
			if duration := v.(time.Duration); ok {
				return &duration
			}
		}
	}

	return nil
}

// sleep implements time.Sleep(d time.Duration).
func sleep(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDurationV(args.Get(0))

	if duration == nil {
		return false, nil
	}

	time.Sleep(*duration)

	return true, nil
}

// add implements t.Add(duration string).
func add(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		t2 := t.Add(*getDurationV(args.Get(0)))

		return makeTime(&t2, s), nil
	}

	return nil, err
}

// clock implements time.Clock().
func clock(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		h, m, s := t.Clock()

		return data.NewList(h, m, s), nil
	}

	return nil, err
}

// after implements t.After(t time.Time).
func after(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		t2, err := getTimeV(args.Get(0))
		if err == nil && t2 != nil {
			return t.After(*t2), nil
		}
	}

	return nil, err
}

// before implements t.Before(t time.Time).
func before(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		t2, err := getTimeV(args.Get(0))
		if err == nil && t2 != nil {
			return t.Before(*t2), nil
		}
	}

	return nil, err
}

// sub implements t.Sub(t time.Time).
func sub(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		d, err := getTimeV(args.Get(0))
		if err == nil && d != nil {
			d := t.Sub(*d)

			r := data.NewStruct(durationType)
			_ = r.Set("duration", d)

			return r, nil
		}
	}

	return nil, err
}
