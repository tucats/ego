package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

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
