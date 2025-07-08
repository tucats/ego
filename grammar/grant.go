package grammar

import "github.com/tucats/ego/app-cli/cli"

var GrantVerbGrammar = []cli.Option{
	{
		LongName:      "dsn",
		Description:   "ego.verb.grant.dsn",
		OptionType:    cli.Subcommand,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value:         GrantObjectGrammar,
	},
	{
		LongName:      "table",
		Description:   "ego.verb.grant.table",
		OptionType:    cli.Subcommand,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		Value:         GrantObjectGrammar,
	},
}
