package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var ShowVerbLogGrammar = []cli.Option{
	{
		LongName:    "entries",
		Aliases:     []string{"lines", "contents", "messages"},
		Description: "ego.verb.show.server.log.entries",
		OptionType:  cli.Subcommand,
		Action:      commands.Logging,
		DefaultVerb: true,
		Value:       ShowLogEntriesGrammar,
	},
	{
		LongName:    "file",
		Description: "ego.verb.show.server.log.file",
		OptionType:  cli.Subcommand,
		Action:      commands.LoggingFile,
	},
	{
		LongName:    "status",
		Description: "ego.verb.show.server.log.status",
		OptionType:  cli.Subcommand,
		Action:      commands.LoggingStatus,
	},
}

var ShowLogEntriesGrammar = []cli.Option{
	{
		LongName:    "limit",
		ShortName:   "l",
		Description: "limit",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "session",
		Description: "server.logging.session",
		OptionType:  cli.IntType,
	},
	{
		LongName:   "as-text",
		ShortName:  "t",
		OptionType: cli.BooleanType,
		Private:    true,
	},
}

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
		Value:       ShowVerbLogGrammar,
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
		LongName:      "permission",
		Aliases:       []string{"perm"},
		Description:   "ego.verb.show.table.permission",
		OptionType:    cli.Subcommand,
		Action:        commands.TableShowPermission,
		ExpectedParms: 1,
		MinParams:     1,
		ParmDesc:      "parm.table.name",
		Prompts:       []string{i18n.L("prompt.table")},
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
				LongName:    defs.DSNOption,
				Aliases:     []string{"ds", "datasource", "data-source", "data-source-name"},
				ShortName:   "d",
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
}

var ShowVerbGrammar = []cli.Option{
	{
		LongName:      "config",
		Aliases:       []string{"conf", "configuration", "settings"},
		Description:   "ego.verb.show.config",
		OptionType:    cli.Subcommand,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "[param [param...]]",
		Action:        config.ShowAction,
	},
	{
		LongName:      defs.DSNOption,
		Aliases:       []string{"ds", "datasource", "data-source", "data-source-name"},
		Description:   "ego.verb.show.dsn",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNShow,
		ExpectedParms: 1,
		MinParams:     1,
		ParmDesc:      "<dsn-name>",
	},
	{
		LongName:    "log",
		Aliases:     []string{"logging"},
		Description: "ego.verb.show.server.log",
		OptionType:  cli.Subcommand,
		Action:      commands.Logging,
		Value:       ShowVerbLogGrammar,
		Private:     true,
	},
	{
		LongName:    "path",
		OptionType:  cli.Subcommand,
		Action:      commands.PathAction,
		Description: "ego.verb.path",
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
