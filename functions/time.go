package functions

import (
	"time"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

const basicLayout = "Mon Jan 2 15:04:05 MST 2006"

// TimeNow implements time.now().
func TimeNow(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t := time.Now()

	return makeTime(&t), nil
}

// TimeParse time.Parse().
func TimeParse(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := util.GetString(args[0])
	fmt := basicLayout

	if len(args) > 1 {
		fmt = util.GetString(args[1])
	}

	t, err := time.Parse(fmt, str)
	if err != nil {
		return nil, err
	}

	return makeTime(&t), nil
}

// TimeAdd implements time.duration().
func TimeAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, NewError("Add", ArgumentCountError)
	}

	t, err := getTime(s)
	if err == nil {
		d, err := time.ParseDuration(util.GetString(args[0]))
		if err == nil {
			t2 := t.Add(d)

			return makeTime(&t2), nil
		}
	}

	return nil, err
}

// TimeSub implements time.duration().
func TimeSub(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, NewError("Sub", ArgumentCountError)
	}

	t, err := getTime(s)
	if err == nil {
		d, err := getTimeV(args[0])
		if err == nil && d != nil {
			t2 := t.Sub(*d)

			return t2.String(), nil
		}
	}

	return nil, err
}

// TimeFormat implements time.Format().
func TimeFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, NewError("Format", ArgumentCountError)
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := util.GetString(args[0])

	return t.Format(layout), nil
}

// TimeSleep implements time.SleepUntil().
func TimeSleep(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, NewError("Sleep", ArgumentCountError)
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	d := time.Until(*t)
	time.Sleep(d)

	return d.String(), nil
}

// TimeFormat implements time.Format().
func TimeString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, NewError("String", ArgumentCountError)
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := basicLayout

	return t.Format(layout), nil
}

// getTime looks in the symbol table for the "this" receiver, and
// extracts the time value from it.
func getTime(symbols *symbols.SymbolTable) (*time.Time, error) {
	if t, ok := symbols.Get("__this"); ok {
		return getTimeV(t)
	}

	return nil, NewError("time", NoFunctionReceiver)
}

// getTimeV extracts a time.Time value from an Ego time
// object, by looking in the [time] member.
func getTimeV(timeV interface{}) (*time.Time, error) {
	if m, ok := timeV.(map[string]interface{}); ok {
		if tv, ok := m["time"]; ok {
			if tp, ok := tv.(*time.Time); ok {
				return tp, nil
			}
		}
	}

	return nil, NewError("time", NoFunctionReceiver)
}

func makeTime(t *time.Time) interface{} {
	r := map[string]interface{}{
		"time":       t,
		"Add":        TimeAdd,
		"Format":     TimeFormat,
		"SleepUntil": TimeSleep,
		"String":     TimeString,
		"Sub":        TimeSub,
	}
	datatypes.SetMetadata(r, datatypes.TypeMDKey, "time")

	return r
}
