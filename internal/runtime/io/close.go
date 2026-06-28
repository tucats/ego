package io

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// closeFile closes a file.
func closeFile(s *symbols.SymbolTable, args data.List) (any, error) {
	f, err := getFile("Close", s)
	if err == nil {
		e2 := f.Close()
		if e2 != nil {
			err = errors.New(e2)
		}

		this := getThis(s)

		this.SetAlways(validFieldName, false)
		this.SetAlways(modeFieldName, "closed")
		this.SetAlways(fileFieldName, nil)
		this.SetAlways(nameFieldName, "")
	} else {
		err = errors.New(err).In("Close")
	}

	return err, nil
}
