package grammar

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
)

var FormatVerbGrammar = []cli.Option{
	{
		LongName:      "json",
		Description:   "ego.verb.format.json",
		OptionType:    cli.Subcommand,
		Action:        commands.FormatJSON,
		Value:         FormatJSONGrammar,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "opt.json.file",
	},
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

var FormatJSONGrammar = []cli.Option{
	{
		LongName:    "indented",
		Aliases:     []string{"indent", "pretty"},
		ShortName:   "i",
		Description: "json.indented",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "query",
		Aliases:     []string{"json-query", "jq"},
		ShortName:   "q",
		Description: "json.query",
		OptionType:  cli.StringType,
	},
}
