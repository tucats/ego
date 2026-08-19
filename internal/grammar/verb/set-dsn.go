package verb

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

var SetDSNGrammar = []cli.Option{
	{
		LongName:    defs.PasswordOption,
		Aliases:     []string{"pw"},
		ShortName:   "p",
		Description: "dsns.update.password",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "secured",
		Aliases:     []string{"secure"},
		Description: "dsns.update.secured",
		OptionType:  cli.BooleanValueType,
	},
	{
		LongName:    "restricted",
		Description: "dsns.update.restricted",
		OptionType:  cli.BooleanValueType,
	},
	{
		LongName:    "force",
		Description: "dsns.update.force",
		OptionType:  cli.BooleanType,
	},
}
