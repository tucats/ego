package grammar

import "github.com/tucats/ego/app-cli/cli"

var CreateTableGrammar = []cli.Option{
	{
		LongName:    "dsn",
		ShortName:   "d",
		Aliases:     []string{"ds", "datasource"},
		Description: "dsn",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "file",
		Aliases:     []string{"json-file", "json"},
		ShortName:   "f",
		Description: "table.create.file",
		OptionType:  cli.StringListType,
	},
}
