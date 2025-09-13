package http

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Set(s *symbols.SymbolTable, args data.List) (any, error) {
	header, err := getHeader(s)
	if err != nil {
		return nil, errors.New(err).In("Add")
	}

	key := data.String(args.Get(0))
	value := data.String(args.Get(1))
	array := data.NewArray(data.StringType, 1)

	array.Set(0, value)
	header.SetAlways(key, array)

	return nil, nil
}

func Add(s *symbols.SymbolTable, args data.List) (any, error) {
	header, err := getHeader(s)
	if err != nil {
		return nil, errors.New(err).In("Add")
	}

	key := data.String(args.Get(0))
	value := data.String(args.Get(1))

	if arrayV, found, _ := header.Get(key); found {
		array := arrayV.(*data.Array)
		array.Append(value)
		header.SetAlways(key, array)
	} else {
		array := data.NewArray(data.StringType, 1)
		array.Set(0, value)
		header.SetAlways(key, array)
	}

	return nil, nil
}

func Del(s *symbols.SymbolTable, args data.List) (any, error) {
	header, err := getHeader(s)
	if err != nil {
		return nil, errors.New(err).In("Del")
	}

	key := data.String(args.Get(0))
	_, err = header.Delete(key)

	return nil, err
}

func getHeader(s *symbols.SymbolTable) (*data.Map, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_headers")
			if header, ok := value.(*data.Map); ok {
				return header, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
}
