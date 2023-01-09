package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/i18n"
)

// StructAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func StructAsString(vv *datatypes.EgoStruct) string {
	t, _ := tables.New([]string{i18n.L("Field"), i18n.L("Type"), i18n.L("Value")})

	keys := vv.FieldNames()
	for _, key := range keys {
		keyString := datatypes.String(key)
		value, _ := vv.Get(keyString)
		valueString := datatypes.String(value)
		typeString := datatypes.TypeOf(value).String()

		_ = t.AddRowItems(keyString, typeString, valueString)
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
