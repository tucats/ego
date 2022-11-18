package functions

import (
	"time"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

const basicLayout = "Mon Jan 2 15:04:05 MST 2006"

var timeType *datatypes.Type

func initializeType() {
	if timeType == nil {
		structType := datatypes.Structure()
		structType.DefineField("time", &datatypes.InterfaceType)

		t := datatypes.TypeDefinition("time.Time", structType)
		t.DefineFunction("Add", TimeAdd)
		t.DefineFunction("Format", TimeFormat)
		t.DefineFunction("SleepUntil", TimeSleep)
		t.DefineFunction("String", TimeString)
		t.DefineFunction("Sub", TimeSub)
		timeType = t
	}
}

// TimeNow implements time.now().
func TimeNow(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	t := time.Now()

	return MakeTime(&t), nil
}

// TimeParse time.Parse().
func TimeParse(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	str := datatypes.GetString(args[0])
	fmt := basicLayout

	if len(args) > 1 {
		fmt = datatypes.GetString(args[1])
	}

	t, err := time.Parse(fmt, str)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	return MakeTime(&t), nil
}

// TimeAdd implements time.duration().
func TimeAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Add()")
	}

	t, err := getTime(s)
	if errors.Nil(err) {
		d, err := time.ParseDuration(datatypes.GetString(args[0]))
		if errors.Nil(err) {
			t2 := t.Add(d)

			return MakeTime(&t2), nil
		}
	}

	return nil, err
}

// TimeSub implements time.duration().
func TimeSub(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Sub()")
	}

	t, err := getTime(s)
	if errors.Nil(err) {
		d, err := getTimeV(args[0])
		if errors.Nil(err) && d != nil {
			t2 := t.Sub(*d)

			return t2.String(), nil
		}
	}

	return nil, err
}

// TimeFormat implements time.Format().
func TimeFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Format()")
	}

	t, err := getTime(s)
	if !errors.Nil(err) {
		return nil, err
	}

	layout := datatypes.GetString(args[0])

	return t.Format(layout), nil
}

// TimeSleep implements time.SleepUntil().
func TimeSleep(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Sleep()")
	}

	t, err := getTime(s)
	if !errors.Nil(err) {
		return nil, err
	}

	d := time.Until(*t)
	time.Sleep(d)

	return d.String(), nil
}

// TimeFormat implements time.Format().
func TimeString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("String()")
	}

	t, err := getTime(s)
	if !errors.Nil(err) {
		return nil, err
	}

	layout := basicLayout

	return t.Format(layout), nil
}

func TimeSince(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Since()")
	}

	// Get the time value stored in the argument
	t, err := getTimeV(args[0])
	if !errors.Nil(err) {
		return nil, err
	}

	// Calculate duration and return as a string.
	duration := time.Since(*t)

	return duration.String(), nil
}

// getTime looks in the symbol table for the "this" receiver, and
// extracts the time value from it.
func getTime(symbols *symbols.SymbolTable) (*time.Time, *errors.EgoError) {
	if t, ok := symbols.Get("__this"); ok {
		return getTimeV(t)
	}

	return nil, errors.New(errors.ErrNoFunctionReceiver).In("time function")
}

// getTimeV extracts a time.Time value from an Ego time
// object, by looking in the [time] member.
func getTimeV(timeV interface{}) (*time.Time, *errors.EgoError) {
	if m, ok := timeV.(*datatypes.EgoStruct); ok {
		if tv, ok := m.Get("time"); ok {
			if tp, ok := tv.(*time.Time); ok {
				return tp, nil
			}
		}
	}

	return nil, errors.New(errors.ErrNoFunctionReceiver).In("time function")
}

// Make a time object with the given time value.
func MakeTime(t *time.Time) interface{} {
	initializeType()

	r := datatypes.NewStruct(timeType)
	_ = r.Set("time", t)

	r.SetReadonly(true)

	return r
}
