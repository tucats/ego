package grammar

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
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
