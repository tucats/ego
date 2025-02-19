package http

import (
	nativeHttp "net/http"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Write(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	w, err := getWriter(s)
	if err != nil {
		return nil, errors.New(err).In("Write")
	}

	v := args.Get(0)
	if b, ok := v.([]byte); ok {
		err = setSize(s, len(b))
		if err == nil {
			return w.Write(b)
		}

		return 0, err
	}

	if a, ok := v.(*data.Array); ok {
		var b []byte

		if a.Type().BaseType() == data.ByteType {
			b = a.GetBytes()

			return w.Write(b)
		}
	}

	return 0, errors.ErrArgumentType.In("Write")
}

func WriteStatus(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	w, err := getWriter(s)
	if err == nil {
		status, err := data.Int(args.Get(0))
		if err == nil {
			w.WriteHeader(status)

			return nil, nil
		}
	}

	return nil, errors.New(err).In("WriteStatus")
}

func Header(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	w, err := getWriter(s)
	if err != nil {
		return nil, errors.New(err).In("Header")
	}

	header := w.Header()

	result := data.NewStructOfTypeFromMap(HeaderType, map[string]interface{}{
		"_header": header,
	})

	return result, nil
}

func getWriter(s *symbols.SymbolTable) (nativeHttp.ResponseWriter, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_writer")
			if writer, ok := value.(nativeHttp.ResponseWriter); ok {
				return writer, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
}

func setSize(s *symbols.SymbolTable, size int) error {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_size")
			if oldSize, ok := value.(int); ok {
				oldSize += size
				s.SetAlways("_size", oldSize)

				return nil
			}
		}
	}

	return errors.ErrNoFunctionReceiver
}
