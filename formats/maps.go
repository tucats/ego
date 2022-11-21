package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/i18n"
)

// MapAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func MapAsString(vv *datatypes.EgoMap) string {
	t, _ := tables.New([]string{i18n.T("label.Key"), i18n.T("label.Type"), i18n.T("label.Value")})

	keys := vv.Keys()
	for _, key := range keys {
		keyString := datatypes.GetString(key)
		value, _, _ := vv.Get(keyString)
		valueString := datatypes.GetString(value)
		typeString := datatypes.TypeOf(value).TypeString()

		_ = t.AddRow([]string{keyString, typeString, valueString})
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
