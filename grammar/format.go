package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

var FormatVerbGrammar = []cli.Option{
	{
		LongName:      "log",
		Description:   "ego.verb.format.log",
		OptionType:    cli.Subcommand,
		Action:        commands.FormatLog,
		DefaultVerb:   true,
		Value:         FormatLogGrammar,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "opt.log.file",
	},
}
