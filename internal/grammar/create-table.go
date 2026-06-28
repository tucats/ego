package grammar

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

var CreateTableGrammar = []cli.Option{
	{
		LongName:    defs.DSNOption,
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
