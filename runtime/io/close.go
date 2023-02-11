package io

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// closeFile closes a file.
func closeFile(s *symbols.SymbolTable, args data.List) (interface{}, error) {
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
	} else {
		err = errors.NewError(err).In("Close")
	}

	return err, nil
}
