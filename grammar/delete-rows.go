package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/defs"
)

var DeleteRowsGrammar = []cli.Option{
	{
		LongName:    defs.DSNOption,
		ShortName:   "d",
		Aliases:     []string{"ds", "datasource"},
		Description: "dsn",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "filter",
		ShortName:   "f",
		Aliases:     []string{"where"},
		Description: "table.delete.filter",
		OptionType:  cli.StringListType,
	},
}
