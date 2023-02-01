// Package formats has formatting operations for the built-in
// nonscalar types in Ego. This includes maps, arrays, and structs.
package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/i18n"
)

// MapAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func MapAsString(vv *data.Map, showTypes bool) string {
	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Type"), i18n.L("Value")})

	keys := vv.Keys()
	for _, key := range keys {
		keyString := data.String(key)
		value, _, _ := vv.Get(keyString)
		valueString := data.String(value)
		typeString := data.TypeOf(value).TypeString()

		_ = t.AddRow([]string{keyString, typeString, valueString})
	}

	if !showTypes {
		_ = t.SetColumnOrderByName([]string{i18n.L("Key"), i18n.L("Type"), i18n.L("Value")})
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
