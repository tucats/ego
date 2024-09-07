package formats

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/i18n"
)

// StructAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console. If the showTypes
// parameter is true, the type of each value is shown in the table.
func StructAsString(vv *data.Struct, showTypes bool) string {
	var (
		t   *tables.Table
		err error
	)

	// Is this a native object? If so, we just return tye type string.
	if nativeName := vv.Type().NativeName(); nativeName != "" {
		return fmt.Sprintf("%s %s", nativeName, vv.FormatNative())
	}

	// Create the table with two or three columns depending on whether
	// we are showing types or not.
	if showTypes {
		t, err = tables.New([]string{i18n.L("Field"), i18n.L("Type"), i18n.L("Value")})
		if err != nil {
			return fmt.Sprintf("Error creating table: %s", err)
		}
	} else {
		t, err = tables.New([]string{i18n.L("Field"), i18n.L("Value")})
		if err != nil {
			return fmt.Sprintf("Error creating table: %s", err)
		}
	}

	// Scan over the struct using the field names to get the field name,
	// and add the field name, optional type, and value to the table.
	keys := vv.FieldNames(false)
	for _, key := range keys {
		keyString := data.String(key)
		value, _ := vv.Get(keyString)
		valueString := data.String(value)
		typeString := data.TypeOf(value).String()

		if showTypes {
			err = t.AddRowItems(keyString, typeString, valueString)
		} else {
			err = t.AddRowItems(keyString, valueString)
		}

		if err != nil {
			return fmt.Sprintf("Error adding row to table: %s", err)
		}
	}

	r, err := t.String("text")
	if err != nil {
		return fmt.Sprintf("Error formatting table: %s", err)
	}

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
