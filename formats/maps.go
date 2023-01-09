package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/i18n"
)

// MapAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func MapAsString(vv *data.EgoMap) string {
	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Type"), i18n.L("Value")})

	keys := vv.Keys()
	for _, key := range keys {
		keyString := data.String(key)
		value, _, _ := vv.Get(keyString)
		valueString := data.String(value)
		typeString := data.TypeOf(value).TypeString()

		_ = t.AddRow([]string{keyString, typeString, valueString})
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
