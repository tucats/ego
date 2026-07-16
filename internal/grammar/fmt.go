package grammar

import (
	"github.com/tucats/ego/internal/cli/cli"
)

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
