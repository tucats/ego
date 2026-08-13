package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
)

// ClusterVerbGrammar is the verb-object grammar for cluster commands. The
// "server" sub-entry delegates further to ClusterSubVerbGrammar for the
// traditional verb-object form (ego cluster server start/show/stop/remove).
// The "start" sub-entry handles the shorthand "ego cluster start" directly.
var ClusterVerbGrammar = []cli.Option{
	{
		LongName:      "start",
		Description:   "ego.cluster.start",
		Action:        commands.ClusterStart,
		ParmDesc:      "parm.cluster.name",
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
		Prompts:       []string{"prompt.cluster.name"},
		OptionType:    cli.Subcommand,
		Unsupported:   []string{"windows"},
		Value:         ClusterStartGrammar,
	},
	{
		LongName:    "server",
		Description: "ego.cluster",
		Action:      commands.ClusterShow,
		OptionType:  cli.Subcommand,
		Value:       ClusterSubVerbGrammar,
	},
}

// ClusterSubVerbGrammar defines the sub-verbs available under "ego server cluster"
// (traditional) or "ego cluster server" (verb-object). Sub-verbs: start, show, stop, remove.
var ClusterSubVerbGrammar = []cli.Option{
	{
		LongName:      "start",
		Description:   "ego.cluster.start",
		OptionType:    cli.Subcommand,
		Action:        commands.ClusterStart,
		ParmDesc:      "parm.cluster.name",
		ExpectedParms: 1,
		Prompts:       []string{"prompt.cluster.name"},
		Unsupported:   []string{"windows"},
		Value:         ClusterStartGrammar,
	},
	{
		LongName:    "show",
		Description: "ego.cluster.show",
		OptionType:  cli.Subcommand,
		Action:      commands.ClusterShow,
	},
	{
		LongName:    "stop",
		Description: "ego.cluster.stop",
		OptionType:  cli.Subcommand,
		Action:      commands.ClusterStopNode,
		Value: append(ClusterNodeGrammar, []cli.Option{
			{
				LongName:    "all",
				ShortName:   "a",
				Description: "opt.cluster.all",
				OptionType:  cli.BooleanType,
			},
		}...),
	},
	{
		LongName:    "remove",
		Description: "ego.cluster.remove",
		OptionType:  cli.Subcommand,
		Action:      commands.ClusterRemoveNode,
		Value:       ClusterNodeGrammar,
	},
}

// ClusterNodeGrammar holds the options shared by cluster stop and cluster remove.
var ClusterNodeGrammar = []cli.Option{
	{
		LongName:    "node",
		ShortName:   "n",
		Description: "opt.cluster.node",
		OptionType:  cli.StringType,
	},
}

// ClusterStartGrammar defines the options for "ego start cluster" and
// "ego server cluster start". It is similar to ServerRunGrammar but uses
// --ports (a comma-separated StringList) instead of --port (a single int),
// and omits internal options like --is-detached and --session-uuid that are
// not meaningful when starting multiple nodes from a single command.
var ClusterStartGrammar = []cli.Option{
	{
		LongName:    "ports",
		Aliases:     []string{"port"},
		ShortName:   "p",
		OptionType:  cli.RangeType,
		Description: "cluster.start.ports",
	},
	{
		LongName:    "not-secure",
		ShortName:   "k",
		OptionType:  cli.BooleanType,
		Description: "server.run.not.secure",
		EnvVar:      defs.EgoInsecureEnv,
	},
	{
		LongName:    "cert-dir",
		Aliases:     []string{"certs"},
		Description: "server.run.certs",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "keep-logs",
		Description: "server.run.keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "auth-server",
		Aliases:     []string{"auth"},
		Description: "server.auth.server",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "users",
		Aliases:     []string{"user-database"},
		ShortName:   "u",
		Description: "server.run.users",
		OptionType:  cli.StringType,
		EnvVar:      defs.EgoUsersEnv,
	},
	{
		LongName:    "superuser",
		Description: "server.run.superuser",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "cache-size",
		Description: "server.run.cache",
		OptionType:  cli.IntType,
	},
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "server.run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      defs.EgoTypesEnv,
	},
	{
		LongName:    "realm",
		ShortName:   "r",
		Description: "server.run.realm",
		OptionType:  cli.StringType,
		EnvVar:      defs.EgoRealmEnv,
	},
	{
		LongName:    "sandbox-path",
		Description: "server.run.sandbox",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "child-services",
		Description: "server.run.child.services",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "no-log",
		Description: "server.run.no.log",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "oauth-server",
		Description: "server.run.oauth.server",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.VerboseOption,
		ShortName:   "v",
		OptionType:  cli.BooleanType,
		Description: "verbose",
	},
}
