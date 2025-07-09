package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/i18n"
)

var GrantVerbGrammar = []cli.Option{
	{
		LongName:      "dsn",
		Description:   "ego.verb.grant.dsn",
		OptionType:    cli.Subcommand,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.dsn")},
		Value:         GrantObjectGrammar,
	},
	{
		LongName:      "table",
		Description:   "ego.verb.grant.table",
		OptionType:    cli.Subcommand,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		Value:         GrantObjectGrammar,
	},
}
