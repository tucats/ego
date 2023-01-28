package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ParseDuration implements the time.ParseDuration(d string)(time.Duration, error) function.
func ParseDuration(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])

	t, err := time.ParseDuration(str)
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	d := data.NewStruct(durationType)
	_ = d.Set("duration", t)

	return data.List(d, nil), nil
}

func DurationString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

// Sleep implements time.Sleep(d time.Duration).
func Sleep(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	duration := getDurationV(args[0])

	if duration == nil {
		return false, nil
	}

	time.Sleep(*duration)

	return true, nil
}

// Add implements t.Add(duration string).
func Add(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		t2 := t.Add(*getDurationV(args[0]))

		return MakeTime(&t2, s), nil
	}

	return nil, err
}

// Sub implements t.Sub(t time.Time).
func Sub(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTime(s)
	if err == nil {
		d, err := getTimeV(args[0])
		if err == nil && d != nil {
			d := t.Sub(*d)

			r := data.NewStruct(durationType)
			_ = r.Set("duration", d)

			return r, nil
		}
	}

	return nil, err
}
