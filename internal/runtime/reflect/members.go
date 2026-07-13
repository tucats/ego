package reflect

import (
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// members gets an array of the names of the fields in a structure.
func members(syms *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		keyList := v.Keys()
		keys := data.NewArray(data.StringType, len(keyList))

		for i, v := range keyList {
			_ = keys.Set(i, data.String(v))
		}

		_ = keys.Sort()

		return data.NewList(keys, nil), nil

	case *data.Struct:
		return data.NewList(v.FieldNamesArray(false), nil), nil

	case *data.Package:
		keys := data.NewArray(data.StringType, 0)

		for _, k := range v.Keys() {
			if !strings.HasPrefix(k, data.MetadataPrefix) {
				keys.Append(k)
			}
		}

		// Also need to collect any exported symbols from the package.
		s := symbols.GetPackageSymbolTable(v)
		for _, k := range s.Names() {
			// If invisible, ignore
			if strings.HasPrefix(k, defs.InvisiblePrefix) {
				continue
			}

			// If not exported, ignore
			if !egostrings.HasCapitalizedName(k) {
				continue
			}

			// If already in the array, ignore
			if _, found := v.Get(k); found {
				continue
			}

			keys.Append(k)
		}

		err := keys.Sort()

		return data.NewList(keys, err), err

	default:
		err := errors.ErrInvalidType.In("Members")

		return data.NewList(nil, err), err
	}
}
