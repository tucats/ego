package formats

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/i18n"
)

// PackageAsString formats a map for printing as a table. The result is
// a string suitable for directing to the console.
func PackageAsString(vv *data.Package) string {
	t, _ := tables.New([]string{i18n.L("Member"), i18n.L("Value")})
	t.SetPagination(999, -1)

	keys := vv.Keys()
	for _, key := range keys {
		keyString := data.String(key)
		if strings.HasPrefix(keyString, "__") {
			continue
		}

		value, _ := vv.Get(keyString)
		valueString := data.Format(value)

		_ = t.AddRow([]string{keyString, valueString})
	}

	r, _ := t.String("text")

	return strings.TrimPrefix(strings.TrimSuffix(r, "\n\n"), "\n")
}
