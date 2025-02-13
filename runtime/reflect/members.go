package reflect

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// members gets an array of the names of the fields in a structure.
func members(syms *symbols.SymbolTable, args data.List) (interface{}, error) {
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

			// If not exporited, ignore
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

		return keys, err

	default:
		return nil, errors.ErrInvalidType.In("Members")
	}
}
