package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var DeleteVerbGrammar = []cli.Option{
	{
		LongName:      "dsn",
		Aliases:       []string{"dsns"},
		Description:   "ego.verb.delete.dsn",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSDelete,
		ParmDesc:      "dsn-name[ ds-name...]",
		ExpectedParms: defs.VariableParameterCount,
		Value:         DeleteDSNSGrammar,
	},
	{
		LongName:      "table",
		Aliases:       []string{"tables"},
		Description:   "ego.verb.delete.table",
		OptionType:    cli.Subcommand,
		Action:        commands.TableDrop,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "table-name [table-name...]",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				ShortName:   "d",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "rows",
		Aliases:       []string{"rows"},
		Description:   "ego.verb.delete.rows",
		OptionType:    cli.Subcommand,
		Action:        commands.TableDelete,
		ExpectedParms: 1,
		Value:         DeleteRowsGrammar,
	},
	{
		LongName:      "user",
		Description:   "ego.server.user.delete",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.DeleteUser,
		Value:         ServerDeleteUserGrammar,
	},
}
