package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

var CreateVerbGrammar = []cli.Option{
	{
		LongName:      defs.DSNOption,
		Description:   "ego.verb.create.dsn",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSAdd,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value:         CreateDSNGrammar,
		MinParams:     1,
		Prompts:       []string{"prompt.dsn"},
	},
	{
		LongName:      "table",
		Description:   "ego.verb.create.table",
		OptionType:    cli.Subcommand,
		Action:        commands.TableCreate,
		ParmDesc:      "parms,table.create",
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
		Prompts:       []string{i18n.L("prompt.table")},
		Value:         CreateTableGrammar,
	},
	{
		LongName:      "user",
		Aliases:       []string{defs.UsernameOption},
		Description:   "ego.verb.create.user",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		MinParams:     1,
		Prompts:       []string{"prompt.user"},
		Action:        commands.AddUser,
		Value:         ServerUserGrammar,
	},
}
