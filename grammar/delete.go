package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var DeleteVerbGrammar = []cli.Option{
	{
		LongName:      "config",
		Aliases:       []string{"conf"},
		Description:   "ego.verb.delete.config",
		OptionType:    cli.Subcommand,
		Action:        config.DeleteAction,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm-key [parm-key...]",
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.item")},
	},
	{
		LongName:      defs.DSNOption,
		Aliases:       []string{"dsns"},
		Description:   "ego.verb.delete.dsn",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSDelete,
		ParmDesc:      "dsn-name[ ds-name...]",
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.dsn")},
		Value:         DeleteDSNSGrammar,
	},
	{
		LongName:      "profile",
		Description:   "ego.verb.delete.profile",
		OptionType:    cli.Subcommand,
		Action:        config.DeleteProfileAction,
		ExpectedParms: 1,
		ParmDesc:      "profile-name",
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
				LongName:    defs.DSNOption,
				ShortName:   "d",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "token",
		Description:   "ego.verb.delete.token",
		OptionType:    cli.Subcommand,
		ParmDesc:      "token-id [token-id...]",
		ExpectedParms: -99,
		MinParams:     1,
		Action:        commands.TokenDelete,
		Prompts:       []string{i18n.L("prompt.token.id")},
	},
	{
		LongName:      "rows",
		Aliases:       []string{"rows"},
		Description:   "ego.verb.delete.rows",
		OptionType:    cli.Subcommand,
		Action:        commands.TableDelete,
		ExpectedParms: 1,
		ParmDesc:      "table-name",
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
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
