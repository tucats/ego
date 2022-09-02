package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/datatypes"
)

// StructAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func StructAsString(vv *datatypes.EgoStruct) string {
	t, _ := tables.New([]string{"Field", "Type", "Value"})
	//t.ShowUnderlines(false).ShowHeadings(false)

	keys := vv.FieldNames()
	for _, key := range keys {
		keyString := datatypes.GetString(key)
		value, _ := vv.Get(keyString)
		valueString := datatypes.GetString(value)
		typeString := datatypes.TypeOf(value).TypeString()

		_ = t.AddRowItems(keyString, typeString, valueString)
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
