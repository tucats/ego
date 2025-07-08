package grammar

import "github.com/tucats/ego/app-cli/cli"

var CreateDSNGrammar = []cli.Option{
	{
		LongName:    "type",
		ShortName:   "t",
		Aliases:     []string{"provider"},
		Description: "dsns.add.type",
		OptionType:  cli.KeywordType,
		Keywords:    []string{"sqlite3", "postgres"},
		Required:    true,
	},
	{
		LongName:    "database",
		ShortName:   "d",
		Aliases:     []string{"db"},
		Description: "dsns.add.database",
		OptionType:  cli.StringType,
		Required:    true,
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
		LongName:    "username",
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.add.username",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "password",
		Aliases:     []string{"pw"},
		ShortName:   "p",
		Description: "dsns.add.password",
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
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "native",
		Description: "dsns.add.native",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "row-id",
		ShortName:   "i",
		Aliases:     []string{"rowid", "id"},
		OptionType:  cli.BooleanValueType,
		Description: "dsns.add.rowid",
	},
}
