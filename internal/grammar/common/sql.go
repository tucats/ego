package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

// SQLGrammar specifies the command line options for the "sql" Ego command.
var SQLGrammar = []cli.Option{
	{
		LongName:    defs.DSNOption,
		ShortName:   "d",
		Aliases:     []string{"ds", "datasource"},
		Description: "dsn",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "sql-file",
		ShortName:   "f",
		Aliases:     []string{"file"},
		Description: "ego.sql.file",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "row-ids",
		ShortName:   "i",
		Aliases:     []string{"ids"},
		Excludes:    []string{"generate"},
		Description: "ego.sql.row-ids",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "row-numbers",
		ShortName:   "n",
		Excludes:    []string{"generate"},
		Aliases:     []string{"row-number", "row"},
		Description: "ego.sql.row-numbers",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "generate",
		ShortName:   "g",
		Excludes:    []string{"row-numbers", "row-ids"},
		Description: "ego.sql.generate",
		OptionType:  cli.BooleanType,
	},
}
