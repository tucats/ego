package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var FlushVerbGrammar = []cli.Option{
	{
		LongName:    "cache",
		Description: "ego.verb.flush.cache",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushCaches,
		DefaultVerb: true,
		Value: []cli.Option{
			{
				LongName:    "port",
				ShortName:   "p",
				OptionType:  cli.IntType,
				Description: "port",
				EnvVar:      defs.EgoPortEnv,
			},
		},
	},
}
