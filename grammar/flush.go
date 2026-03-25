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
		Value:       CacheFlushGrammar,
	},
	{
		LongName:    "tokens",
		Description: "ego.verb.flush.tokens",
		OptionType:  cli.Subcommand,
		Action:      commands.TokenFlush,
	},
}

var CacheFlushGrammar = []cli.Option{
	{
		LongName:    "services",
		Aliases:     []string{"service"},
		Description: "ego.verb.flush.cache.services",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "assets",
		Aliases:     []string{"asset"},
		Description: "ego.verb.flush.cache.assets",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "authorizations",
		Aliases:     []string{"authorization", "permission", "permissions"},
		Description: "ego.verb.flush.cache.authorizations",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "blacklist",
		Description: "ego.verb.flush.cache.blacklist",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "tokens",
		Aliases:     []string{"token"},
		Description: "ego.verb.flush.cache.tokens",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "dsns",
		Aliases:     []string{"dsn", "data-source-name", "data-source-names"},
		Description: "ego.verb.flush.cache.dsns",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "schemas",
		Aliases:     []string{"schema"},
		Description: "ego.verb.flush.cache.schemas",
		OptionType:  cli.BooleanType,
	},
}
