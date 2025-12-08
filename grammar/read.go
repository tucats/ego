package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var ReadVerbGrammar = []cli.Option{
	{
		LongName:      "table",
		Aliases:       []string{"rows"},
		Description:   "ego.table.read",
		OptionType:    cli.Subcommand,
		Action:        commands.TableContents,
		DefaultVerb:   true,
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		Value: []cli.Option{
			{
				LongName:    defs.DSNOption,
				ShortName:   "d",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "table.read.row.ids",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-numbers",
				ShortName:   "n",
				Aliases:     []string{"row-number", "row"},
				Description: "table.read.row.numbers",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "columns",
				ShortName:   "c",
				Aliases:     []string{"column"},
				Description: "table.read.columns",
				OptionType:  cli.StringListType,
			},

			{
				LongName:    "order-by",
				ShortName:   "o",
				Aliases:     []string{"sort", "order"},
				Description: "table.read.order.by",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "filter",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
		},
	},
}
