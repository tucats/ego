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
	var (
		t             *tables.Table
		heterogeneous = true
	)

	keys := vv.Keys()

	// Is the value type heterogeneous? If the value type is interface, scan
	// the values to see if they are all actually the same value.
	if vv.ElementType() == data.InterfaceType {
		expectedTypeString := ""

		for _, k := range keys {
			v, _, _ := vv.Get(k)
			valueTypeString := data.TypeOf(v).TypeString()

			if expectedTypeString == "" {
				expectedTypeString = valueTypeString
			} else if expectedTypeString != valueTypeString {
				heterogeneous = false

				break
			}
		}
	}

	// If we are directed to show the types regardless of the heterogeneity, we
	// override it now.
	if showTypes {
		heterogeneous = false
	}

	// If the types are heterogeneous, we only need two columns for the key and value.
	// Otherwise, we need three columns for the key, type, and value.
	if heterogeneous {
		t, _ = tables.New([]string{i18n.L("Key"), i18n.L("Value")})
	} else {
		t, _ = tables.New([]string{i18n.L("Key"), i18n.L("Type"), i18n.L("Value")})
	}

	// Scan back over the map using the keys to get the key, type, and value. Depending
	// on whether we are in heterogeneous mode or not, we may only need to add the key
	// value value.
	for _, key := range keys {
		keyString := data.String(key)
		value, _, _ := vv.Get(keyString)
		valueString := data.String(value)

		if heterogeneous {
			_ = t.AddRow([]string{keyString, valueString})
		} else {
			_ = t.AddRow([]string{keyString, data.TypeOf(value).TypeString(), valueString})
		}
	}

	// Set the sort order for the columns, and then format the table.
	if heterogeneous {
		_ = t.SetColumnOrderByName([]string{i18n.L("Key"), i18n.L("Value")})
	} else {
		_ = t.SetColumnOrderByName([]string{i18n.L("Key"), i18n.L("Type"), i18n.L("Value")})
	}

	r, _ := t.String("text")

	// Return the text of the table, with line breaks un-escaped.
	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
