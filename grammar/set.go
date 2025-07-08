package grammar

import (
	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
)

var SetVerbGrammar = []cli.Option{
	{
		LongName:      "cache",
		Description:   "ego.server.cache.set.size",
		ExpectedParms: 1,
		ParmDesc:      "[count]",
		OptionType:    cli.Subcommand,
		Action:        commands.SetCacheSize,
	},
	{
		LongName:      "config",
		Aliases:       []string{"conf"},
		Description:   "ego.verb.set.config",
		MinParams:     1,
		OptionType:    cli.Subcommand,
		Action:        app.SetAction,
		ExpectedParms: 1,
		ParmDesc:      "parm.config.key.value",
	},
	{
		LongName:      "description",
		OptionType:    cli.Subcommand,
		Description:   "ego.verb.set.description",
		ParmDesc:      "text",
		ExpectedParms: 1,
		Action:        config.SetDescriptionAction,
	},
	{
		LongName:    "logging",
		Aliases:     []string{"log"},
		Description: "ego.verb.set.logging",
		MinParams:   1,
		OptionType:  cli.Subcommand,
		Value:       SetLoggingGrammar,
	},
	{
		LongName:      "output",
		OptionType:    cli.Subcommand,
		Description:   "ego.verb.set.output",
		ParmDesc:      "type",
		Action:        config.SetOutputAction,
		ExpectedParms: 1,
	},
}
