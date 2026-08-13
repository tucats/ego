package class

import (
	"github.com/tucats/ego/internal/cli/app"
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/grammar/common"
)

// MainGrammar handles the command line options. There is an entry here for
// each subcommand specific to Ego (not those that are supplied by the
// app-cli framework).
var MainGrammar = []cli.Option{
	{
		LongName:    "cluster",
		OptionType:  cli.Subcommand,
		Value:       common.ClusterVerbGrammar,
		Description: "ego.cluster",
	},
	{
		LongName:      "fmt",
		OptionType:    cli.Subcommand,
		Action:        commands.FmtAction,
		Value:         common.FmtVerbGrammar,
		Description:   "ego.verb.fmt",
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file",
	},
	{
		LongName:    "tokens",
		Aliases:     []string{"blacklist"},
		Description: "ego.tokens",
		Value:       TokenGrammar,
		OptionType:  cli.Subcommand,
	},
	{
		LongName:   "service",
		OptionType: cli.StringType,
		Action:     app.ChildService,
		Private:    true,
	},
	{
		LongName:    "dsns",
		Aliases:     []string{defs.DSNOption},
		Description: "ego.dsns",
		OptionType:  cli.Subcommand,
		Value:       DSNSGrammar,
	},
	{
		LongName:      "sql",
		Description:   "ego.sql",
		OptionType:    cli.Subcommand,
		Action:        commands.TableSQL,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "sql-text",
		Value:         common.SQLGrammar,
	},
	{
		LongName:    "table",
		Aliases:     []string{"tables", "db", "database"},
		Description: "ego.table",
		OptionType:  cli.Subcommand,
		Value:       TableGrammar,
	},
	{
		LongName:      "path",
		Description:   "ego.path",
		OptionType:    cli.Subcommand,
		Action:        commands.PathAction,
		ExpectedParms: 0,
	},
	{
		LongName:      "log",
		Aliases:       []string{"formatlog", "format-log"},
		OptionType:    cli.Subcommand,
		Description:   "ego.log",
		Action:        commands.FormatLog,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "opt.log.file",
		Value:         common.FormatLogGrammar,
	},
	{
		LongName:    "rest",
		OptionType:  cli.Subcommand,
		Description: "ego.verb.rest",
		Value:       common.RestGrammar,
	},
	{
		LongName:      "run",
		Description:   "ego.run",
		OptionType:    cli.Subcommand,
		Action:        commands.RunAction,
		Value:         common.RunGrammar,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file",
		DefaultVerb:   true,
	},
	{
		LongName:    "server",
		Description: "ego.server",
		OptionType:  cli.Subcommand,
		Value:       ServerGrammar,
	},
	{
		LongName:      "test",
		Description:   "ego.test",
		OptionType:    cli.Subcommand,
		Value:         common.TestGrammar,
		Action:        commands.TestAction,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file.or.path",
	},
}
