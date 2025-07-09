package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var ShowVerbServerGrammar = []cli.Option{
	{
		LongName:    "cache",
		Description: "ego.verb.show.server.cache",
		OptionType:  cli.Subcommand,
		Action:      commands.ShowCaches,
	},
	{
		LongName:    "log",
		Aliases:     []string{"logging"},
		Description: "ego.verb.show.server.log",
		OptionType:  cli.Subcommand,
		Action:      commands.Logging,
		Value:       LoggingGrammar,
	},
	{
		LongName:    "memory",
		Description: "ego.verb.show.server.memory",
		OptionType:  cli.Subcommand,
		Action:      commands.ServerMemory,
		Value:       ServerMemoryGrammar,
	},
	{
		LongName:      "status",
		Description:   "ego.verb.show.server.status",
		OptionType:    cli.Subcommand,
		Action:        commands.Status,
		ExpectedParms: -1,
		ParmDesc:      "server",
		DefaultVerb:   true,
		Value:         ServerStateGrammar,
	},
	{
		LongName:      "validations",
		Description:   "ego.verb.show.server.validations",
		OptionType:    cli.Subcommand,
		Action:        commands.ServerValidations,
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     0,
		ParmDesc:      "item",
		Value: []cli.Option{
			{
				LongName:    "all",
				ShortName:   "a",
				Description: "server.validation.all",
				OptionType:  cli.BooleanType,
				Excludes:    []string{"entry", "path", "method"},
			},
			{
				LongName:    "entry",
				ShortName:   "e",
				Description: "server.validation.entry",
				OptionType:  cli.BooleanType,
				Excludes:    []string{"path", "method", "all"},
			},
			{
				LongName:    "path",
				ShortName:   "p",
				Description: "server.validation.path",
				OptionType:  cli.BooleanType,
				Excludes:    []string{"entry", "all"},
			},
			{
				LongName:    "method",
				ShortName:   "m",
				Description: "server.validation.method",
				OptionType:  cli.StringType,
				Keywords:    []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
				Excludes:    []string{"entry", "all"},
			},
		},
	},
}

var TableVerbGrammar = []cli.Option{
	{
		LongName:      "permissions",
		Aliases:       []string{"perms"},
		Description:   "ego.verb.show.table.permissions",
		OptionType:    cli.Subcommand,
		Action:        commands.TablePermissions,
		ExpectedParms: 0,
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "table.permissions.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "columns",
		Aliases:       []string{"metadata", "schema"},
		Description:   "ego.verb.show.table.columns",
		OptionType:    cli.Subcommand,
		Action:        commands.TableShow,
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		ParmDesc:      "parm.table.name",
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
}

var ShowVerbGrammar = []cli.Option{
	{
		LongName:    "config",
		Aliases:     []string{"conf"},
		Description: "ego.verb.show.config",
		OptionType:  cli.Subcommand,
		Action:      config.ShowAction,
	},
	{
		LongName:    "server",
		Description: "ego.verb.show.server",
		OptionType:  cli.Subcommand,
		Value:       ShowVerbServerGrammar,
	},

	{
		LongName:      "user",
		Description:   "ego.verb.show.user",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.user")},
		Action:        commands.ShowUser,
		Value:         ServerShowUserGrammar,
	},
	{
		LongName:    "table",
		Description: "ego.verb.show.table",
		OptionType:  cli.Subcommand,
		Value:       TableVerbGrammar,
	},
}
