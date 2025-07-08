package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
)

var DescribeVerbGrammar = []cli.Option{
	{
		LongName:    "config",
		Description: "ego.verb.describe.config",
		Action:      config.DescribeAction,
		OptionType:  cli.Subcommand,
		DefaultVerb: true,
		Value: []cli.Option{
			{
				LongName:    "verbose",
				ShortName:   "v",
				OptionType:  cli.BooleanType,
				Description: "config.verbose",
			},
		},
	},
}
