package verb

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

var SetDSNGrammar = []cli.Option{
	{
		LongName:    "database",
		ShortName:   "d",
		Aliases:     []string{"db"},
		Description: "dsns.add.database",
		OptionType:  cli.StringType,
		Prompts:     []string{"database.name"},
	},
	{
		LongName:    "host",
		Description: "dsns.add.host",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "port",
		Description: "dsns.add.port",
		OptionType:  cli.IntType,
	},
	{
		LongName:    defs.UsernameOption,
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.add.username",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "schema",
		Aliases:     []string{"user"},
		Description: "dsns.add.schema",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "secured",
		Aliases:     []string{"secure"},
		Description: "dsns.add.secured",
		OptionType:  cli.BooleanValueType,
	},
	{
		LongName:    "restricted",
		Description: "dsns.add.restricted",
		OptionType:  cli.BooleanValueType,
	},
	{
		LongName:    "row-id",
		ShortName:   "i",
		Aliases:     []string{"rowid", "id"},
		OptionType:  cli.BooleanValueType,
		Description: "dsns.add.rowid",
	},
	{
		LongName:    defs.PasswordOption,
		Aliases:     []string{"pw"},
		ShortName:   "p",
		Description: "dsns.update.password",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "force",
		Description: "dsns.update.force",
		OptionType:  cli.BooleanType,
	},
}
