package grammar

import "github.com/tucats/ego/app-cli/cli"

var DeleteRowsGrammar = []cli.Option{
	{
		LongName:    "dsn",
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
