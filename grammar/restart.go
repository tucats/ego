package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

var RestartVerbGrammar = []cli.Option{
	{
		LongName:    "server",
		Description: "ego.server.restart",
		OptionType:  cli.Subcommand,
		Action:      commands.Restart,
		DefaultVerb: true,
		Unsupported: []string{"windows"},
		Value: append(ServerStateGrammar, []cli.Option{
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
}
