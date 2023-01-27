package io

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Close closes a file.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	f, err := getFile("Close", s)
	if err == nil {
		e2 := f.Close()
		if e2 != nil {
			err = errors.NewError(e2)
		}

		this := getThis(s)

		this.SetAlways(validFieldName, false)
		this.SetAlways(modeFieldName, "closed")
		this.SetAlways(fileFieldName, nil)
		this.SetAlways(nameFieldName, "")
	}

	return err, nil
}
