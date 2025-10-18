package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

var FlushVerbGrammar = []cli.Option{
	{
		LongName:    "cache",
		Description: "ego.verb.flush.cache",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushCaches,
	},
	{
		LongName:    "tokens",
		Description: "ego.verb.flush.tokens",
		OptionType:  cli.Subcommand,
		Action:      commands.TokenFlush,
	},
}
