package grammar

import (
	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

// EgoGrammar handles the command line options. There is an entry here for
// each subcommand specific to Ego (not those that are supplied by the
// app-cli framework).
//
// This version of the grammar uses verb/subject order, rather than the
// collective/detailed order used in the previous version.
//
// Primary verbs:
//  create dsn
//. create table
//. create user
//  delete dsn
//. delete rows
//. delete table
//. delete user
//  describe config
//  flush server cache
//  format log
//  grant dsn permission
//. grant table permission
//. insert row
//  list dsns
//. list tables
//. list users
//. logon
//. path
//. read rows
//. restart server
//. revoke table permission
//  revoke dsn permission
//. run
//  set  config
//. set cache size
//  show dsn
//. show log
//. show log status
//. show path
//. show server cache
//  show server status
//. show server validations
//. show server log file
//  show server log status
//. show server memory
//. show table schema
//  show table permissions
//. show users
//. show user permissions
//  start server
//. stop server
//  sql
//  test
//  update rows

var EgoGrammar2 = []cli.Option{
	{
		LongName:   "service",
		OptionType: cli.StringType,
		Action:     app.ChildService,
		Private:    true,
	},
	{
		LongName:      "create",
		OptionType:    cli.Subcommand,
		Value:         CreateVerbGrammar,
		Description:   "ego.verb.create",
		ExpectedParms: 1,
		ParmDesc:      "opt.type",
	},
	{
		LongName:      "delete",
		OptionType:    cli.Subcommand,
		Value:         DeleteVerbGrammar,
		Description:   "ego.verb.delete",
		ExpectedParms: 1,
		ParmDesc:      "opt.type",
	},
	{
		LongName:    "describe",
		OptionType:  cli.Subcommand,
		Value:       DescribeVerbGrammar,
		Description: "ego.verb.describe",
	},
	{
		LongName:      "flush",
		OptionType:    cli.Subcommand,
		Value:         FlushVerbGrammar,
		Description:   "ego.verb.flush",
		ExpectedParms: 1,
		ParmDesc:      "opt.type",
	},
	{
		LongName:      "format",
		OptionType:    cli.Subcommand,
		Value:         FormatVerbGrammar,
		Description:   "ego.verb.format",
		ExpectedParms: 1,
		ParmDesc:      "opt.type",
	},
	{
		LongName:    "grant",
		OptionType:  cli.Subcommand,
		Value:       GrantVerbGrammar,
		Description: "ego.verb.grant",
	},
	{
		LongName:    "insert",
		OptionType:  cli.Subcommand,
		Value:       InsertVerbGrammar,
		Description: "ego.verb.insert",
	},
	{
		LongName:    "list",
		OptionType:  cli.Subcommand,
		Value:       ListVerbGrammar,
		Description: "ego.verb.list",
	},
	{
		LongName:    "log",
		Aliases:     []string{"logging"},
		Description: "ego.verb.show.server.log",
		OptionType:  cli.Subcommand,
		Action:      commands.Logging,
		Value:       ShowVerbLogGrammar,
		Private:     true,
	},
	{
		LongName:    "logon",
		Aliases:     []string{"login"},
		OptionType:  cli.Subcommand,
		Value:       LogonVerbGrammar,
		Description: "ego.verb.logon",
		Action:      app.Logon,
	},
	{
		LongName:    "path",
		OptionType:  cli.Subcommand,
		Action:      commands.PathAction,
		Description: "ego.verb.path",
		Private:     true,
	},
	{
		LongName:    "read",
		Aliases:     []string{"select"},
		OptionType:  cli.Subcommand,
		Value:       ReadVerbGrammar,
		Description: "ego.verb.read",
	},
	{
		LongName:    "restart",
		OptionType:  cli.Subcommand,
		Value:       RestartVerbGrammar,
		Description: "ego.verb.restart",
	},
	{
		LongName:    "revoke",
		OptionType:  cli.Subcommand,
		Value:       RevokeVerbGrammar,
		Description: "ego.verb.revoke",
	},
	{
		LongName:      "run",
		Description:   "ego.run",
		OptionType:    cli.Subcommand,
		Action:        commands.RunAction,
		Value:         RunGrammar,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file",
		DefaultVerb:   true,
	},
	{
		LongName:    "set",
		OptionType:  cli.Subcommand,
		Value:       SetVerbGrammar,
		Description: "ego.verb.set",
	},
	{
		LongName:    "server",
		OptionType:  cli.Subcommand,
		Value:       ServerRunGrammar,
		Action:      commands.RunServer,
		Description: "ego.verb.server",
	},
	{
		LongName:    "show",
		OptionType:  cli.Subcommand,
		Value:       ShowVerbGrammar,
		Description: "ego.verb.show",
	},
	{
		LongName:    "start",
		OptionType:  cli.Subcommand,
		Value:       StartVerbGrammar,
		Description: "ego.verb.start",
	},
	{
		LongName:    "stop",
		OptionType:  cli.Subcommand,
		Value:       StopVerbGrammar,
		Description: "ego.verb.stop",
	},
	{
		LongName:      "sql",
		Description:   "ego.sql",
		OptionType:    cli.Subcommand,
		Action:        commands.TableSQL,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "sql-text",
		Value:         SQLGrammar,
	},
	{
		LongName:      "test",
		Description:   "ego.test",
		OptionType:    cli.Subcommand,
		Value:         TestGrammar,
		Action:        commands.TestAction,
		ExpectedParms: defs.VariableParameterCount,
		ParmDesc:      "parm.file.or.path",
	},
	{
		LongName:      "update",
		Description:   "ego.verb.update",
		OptionType:    cli.Subcommand,
		Action:        commands.TableUpdate,
		ExpectedParms: defs.VariableParameterCount,
		MinParams:     1,
		Prompts:       []string{"prompt.table"},
		ParmDesc:      "parm.table.update",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				ShortName:   "d",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "table.update.filter",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:    "version",
		Description: "opt.global.version",
		OptionType:  cli.Subcommand,
		Action:      app.VersionAction,
	},
}
