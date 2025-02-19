package http

import (
	nativeHttp "net/http"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Add(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	header, err := getHeader(s)
	if err != nil {
		return nil, errors.New(err).In("Add")
	}

	key := data.String(args.Get(0))
	value := data.String(args.Get(1))

	header.Add(key, value)

	return nil, nil
}

func Del(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	header, err := getHeader(s)
	if err != nil {
		return nil, errors.New(err).In("Del")
	}

	key := data.String(args.Get(0))

	header.Del(key)

	return nil, nil
}

func getHeader(s *symbols.SymbolTable) (nativeHttp.Header, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_header")
			if header, ok := value.(nativeHttp.Header); ok {
				return header, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
}
