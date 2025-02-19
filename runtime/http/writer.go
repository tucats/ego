package http

import (
	"encoding/json"
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

	// If the parameter is an array of bytes, write it directly since this is
	// the default behavior of Write() in Go's net/http package.
	if array, ok := v.(*data.Array); ok {
		if array.Type().BaseType() == data.ByteType {
			b := array.GetBytes()
			err = setSize(s, len(b))

			if err == nil {
				return w.Write(b)
			}

			return 0, err
		}
	}

	// For anything else, let's check and see if we are doing JSON versus TEXT output.
	// Format the message accordingly. Note that if both JSON and TEXT are allowed,
	// this assumes TEXT is the preferred format.
	isJson, _ := getJSON(s)
	isText, _ := getText(s)

	doJson := isJson
	if isText {
		doJson = false
	}

	if doJson {
		b, _ := json.Marshal(v)
		err = setSize(s, len(b))

		if err == nil {
			return w.Write(b)
		}

		return 0, err
	} else {
		text := data.FormatUnquoted(v) + "\n"
		b := []byte(text)
		err = setSize(s, len(b))

		if err == nil {
			return w.Write(b)
		}

		return 0, err
	}
}

func WriteHeader(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	_, err := getWriter(s)
	if err == nil {
		status, err := data.Int(args.Get(0))
		if err == nil {
			setStatus(s, status)

			return nil, nil
		}
	}

	return nil, errors.New(err).In("WriteStatus")
}

func Header(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_headers")
			if s, ok := value.(*data.Struct); ok {
				return s, nil
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
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

func getJSON(s *symbols.SymbolTable) (bool, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_json")

			return data.Bool(value)
		}
	}

	return false, errors.ErrNoFunctionReceiver
}

func getText(s *symbols.SymbolTable) (bool, error) {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			value := s.GetAlways("_text")

			return data.Bool(value)
		}
	}

	return false, errors.ErrNoFunctionReceiver
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

func setStatus(s *symbols.SymbolTable, status int) error {
	if this, ok := s.Get(defs.ThisVariable); ok {
		if s, ok := this.(*data.Struct); ok {
			s.SetAlways("_status", status)

			return nil
		}
	}

	return errors.ErrNoFunctionReceiver
}
