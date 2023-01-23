package functions

import (
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

const basicLayout = "Mon Jan 2 15:04:05 MST 2006"

var timeType *data.Type

func initializeType(s *symbols.SymbolTable) error {
	if timeType == nil {
		structType := data.StructureType()
		structType.DefineField("Time", data.InterfaceType)

		t := data.TypeDefinition("time.Time", structType)
		t.DefineFunction("Add", nil, TimeAdd)
		t.DefineFunction("Format", nil, TimeFormat)
		t.DefineFunction("SleepUntil", nil, TimeSleep)
		t.DefineFunction("String", nil, TimeString)
		t.DefineFunction("Sub", nil, TimeSub)
		timeType = t

		if s != nil {
			if p, found := s.Get("time"); found {
				if pkg, ok := p.(*data.Package); ok {
					pkg.Set("Time", t)

					if err := s.Set("time", pkg); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// TimeNow implements time.Now().
func TimeNow(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t := time.Now()

	return MakeTime(&t, s), nil
}

// TimeParse time.Parse().
func TimeParse(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])
	fmt := basicLayout

	if len(args) > 1 {
		fmt = data.String(args[1])
	}

	t, err := time.Parse(fmt, str)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return MakeTime(&t, s), nil
}

// TimeAdd implements time.duration().
func TimeAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Add()")
	}

	t, err := getTime(s)
	if err == nil {
		d, err := time.ParseDuration(data.String(args[0]))
		if err == nil {
			t2 := t.Add(d)

			return MakeTime(&t2, s), nil
		}
	}

	return nil, err
}

// TimeSub implements time.duration().
func TimeSub(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Sub()")
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
		return nil, errors.ErrArgumentCount.In("Format()")
	}

	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := data.String(args[0])

	return t.Format(layout), nil
}

// TimeSleep implements time.SleepUntil().
func TimeSleep(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

// TimeFormat implements time.Format().
func TimeString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

func TimeSince(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

	return duration.String(), nil
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
		if tv, ok := m.Get("Time"); ok {
			if tp, ok := tv.(*time.Time); ok {
				return tp, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("time function")
}

// Make a time object with the given time value.
func MakeTime(t *time.Time, s *symbols.SymbolTable) interface{} {
	if err := initializeType(s); err != nil {
		ui.Log(ui.InternalLogger, "Failed to create time.Time type, %v", err)
	}

	r := data.NewStruct(timeType)
	_ = r.Set("Time", t)

	r.SetReadonly(true)

	return r
}
