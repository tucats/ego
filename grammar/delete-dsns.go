package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var DeleteDSNSGrammar = []cli.Option{
	{
		LongName:      "delete",
		Description:   "ego.dsns.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSDelete,
		ParmDesc:      "dsn-name[ ds-name...]",
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
	},
}
