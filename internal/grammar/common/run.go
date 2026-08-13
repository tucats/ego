package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

// RunGrammar handles the command line options.
var RunGrammar = []cli.Option{
	{
		LongName:    "disassemble",
		Aliases:     []string{"disasm"},
		Description: "run.disasm",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "project",
		ShortName:   "p",
		Description: "run.project",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:   "pprof",
		OptionType: cli.StringType,
		Private:    true,
	},
	{
		LongName:    "profile",
		Aliases:     []string{"profiling", "prof"},
		Description: "run.profile",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "profile-file",
		Aliases:     []string{"profile-output", "prof-file"},
		Description: "run.profile.file",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "log-file",
		Description: "run.log",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "trace",
		ShortName:   "t",
		Description: "trace",
		OptionType:  cli.BooleanType,
		EnvVar:      defs.EgoTraceEnv,
	},
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      defs.EgoTypesEnv,
	},
	{
		LongName:    "debug",
		ShortName:   "d",
		Description: "run.debug",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.OptimizerOption,
		ShortName:   "o",
		Description: "run.optimize",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "full-symbol-scope",
		Description: "scope",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "symbols",
		ShortName:   "s",
		Description: "run.symbols",
		OptionType:  cli.BooleanType,
		Private:     true,
	},
	{
		LongName:    "symbol-allocation",
		Description: "symbol.allocation",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "auto-import",
		Description: "run.auto.import",
		OptionType:  cli.BooleanValueType,
	},
	{
		LongName:    "entry-point",
		ShortName:   "e",
		Description: "run.entry.point",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "sandbox",
		Description: "run.sandbox",
		OptionType:  cli.BooleanValueType,
	},
}
