package grammar

import "github.com/tucats/ego/app-cli/cli"

var RevokeVerbGrammar = []cli.Option{
	{
		LongName:      "dsn",
		Description:   "ego.verb.revoke.dsn",
		OptionType:    cli.Subcommand,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value:         GrantObjectGrammar,
	},
	{
		LongName:      "table",
		Description:   "ego.verb.revoke.table",
		OptionType:    cli.Subcommand,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		Value:         GrantObjectGrammar,
	},
}

var GrantObjectGrammar = []cli.Option{
	{
		LongName:    "username",
		Aliases:     []string{"user"},
		ShortName:   "u",
		Description: "dsns.revoke.username",
		OptionType:  cli.StringType,
		Required:    true,
	},
	{
		LongName:    "permissions",
		Aliases:     []string{"perms"},
		ShortName:   "p",
		Description: "dsns.revoke.permissions",
		OptionType:  cli.StringListType,
		Keywords:    []string{"read", "write", "admin"},
		Required:    true,
	},
}
