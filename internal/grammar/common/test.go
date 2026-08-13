package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

// TestGrammar handles the command line options.
var TestGrammar = []cli.Option{
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      defs.EgoTypesEnv,
	},
	{
		LongName:    "count",
		ShortName:   "c",
		Description: "test.count",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "debug",
		ShortName:   "d",
		Description: "run.debug",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.DisassembleOption,
		Aliases:     []string{"disasm"},
		Description: "run.disasm",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.OptimizerOption,
		ShortName:   "o",
		Description: "run.optimize",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "trace",
		ShortName:   "t",
		Description: "trace",
		OptionType:  cli.BooleanType,
		EnvVar:      defs.EgoTraceEnv,
	},
	{
		LongName:    "sandbox",
		Description: "run.sandbox",
		OptionType:  cli.BooleanValueType,
	},
}
