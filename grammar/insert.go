package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var InsertVerbGrammar = []cli.Option{
	{
		LongName:      "row",
		Aliases:       []string{"rows"},
		Description:   "ego.table.insert",
		OptionType:    cli.Subcommand,
		Action:        commands.TableInsert,
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
		ParmDesc:      "parm.table.insert",
		DefaultVerb:   true,
		Value: []cli.Option{
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
				Description: "table.insert.file",
				OptionType:  cli.StringListType,
			},
		},
	},
}
