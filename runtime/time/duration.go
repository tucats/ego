package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// parseDuration implements the time.parseDuration(d string)(time.Duration, error) function.
func parseDuration(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	str := data.String(args.Get(0))

	t, err := time.ParseDuration(str)
	if err != nil {
		return data.NewList(nil, err), errors.NewError(err)
	}

	d := data.NewStruct(durationType)
	_ = d.Set("duration", t)

	return data.NewList(d, nil), nil
}

func DurationString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)
	if duration != nil {
		return duration.String(), nil
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

// sleepForDuration implements time.sleepForDuration(d time.Duration).
func sleepForDuration(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDurationV(args.Get(0))

	if duration == nil {
		return false, nil
	}

	time.Sleep(*duration)

	return true, nil
}

// Add implements t.Add(duration string).
func Add(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		t2 := t.Add(*getDurationV(args.Get(0)))

		return MakeTime(&t2, s), nil
	}

	return nil, err
}

// Sub implements t.Sub(t time.Time).
func Sub(s *symbols.SymbolTable, args data.List) (interface{}, error) {
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
