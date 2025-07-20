package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var SetVerbGrammar = []cli.Option{
	{
		LongName:      "cache",
		Description:   "ego.server.cache.set.size",
		ExpectedParms: 1,
		ParmDesc:      "[count]",
		OptionType:    cli.Subcommand,
		Action:        commands.SetCacheSize,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.cache.size")},
	},
	{
		LongName:      "config",
		Aliases:       []string{"conf"},
		Description:   "ego.verb.set.config",
		MinParams:     1,
		OptionType:    cli.Subcommand,
		Action:        config.SetAction,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.config.key.value",
		Prompts:       []string{i18n.L("prompt.kv")},
	},
	{
		LongName:      "description",
		OptionType:    cli.Subcommand,
		Description:   "ego.verb.set.description",
		ParmDesc:      "text",
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.text")},
		Action:        config.SetDescriptionAction,
	},
	{
		LongName:      "logging",
		Aliases:       []string{"log"},
		Description:   "ego.verb.set.logging",
		OptionType:    cli.Subcommand,
		Value:         SetLoggingGrammar,
		Action:        commands.Logging,
		ExpectedParms: defs.VariableParameterCount,
	},
	{
		LongName:      "output",
		OptionType:    cli.Subcommand,
		Description:   "ego.verb.set.output",
		ParmDesc:      "type",
		Action:        config.SetOutputAction,
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.output.type")},
	},
	{
		LongName:      "user",
		Description:   "ego.verb.set.user",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.user")},
		Action:        commands.UpdateUser,
		Value:         ServerUserGrammar,
	},
}
