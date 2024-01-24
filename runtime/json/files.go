package json

import (
	gojson "encoding/json"
	"io"
	"os"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func readFile(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err      error
		fileName string
		bytes    []byte
		v        interface{}
	)

	// Read the file
	fileName = data.String(args.Get(0))
	if r, err := os.Open(fileName); err != nil {
		return data.NewList(nil, errors.New(err).In("ReadFile")), err
	} else {
		defer r.Close()

		if bytes, err = io.ReadAll(r); err != nil {
			return data.NewList(nil, errors.New(err).In("ReadFile")), err
		}
	}

	err = gojson.Unmarshal(bytes, &v)
	if err != nil {
		return data.NewList(nil, errors.New(err).In("ReadFile")), err
	}

	// If the result is a map or an array, convert ot the Ego version
	// of a map or array.
	if m, ok := v.(map[string]interface{}); ok {
		v = data.NewMapFromMap(m)
	} else if a, ok := v.([]interface{}); ok {
		v = data.NewArrayFromInterfaces(data.InterfaceType, a...)
	}

	return data.NewList(v, nil), nil
}

func writeFile(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err      error
		fileName string
		bytes    []byte
	)

	// Make json from the object
	if bytes, err = gojson.Marshal(data.Sanitize(args.Get(1))); err != nil {
		return errors.New(err).In("WriteFile"), err
	}

	// Write the file
	fileName = data.String(args.Get(0))
	if w, err := os.Create(fileName); err != nil {
		return errors.New(err).In("WriteFile"), err
	} else {
		defer w.Close()

		if _, err = w.Write(bytes); err != nil {
			return errors.New(err).In("WriteFile"), err
		}
	}

	return data.NewList(nil), nil
}
