package verb

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/grammar/common"
)

var FlushVerbGrammar = []cli.Option{
	{
		LongName:    "cache",
		Description: "ego.verb.flush.cache",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushCaches,
		Value:       common.CacheFlushGrammar,
	},
	{
		LongName:    "tokens",
		Description: "ego.verb.flush.tokens",
		OptionType:  cli.Subcommand,
		Action:      commands.TokenFlush,
	},
}
