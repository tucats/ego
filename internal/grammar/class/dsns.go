package class

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
)

// DSNSGrammar specifies the command line options for the "dsns" Ego command.
var DSNSGrammar = []cli.Option{
	{
		LongName:      "show",
		Description:   "ego.dsns.show",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNShow,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "metadata",
				ShortName:   "m",
				Aliases:     []string{"schema"},
				Description: "dsns.show.metadata",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
		},
	},
	{
		LongName:      "delete",
		Description:   "ego.dsns.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSDelete,
		ParmDesc:      "dsn-name[ ds-name...]",
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
	},
	{
		LongName:      "add",
		Description:   "ego.dsns.add",
		Aliases:       []string{"create"},
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSAdd,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "type",
				ShortName:   "t",
				Aliases:     []string{"provider"},
				Description: "dsns.add.type",
				OptionType:  cli.KeywordType,
				Keywords:    []string{"sqlite", defs.PostgresProvider},
				Required:    true,
				Prompts:     []string{"database.type"},
			},
			{
				LongName:    "database",
				ShortName:   "d",
				Aliases:     []string{"db"},
				Description: "dsns.add.database",
				OptionType:  cli.StringType,
				Required:    true,
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
				LongName:    defs.PasswordOption,
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
				LongName:    "restricted",
				Description: "dsns.add.restricted",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-id",
				ShortName:   "i",
				Aliases:     []string{"rowid", "id"},
				OptionType:  cli.BooleanValueType,
				Description: "dsns.add.rowid",
			},
		},
	},
	{
		LongName:      defs.GrantOption,
		Description:   "ego.dsns.grant",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSGrant,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    defs.UsernameOption,
				Aliases:     []string{"user"},
				ShortName:   "u",
				Description: "dsns.grant.username",
				OptionType:  cli.StringType,
				Required:    true,
				Prompts:     []string{"user.name"},
			},
			{
				LongName:    "permissions",
				Aliases:     []string{"perms"},
				ShortName:   "p",
				Description: "dsns.grant.permissions",
				OptionType:  cli.StringListType,
				Keywords:    []string{defs.DSNReadPermission, defs.DSNWritePermission, defs.DSNAdminPermission},
				Required:    true,
				Prompts:     []string{"user.permissions"},
			},
		},
	},
	{
		LongName:      defs.RevokeOption,
		Description:   "ego.dsns.revoke",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSRevoke,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    defs.UsernameOption,
				Aliases:     []string{"user"},
				ShortName:   "u",
				Description: "dsns.revoke.username",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "permissions",
				Aliases:     []string{"perms"},
				ShortName:   "p",
				Description: "dsns.revoke.permissions",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:      "update",
		Description:   "ego.dsns.update",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSUpdate,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    defs.PasswordOption,
				Aliases:     []string{"pw"},
				ShortName:   "p",
				Description: "dsns.update.password",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "secured",
				Aliases:     []string{"secure"},
				Description: "dsns.update.secured",
				OptionType:  cli.BooleanValueType,
			},
			{
				LongName:    "restricted",
				Description: "dsns.update.restricted",
				OptionType:  cli.BooleanValueType,
			},
			{
				LongName:    "force",
				Description: "dsns.update.force",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:    "list",
		Description: "ego.dsns.list",
		OptionType:  cli.Subcommand,
		Action:      commands.DSNSList,
		DefaultVerb: true,
		Value: []cli.Option{
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
		},
	},
}
