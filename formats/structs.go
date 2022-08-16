package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/datatypes"
)

// MapAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func StructAsString(vv *datatypes.EgoStruct) string {
	t, _ := tables.New([]string{"Field", "Value"})
	//t.ShowUnderlines(false).ShowHeadings(false)

	keys := vv.FieldNames()
	for _, key := range keys {
		keyString := datatypes.GetString(key)
		value, _ := vv.Get(keyString)
		valueString := datatypes.GetString(value)

		t.AddRowItems(keyString, valueString)
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
