package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/i18n"
)

var RevokeVerbGrammar = []cli.Option{
	{
		LongName:      "token",
		Description:   "ego.verb.revoke.token",
		OptionType:    cli.Subcommand,
		ParmDesc:      "id",
		ExpectedParms: -99,
		MinParams:     1,
		Action:        commands.TokenRevoke,
		Prompts:       []string{i18n.L("prompt.token.id")},
	},
	{
		LongName:      "dsn",
		Aliases:       []string{"ds", "datasource", "data-source", "data-source-name"},
		Description:   "ego.verb.revoke.dsn",
		OptionType:    cli.Subcommand,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		MinParams:     1,
		Action:        commands.DSNSRevoke,
		Prompts:       []string{i18n.L("prompt.dsn")},
		Value:         GrantObjectGrammar,
	},
	{
		LongName:      "table",
		Description:   "ego.verb.revoke.table",
		OptionType:    cli.Subcommand,
		ParmDesc:      "parm.table.name",
		Action:        commands.TableRevoke,
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		Value:         GrantObjectGrammar,
	},
	{
		LongName:      "user",
		Description:   "ego.verb.revoke.user",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.user")},
		Action:        commands.RevokeUser,
		Value: []cli.Option{
			{
				LongName:    "permissions",
				Aliases:     []string{"permission"},
				Description: "server.user.perms",
				OptionType:  cli.StringListType,
			},
		},
	},
}

var GrantObjectGrammar = []cli.Option{
	{
		LongName:    "username",
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.revoke.username",
		OptionType:  cli.StringType,
		Required:    true,
	},
	{
		LongName:    "permissions",
		Aliases:     []string{"perms"},
		ShortName:   "p",
		Description: "dsns.revoke.permissions",
		OptionType:  cli.StringListType,
		Keywords:    []string{"read", "write", "admin"},
		Required:    true,
	},
}
