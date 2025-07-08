package grammar

import "github.com/tucats/ego/app-cli/cli"

var SetLoggingGrammar = []cli.Option{
	{
		LongName:    "enable",
		Aliases:     []string{"set"},
		Description: "server.logging.enable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "disable",
		Aliases:     []string{"clear"},
		Description: "server.logging.disable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "keep",
		Description: "server.logging.keep",
		OptionType:  cli.IntType,
	},
}
