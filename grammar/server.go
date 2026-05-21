package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

// ClusterNodeGrammar holds the options shared by cluster stop and cluster remove.
var ClusterNodeGrammar = []cli.Option{
	{
		LongName:    "node",
		ShortName:   "n",
		Description: "opt.cluster.node",
		OptionType:  cli.StringType,
	},
}

// ClusterSubVerbGrammar defines the sub-verbs available under "ego server cluster"
// (traditional) or "ego cluster server" (verb-object). Sub-verbs: show, stop, remove.
var ClusterSubVerbGrammar = []cli.Option{
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

// ClusterVerbGrammar is the verb-object grammar for cluster commands. The object
// is always "server", matching the pattern used by start/stop/restart.
// The default action (when no sub-verb is given) shows cluster status.
var ClusterVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.cluster",
		Action:      commands.ClusterShow,
		OptionType:  cli.Subcommand,
		Value:       ClusterSubVerbGrammar,
	},
}

var StartVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.server.start",
		Action:      commands.Start,
		OptionType:  cli.Subcommand,
		//DefaultVerb: true,
		Unsupported: []string{"windows"},
		Value:       ServerRunGrammar,
	},
}

var StopVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.server.stop",
		Action:      commands.Stop,
		OptionType:  cli.Subcommand,
		//DefaultVerb: true,
		Unsupported: []string{"windows"},
		Value: append(ServerStateGrammar, []cli.Option{
			{
				LongName:    "force",
				Description: "server.stop.force",
				OptionType:  cli.BooleanType,
			},
		}...),
	},
}

var RestartVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.server.restart",
		Action:      commands.Restart,
		OptionType:  cli.Subcommand,
		//DefaultVerb: true,
		Unsupported: []string{"windows"},
		Value: append(ServerStateGrammar, []cli.Option{
			{
				LongName:    "force",
				Description: "server.stop.force",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "new-token",
				Description: "new.token",
				OptionType:  cli.BooleanType,
			},
		}...),
	},
}
