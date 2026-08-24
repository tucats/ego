package verb

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/grammar/common"
)

var StartVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.server.start",
		Action:      commands.Start,
		OptionType:  cli.Subcommand,
		//DefaultVerb: true,
		Unsupported: []string{"windows"},
		Value:       common.ServerRunGrammar,
	},
	{
		LongName:      "cluster",
		Description:   "ego.cluster.start",
		Action:        commands.ClusterStart,
		OptionType:    cli.Subcommand,
		ParmDesc:      "parm.cluster.name",
		ExpectedParms: 1,
		Prompts:       []string{"prompt.cluster.name"},
		Unsupported:   []string{"windows"},
		Value:         common.ClusterStartGrammar,
	},
	{
		LongName:      "task",
		Description:   "ego.verb.start.task",
		Action:        commands.TaskStart,
		OptionType:    cli.Subcommand,
		ParmDesc:      "task-id",
		ExpectedParms: 1,
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
		Value: append(common.ServerStateGrammar, []cli.Option{
			{
				LongName:    "force",
				Description: "server.stop.force",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "grace",
				Description: "server.stop.grace",
				OptionType:  cli.StringType,
			},
		}...),
	},
	{
		LongName:    "cluster",
		Description: "ego.cluster.stop",
		OptionType:  cli.Subcommand,
		Action:      commands.ClusterStopNode,
		Value: append(common.ClusterNodeGrammar, []cli.Option{
			{
				LongName:    "all",
				ShortName:   "a",
				Description: "opt.cluster.all",
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
		Value: append(common.ServerStateGrammar, []cli.Option{
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
