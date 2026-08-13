package class

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/grammar/common"
)

// CachesGrammar defines the grammar for the SERVER CACHES subcommands.
var CachesGrammar = []cli.Option{
	{
		LongName:    "flush",
		Description: "ego.server.cache.flush",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushCaches,
		Value:       common.ServerStateGrammar,
	},
	{
		LongName:    "show",
		Aliases:     []string{"list"},
		Description: "ego.server.cache.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ShowCaches,
		Value: []cli.Option{
			{
				LongName:    "services",
				Aliases:     []string{"service"},
				ShortName:   "s",
				Description: "cache.list.services",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "assets",
				Aliases:     []string{"asset"},
				ShortName:   "a",
				Description: "cache.list.assets",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "order-by",
				Aliases:     []string{"sort", "order"},
				Description: "cache.list.order.by",
				OptionType:  cli.KeywordType,
				Keywords:    []string{"url", "count", "last-used"},
			},
		},
		DefaultVerb: true,
	},
	{
		LongName:      "set-size",
		Description:   "ego.server.cache.set.size",
		ExpectedParms: 1,
		ParmDesc:      "limit",
		OptionType:    cli.Subcommand,
		Action:        commands.SetCacheSize,
		Value:         common.ServerStateGrammar,
	},
}
