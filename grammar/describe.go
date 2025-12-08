package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var DescribeVerbGrammar = []cli.Option{
	{
		LongName:    "config",
		Description: "ego.verb.describe.config",
		Action:      config.DescribeAction,
		OptionType:  cli.Subcommand,
		Value: []cli.Option{
			{
				LongName:    defs.VerboseOption,
				ShortName:   "v",
				OptionType:  cli.BooleanType,
				Description: "config.verbose",
			},
		},
	},
	{
		LongName:      "table",
		Aliases:       []string{"metadata", "schema"},
		Description:   "ego.verb.show.table.columns",
		OptionType:    cli.Subcommand,
		Action:        commands.TableShow,
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		ParmDesc:      "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    defs.DSNOption,
				ShortName:   "d",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
}
