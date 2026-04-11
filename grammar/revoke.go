package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
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
		LongName:      defs.DSNOption,
		Aliases:       []string{"ds", "datasource", "data-source", "data-source-name"},
		Description:   "ego.verb.revoke.dsn",
		OptionType:    cli.Subcommand,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		MinParams:     1,
		Action:        commands.DSNSRevoke,
		Prompts:       []string{i18n.L("prompt.dsn")},
		Value:         GrantDSNGrammar,
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
		Value:         GrantTableGrammar,
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

var GrantDSNGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.revoke.username",
		OptionType:  cli.StringType,
		Prompts:     []string{"user.name"},
	},
	{
		LongName:    "permissions",
		Aliases:     []string{"perms"},
		ShortName:   "p",
		Description: "dsns.revoke.permissions",
		OptionType:  cli.StringListType,
		Keywords:    []string{defs.DSNReadPermission, defs.DSNWritePermission, defs.DSNAdminPermission},
		Required:    true,
		Prompts:     []string{"user.permissions"},
	},
}

var GrantTableGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.revoke.username",
		OptionType:  cli.StringType,
		Prompts:     []string{"user.name"},
	},
	{
		LongName:    "permissions",
		Aliases:     []string{"perms"},
		ShortName:   "p",
		Description: "dsns.revoke.permissions",
		OptionType:  cli.StringListType,
		Keywords: []string{
			defs.TableReadPermission,
			defs.TableWritePermission,
			defs.TableUpdatePermission,
			defs.TableDeletePermission,
			defs.TableAdminPermission},
		Required: true,
		Prompts:  []string{"user.permissions"},
	},
}
