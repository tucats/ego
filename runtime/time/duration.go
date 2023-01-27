package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Sleep implements time.Sleep(d time.Duration).
func Sleep(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	duration := data.Int64(args[0])
	time.Sleep(time.Duration(duration))

	return nil, nil
}

// Add implements t.Add(duration string).
func Add(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Add()")
	}

	t, err := getTime(s)
	if err == nil {
		t2 := t.Add(time.Duration(data.Int64(args[0])))

		return MakeTime(&t2, s), nil
	}

	return nil, err
}

// Sub implements t.Sub(t time.Time).
func Sub(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Sub()")
	}

	t, err := getTime(s)
	if err == nil {
		d, err := getTimeV(args[0])
		if err == nil && d != nil {
			d := t.Sub(*d)

			return int64(d), nil
		}
	}

	return nil, err
}
