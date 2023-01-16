package main

import (
	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/defs"
)

// EgoGrammar handles the command line options. There is an entry here for
// each subcommand specific to Ego (not those that are supplied by the
// app-cli framework).
var EgoGrammar = []cli.Option{
	{
		LongName:             "sql",
		Description:          "ego.sql",
		OptionType:           cli.Subcommand,
		Action:               commands.TableSQL,
		ParametersExpected:   -99,
		ParameterDescription: "sql-text",
		Value:                SQLGrammar,
	},
	{
		LongName:    "table",
		Aliases:     []string{"tables", "db", "database"},
		Description: "ego.table",
		OptionType:  cli.Subcommand,
		Value:       TableGrammar,
	},
	{
		LongName:           "path",
		Description:        "ego.path",
		OptionType:         cli.Subcommand,
		Action:             commands.PathAction,
		ParametersExpected: 0,
	},
	{
		LongName:             "run",
		Description:          "ego.run",
		OptionType:           cli.Subcommand,
		Action:               commands.RunAction,
		Value:                RunGrammar,
		ParametersExpected:   -99,
		ParameterDescription: "parm.file",
	},
	{
		LongName:    "server",
		Description: "ego.server",
		OptionType:  cli.Subcommand,
		Value:       ServerGrammar,
	},
	{
		LongName:             "test",
		Description:          "ego.test",
		OptionType:           cli.Subcommand,
		Value:                TestGrammar,
		Action:               commands.TestAction,
		ParametersExpected:   -99,
		ParameterDescription: "parm.file.or.path",
	},
}

// SQLGrammar specifies the command line options for the "sql" Ego command.
var SQLGrammar = []cli.Option{
	{
		LongName:    "sql-file",
		ShortName:   "f",
		Aliases:     []string{"file"},
		Description: "ego.sql.file",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "row-ids",
		ShortName:   "i",
		Aliases:     []string{"ids"},
		Description: "ego.sql.row-ids",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "row-numbers",
		ShortName:   "n",
		Aliases:     []string{"row-number", "row"},
		Description: "ego.sql.row-numbers",
		OptionType:  cli.BooleanType,
	},
}

// TableGrammar specifies the command line options for the "tables" Ego command.
var TableGrammar = []cli.Option{
	{
		LongName:             "sql",
		Description:          "ego.table.sql",
		OptionType:           cli.Subcommand,
		Action:               commands.TableSQL,
		ParametersExpected:   -99,
		ParameterDescription: "parm.sql.text",
		Value: []cli.Option{
			{
				LongName:    "sql-file",
				ShortName:   "f",
				Aliases:     []string{"file"},
				Description: "opt.sql.file",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "opt.sql.row.ids",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-numbers",
				ShortName:   "n",
				Aliases:     []string{"row-number", "row"},
				Description: "opt.sql.row.numbers",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:           "permissions",
		Aliases:            []string{"perms"},
		Description:        "ego.table.permissions",
		OptionType:         cli.Subcommand,
		Action:             commands.TablePermissions,
		ParametersExpected: 0,
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "opt.table.permissions.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:             "show-permission",
		Aliases:              []string{"permission", "perm"},
		Description:          "ego.table.permission",
		OptionType:           cli.Subcommand,
		Action:               commands.TableShowPermission,
		ParametersExpected:   1,
		ParameterDescription: "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "opt.table.permission.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:             "grant",
		Aliases:              []string{"permission"},
		Description:          "ego.table.grant",
		OptionType:           cli.Subcommand,
		Action:               commands.TableGrant,
		ParametersExpected:   1,
		ParameterDescription: "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    "permission",
				Aliases:     []string{"permission", "permissions", "perms", "perm"},
				ShortName:   "p",
				Description: "opt.table.grant.permission",
				OptionType:  cli.StringListType,
				Required:    true,
			},
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "opt.table.grant.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:    "list",
		Description: "ego.table.list",
		OptionType:  cli.Subcommand,
		Action:      commands.TableList,
		Value: []cli.Option{
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "opt.limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "opt.start",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "no-row-counts",
				Description: "opt.table.list.no.row.counts",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:           "show-table",
		Aliases:            []string{"show", "metadata", "columns"},
		Description:        "ego.table.show",
		OptionType:         cli.Subcommand,
		Action:             commands.TableShow,
		ParametersExpected: 1,
	},
	{
		LongName:             "drop",
		Description:          "ego.table.drop",
		OptionType:           cli.Subcommand,
		Action:               commands.TableDrop,
		ParametersExpected:   -99,
		ParameterDescription: "table-name [table-name...]",
	},
	{
		LongName:           "read",
		Aliases:            []string{"select", "print", "get", "show-contents", "contents"},
		Description:        "ego.table.read",
		OptionType:         cli.Subcommand,
		Action:             commands.TableContents,
		ParametersExpected: 1,
		Value: []cli.Option{
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "opt.table.read.row.ids",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-numbers",
				ShortName:   "n",
				Aliases:     []string{"row-number", "row"},
				Description: "opt.table.read.row.numbers",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "columns",
				ShortName:   "c",
				Aliases:     []string{"column"},
				Description: "opt.table.read.columns",
				OptionType:  cli.StringListType,
			},

			{
				LongName:    "order-by",
				ShortName:   "o",
				Aliases:     []string{"sort", "order"},
				Description: "opt.table.read.order.by",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "opt.filter",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "opt.limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "opt.start",
				OptionType:  cli.IntType,
			},
		},
	},
	{
		LongName:           "delete",
		Description:        "ego.table.delete",
		OptionType:         cli.Subcommand,
		Action:             commands.TableDelete,
		ParametersExpected: 1,
		Value: []cli.Option{
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "opt.table.delete.filter",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "insert",
		Aliases:              []string{"write", "append"},
		Description:          "ego.table.insert",
		OptionType:           cli.Subcommand,
		Action:               commands.TableInsert,
		ParametersExpected:   -99,
		ParameterDescription: "parm.table.insert",
		Value: []cli.Option{
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "opt.table.insert.file",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "update",
		Description:          "ego.table.update",
		OptionType:           cli.Subcommand,
		Action:               commands.TableUpdate,
		ParametersExpected:   -99,
		ParameterDescription: "parm.table.update",
		Value: []cli.Option{
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "opt.table.update.filter",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "create",
		Description:          "ego.table.create",
		OptionType:           cli.Subcommand,
		Action:               commands.TableCreate,
		ParametersExpected:   -999,
		ParameterDescription: "parm.table.create",
		Value: []cli.Option{
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "opt.table.create.file",
				OptionType:  cli.StringListType,
			},
		},
	},
}

var ServerDeleteGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "opt.server.delete.user",
		OptionType:  cli.StringType,
	},
}

var ServerUserGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "opt.server.user.user",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "password",
		ShortName:   "p",
		Description: "opt.server.user.pass",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "permissions",
		Description: "opt.server.user.perms",
		OptionType:  cli.StringListType,
	},
}

var ServerListUsersGrammar = []cli.Option{
	{
		LongName:    "id",
		ShortName:   "i",
		Description: "opt.server.show.id",
		OptionType:  cli.BooleanType,
	},
}

// UserGrammar contains the grammar for SERVER USERS subcommands.
var UserGrammar = []cli.Option{
	{
		LongName:    "set",
		Description: "ego.server.user.set",
		Aliases:     []string{"add", "create"},
		OptionType:  cli.Subcommand,
		Action:      commands.AddUser,
		Value:       ServerUserGrammar,
	},
	{
		LongName:    "delete",
		Description: "ego.server.user.delete",
		OptionType:  cli.Subcommand,
		Action:      commands.DeleteUser,
		Value:       ServerDeleteGrammar,
	},
	{
		LongName:    "list",
		Description: "ego.server.user.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
		Value:       ServerListUsersGrammar,
	},
}

// CachesGrammar defines the grammar for the SERVER CACHES subcommands.
var CachesGrammar = []cli.Option{
	{
		LongName:    "flush",
		Description: "ego.server.cache.flush",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushServerCaches,
		Value:       ServerStateGrammar,
	},
	{
		LongName:    "list",
		Aliases:     []string{"show"},
		Description: "ego.server.cache.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ListServerCaches,
		Value:       ServerStateGrammar,
	},
	{
		LongName:             "set-size",
		Description:          "ego.server.cache.set.size",
		ParametersExpected:   1,
		ParameterDescription: "limit",
		OptionType:           cli.Subcommand,
		Action:               commands.SetCacheSize,
		Value:                ServerStateGrammar,
	},
}

// LoggingGrammar is the ego server logging grammar.
var LoggingGrammar = []cli.Option{
	{
		LongName:    "enable",
		Aliases:     []string{"set"},
		Description: "opt.server.logging.enable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "disable",
		Aliases:     []string{"clear"},
		Description: "opt.server.logging.disable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "file",
		ShortName:   "f",
		Description: "opt.server.logging.file",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "status",
		ShortName:   "s",
		Description: "opt.server.logging.status",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "limit",
		ShortName:   "l",
		Description: "opt.limit",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "session",
		Description: "opt.server.logging.session",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "keep",
		Description: "opt.server.logging.keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "opt.port",
	},
}

// ServerGrammar contains the grammar of SERVER subcommands.
var ServerGrammar = []cli.Option{
	{
		LongName:             "logging",
		Aliases:              []string{"logger", "log", "logs"},
		Description:          "ego.server.logging",
		OptionType:           cli.Subcommand,
		Value:                LoggingGrammar,
		ParametersExpected:   -1,
		ParameterDescription: "parm.address.port",
		Action:               commands.Logging,
	},
	{
		LongName:    "logon",
		Aliases:     []string{"login"},
		OptionType:  cli.Subcommand,
		Description: "ego.server.logon",
		Action:      app.Logon,
		Value:       app.LogonGrammar,
	},
	{
		LongName:    "users",
		Aliases:     []string{"user"},
		Description: "ego.server.users",
		OptionType:  cli.Subcommand,
		Value:       UserGrammar,
	},
	{
		LongName:    "caches",
		Aliases:     []string{"cache"},
		Description: "ego.server.caches",
		OptionType:  cli.Subcommand,
		Value:       CachesGrammar,
	},
	{
		LongName:    "run",
		Description: "ego.server.run",
		OptionType:  cli.Subcommand,
		Action:      commands.RunServer,
		// Run and Start share a grammar, but Run has additional options
		Value: append(ServerRunGrammar, []cli.Option{
			{
				LongName:    "debug-endpoint",
				ShortName:   "d",
				Description: "opt.server.run.debug",
				OptionType:  cli.StringType,
			},
		}...),
	},
	{
		LongName:    "restart",
		Description: "ego.server.restart",
		OptionType:  cli.Subcommand,
		Action:      commands.Restart,
		Value:       ServerStateGrammar,
	},
	{
		LongName:             "status",
		Description:          "ego.server.status",
		OptionType:           cli.Subcommand,
		Action:               commands.Status,
		ParametersExpected:   -1,
		ParameterDescription: "opt.address.port",
		Value:                ServerStateGrammar,
	},
	{
		LongName:    "start",
		Description: "ego.server.start",
		OptionType:  cli.Subcommand,
		Action:      commands.Start,
		Value:       ServerRunGrammar,
	},
	{
		LongName:    "stop",
		Description: "ego.server.stop",
		OptionType:  cli.Subcommand,
		Action:      commands.Stop,
		Value:       ServerStopGrammar,
	},
}

// ServerStopGrammar handles command line options for the server subcommand.
var ServerStopGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "opt.port",
		EnvironmentVariable: "EGO_PORT",
	},
}

// ServerStateGrammar  is a common sub-grammar for specifying a port.
var ServerStateGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "opt.port",
		EnvironmentVariable: "EGO_PORT",
	},
	{
		LongName:    "local",
		ShortName:   "l",
		Aliases:     []string{"pid", "pidfile"},
		OptionType:  cli.BooleanType,
		Description: "opt.local",
	},
}

// ServerRunGrammar handles command line options for the server subcommand.
var ServerRunGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "opt.port",
		EnvironmentVariable: "EGO_PORT",
	},
	{
		LongName:            "not-secure",
		ShortName:           "k",
		OptionType:          cli.BooleanType,
		Description:         "opt.server.run.not.secure",
		EnvironmentVariable: "EGO_INSECURE",
	},
	{
		LongName:    "is-detached",
		OptionType:  cli.BooleanType,
		Description: "opt.server.run.is.detached",
		Private:     true,
	},
	{
		LongName:    "force",
		ShortName:   "f",
		OptionType:  cli.BooleanType,
		Description: "opt.server.run.force",
		Private:     true,
	},
	{
		LongName:    "log",
		Description: "opt.server.run.log",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "keep-logs",
		Description: "opt.server.run.keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "sandbox-path",
		Description: "opt.server.run.sandbox",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "no-log",
		Description: "opt.server.run.no.log",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "opt.trace",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
	{
		LongName:    "full-symbol-scope",
		Description: "opt.scope",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "symbol-allocation",
		Description: "opt.symbol.allocation",
		OptionType:  cli.IntType,
	},
	{
		LongName:            "static-types",
		Description:         "opt.server.run.static",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_STATIC_TYPES",
	},
	{
		LongName:            "realm",
		ShortName:           "r",
		Description:         "opt.server.run.realm",
		OptionType:          cli.StringType,
		EnvironmentVariable: "EGO_REALM",
	},
	{
		LongName:    "cache-size",
		Description: "opt.server.run.cache",
		OptionType:  cli.IntType,
	},
	{
		LongName:            "users",
		Aliases:             []string{"user-database"},
		ShortName:           "u",
		Description:         "opt.server.run.users",
		OptionType:          cli.StringType,
		EnvironmentVariable: "EGO_USERS",
	},
	{
		LongName:    "superuser",
		Description: "opt.server.run.superuser",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "code",
		ShortName:   "c",
		Description: "opt.server.run.code",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "session-uuid",
		Description: "opt.server.run.uuid",
		OptionType:  cli.UUIDType,
	},
}

// RunGrammar handles the command line options.
var RunGrammar = []cli.Option{
	{
		LongName:            "disassemble",
		Aliases:             []string{"disasm"},
		Description:         "opt.run.disasm",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_DISASM",
	},
	{
		LongName:    "log",
		Description: "opt.run.log",
		OptionType:  cli.StringType,
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "opt.trace",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
	{
		LongName:            "static-types",
		Description:         "opt.run.static",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_STATIC_TYPES",
	},
	{
		LongName:    "debug",
		ShortName:   "d",
		Description: "opt.run.debug",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.OptimizerOption,
		ShortName:   "o",
		Description: "opt.run.optimize",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "full-symbol-scope",
		Description: "opt.scope",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "symbols",
		ShortName:   "s",
		Description: "opt.run.symbols",
		OptionType:  cli.BooleanType,
		Private:     true,
	},
	{
		LongName:    "symbol-allocation",
		Description: "opt.symbol.allocation",
		OptionType:  cli.IntType,
	},
	{
		LongName:            "auto-import",
		Description:         "opt.run.auto.import",
		OptionType:          cli.BooleanValueType,
		EnvironmentVariable: "EGO_AUTOIMPORT",
	},
	{
		LongName:    "entry-point",
		ShortName:   "e",
		Description: "opt.run.entry.point",
		OptionType:  cli.StringType,
	},
}

// TestGrammar handles the command line options.
var TestGrammar = []cli.Option{
	{
		LongName:            "static-types",
		Description:         "opt.run.static",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_STATIC_TYPES",
	},
	{
		LongName:    "debug",
		ShortName:   "d",
		Description: "opt.run.debug",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.OptimizerOption,
		ShortName:   "o",
		Description: "opt.run.optimize",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "opt.trace",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
}
