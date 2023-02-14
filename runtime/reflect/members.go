package reflect

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// members gets an array of the names of the fields in a structure.
func members(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		keyList := v.Keys()
		keys := data.NewArray(data.StringType, len(keyList))

		for i, v := range keyList {
			_ = keys.Set(i, data.String(v))
		}

		_ = keys.Sort()

		return keys, nil

	case *data.Struct:
		return v.FieldNamesArray(false), nil

	case *data.Package:
		keys := data.NewArray(data.StringType, 0)

		for _, k := range v.Keys() {
			if !strings.HasPrefix(k, data.MetadataPrefix) {
				keys.Append(data.String(k))
			}
		}

		err := keys.Sort()

		return keys, err

	default:
		return nil, errors.ErrInvalidType.In("Members")
	}
}
