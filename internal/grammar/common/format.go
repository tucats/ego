package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
)

var FormatVerbGrammar = []cli.Option{
	{
		LongName:      "source",
		OptionType:    cli.Subcommand,
		Action:        commands.FmtAction,
		Value:         FmtVerbGrammar,
		Description:   "ego.verb.fmt",
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file",
	},
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

// FmtVerbGrammar defines the options for the "ego format source" command, which
// parses Ego source into an AST and re-emits it in canonical form. With no file
// arguments it reads standard input and writes to standard output.
var FmtVerbGrammar = []cli.Option{
	{
		LongName:    "write",
		ShortName:   "w",
		Aliases:     []string{"in-place"},
		Description: "fmt.write",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "analysis",
		Description: "fmt.analysis",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "ast",
		ShortName:   "a",
		Aliases:     []string{"tree"},
		Description: "fmt.ast",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "tabs",
		ShortName:   "t",
		Aliases:     []string{"indent", "spaces"},
		Description: "fmt.tabs",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "fragment",
		Description: "fmt.fragment",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "program",
		Description: "fmt.program",
		OptionType:  cli.BooleanType,
	},
}

// FormatLogGrammar specifies the command line options for the "log" Ego command.
var FormatLogGrammar = []cli.Option{
	{
		LongName:    "session",
		Description: "log.session",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "start",
		Aliases:     []string{"first", "begin"},
		Description: "log.start",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "limit",
		Aliases:     []string{"count", "max", "maximum"},
		Description: "log.limit",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "sequence",
		Aliases:     []string{"seq", "sequences", "seqs"},
		Description: "log.sequence",
		OptionType:  cli.RangeType,
	},
	{
		LongName:    "class",
		Aliases:     []string{"type", "kind"},
		Description: "log.class",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "prefix",
		Description: "log.prefix",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "id",
		Aliases:     []string{"uuid"},
		Description: "log.id",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "query",
		ShortName:   "q",
		Aliases:     []string{"json"},
		Description: "json.query",
		OptionType:  cli.StringType,
	},
}
