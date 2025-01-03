package json

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// marshal writes a JSON string from arbitrary data.
func marshal(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var b strings.Builder

	if args.Len() == 1 {
		jsonBuffer, err := json.Marshal(data.Sanitize(args.Get(0)))
		if err != nil {
			err = errors.New(err).In("Marshal")
		}

		list := data.NewList(data.NewArray(data.ByteType, 0).Append(jsonBuffer), err)

		return list, err
	}

	b.WriteString("[")

	for n, v := range args.Elements() {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.Marshal(data.Sanitize(v))
		if err != nil {
			return nil, errors.New(err)
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")
	jsonBuffer := []byte(b.String())

	list := data.NewList(data.NewArray(data.ByteType, 0).Append(jsonBuffer), nil)
	
	return list, nil
}

// marshalIndent writes a  JSON string from arbitrary data.
func marshalIndent(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	prefix := data.String(args.Get(1))
	indent := data.String(args.Get(2))

	jsonBuffer, err := json.MarshalIndent(data.Sanitize(args.Get(0)), prefix, indent)
	if err != nil {
		err = errors.New(err).In("MarshalIndent")
	}

	list := data.NewList(data.NewArray(data.ByteType, 0).Append(jsonBuffer), err)

	return list, err
}
