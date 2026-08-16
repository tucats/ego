package class

import (
	"github.com/tucats/ego/internal/cli/app"
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/grammar/common"
)

// ServerGrammar contains the grammar of SERVER subcommands.
var ServerGrammar = []cli.Option{
	{
		LongName:      "validation",
		Aliases:       []string{"validate", "validations", "valid"},
		Description:   "ego.server.validation",
		OptionType:    cli.Subcommand,
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
		Action: commands.ServerValidations,
	},
	{
		LongName:      "logging",
		Aliases:       []string{"logger", "log", "logs"},
		Description:   "ego.server.logging",
		OptionType:    cli.Subcommand,
		Value:         LoggingGrammar,
		ExpectedParms: cli.Variable,
		Action:        commands.Logging,
	},
	{
		LongName:    "logon",
		Aliases:     []string{"login"},
		OptionType:  cli.Subcommand,
		Description: "ego.server.logon",
		Action:      app.Logon,
		Value:       app.LogonGrammar,
	},
	{
		LongName:    "users",
		Aliases:     []string{"user"},
		Description: "ego.server.users",
		OptionType:  cli.Subcommand,
		Value:       UserGrammar,
	},
	{
		LongName:    "memory",
		Description: "ego.server.memory",
		OptionType:  cli.Subcommand,
		Action:      commands.ServerMemory,
		Value:       common.ServerMemoryGrammar,
	},
	{
		LongName:    "caches",
		Aliases:     []string{"cache"},
		Description: "ego.server.caches",
		OptionType:  cli.Subcommand,
		Value:       CachesGrammar,
	},
	{
		LongName:    "run",
		Description: "ego.server.run",
		OptionType:  cli.Subcommand,
		Action:      commands.RunServer,
		// Run and Start share a grammar, but Run has additional options
		Value: append(common.ServerRunGrammar, []cli.Option{
			{
				LongName:    "debug-endpoint",
				ShortName:   "d",
				Description: "server.run.debug",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "new-token",
				Description: "new.token",
				OptionType:  cli.BooleanType,
			},
		}...),
	},
	{
		LongName:    "restart",
		Description: "ego.server.restart",
		OptionType:  cli.Subcommand,
		Action:      commands.Restart,
		Unsupported: []string{"windows"},
		Value: append(common.ServerStateGrammar, []cli.Option{
			{
				LongName:    "force",
				Description: "server.stop.force",
				ShortName:   "f",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "new-token",
				Description: "new.token",
				OptionType:  cli.BooleanType,
			},
		}...),
	},
	{
		LongName:      "status",
		Description:   "ego.server.status",
		OptionType:    cli.Subcommand,
		Action:        commands.Status,
		ExpectedParms: cli.Variable,
		ParmDesc:      "address.port",
		Value:         common.ServerStateGrammar,
		DefaultVerb:   true,
	},
	{
		LongName:    "start",
		Description: "ego.server.start",
		OptionType:  cli.Subcommand,
		Action:      commands.Start,
		Value: append(common.ServerRunGrammar, []cli.Option{
			{
				LongName:    "new-token",
				Description: "new.token",
				OptionType:  cli.BooleanType,
			},
		}...),
		Unsupported: []string{"windows"},
	},
	{
		LongName:    "stop",
		Description: "ego.server.stop",
		OptionType:  cli.Subcommand,
		Action:      commands.Stop,
		Value:       ServerStopGrammar,
		Unsupported: []string{"windows"},
	},
	{
		LongName:    "cluster",
		Description: "ego.server.cluster",
		OptionType:  cli.Subcommand,
		Value:       common.ClusterSubVerbGrammar,
	},
}

// ServerStopGrammar handles command line options for the server subcommand.
var ServerStopGrammar = []cli.Option{
	{
		LongName:    "force",
		Description: "server.stop.force",
		ShortName:   "f",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "grace",
		Description: "server.stop.grace",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "port",
		EnvVar:      defs.EgoPortEnv,
	},
}
