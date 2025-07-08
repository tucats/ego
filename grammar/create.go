package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var CreateVerbGrammar = []cli.Option{
	{
		LongName:      "dsn",
		Description:   "ego.verb.create.dsn",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSAdd,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value:         CreateDSNGrammar,
	},
	{
		LongName:      "table",
		Description:   "ego.verb.create.table",
		OptionType:    cli.Subcommand,
		Action:        commands.TableCreate,
		ParmDesc:      "parms,table.create",
		ExpectedParms: defs.VariableParameterCount,
		Value:         CreateTableGrammar,
	},
	{
		LongName:      "user",
		Aliases:       []string{"username"},
		Description:   "ego.verb.create.user",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.AddUser,
		Value:         ServerUserGrammar,
	},
}
