package main

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

// EgoGrammar handles the command line options.
var EgoGrammar = []cli.Option{
	{
		LongName:    "table",
		Aliases:     []string{"tables", "db", "database"},
		Description: "Operate on database tables",
		OptionType:  cli.Subcommand,
		Value:       TableGrammar,
	},
	{
		LongName:           "path",
		Description:        "Print the default ego path",
		OptionType:         cli.Subcommand,
		Action:             commands.PathAction,
		ParametersExpected: 0,
	},
	{
		LongName:             "run",
		Description:          "Run an existing program",
		OptionType:           cli.Subcommand,
		Action:               commands.RunAction,
		Value:                RunGrammar,
		ParametersExpected:   -99,
		ParameterDescription: "file-name",
	},
	{
		LongName:    "server",
		Description: "Start to accept REST calls",
		OptionType:  cli.Subcommand,
		Value:       ServerGrammar,
	},
	{
		LongName:             "test",
		Description:          "Run a test suite",
		OptionType:           cli.Subcommand,
		Action:               commands.TestAction,
		ParametersExpected:   -99,
		ParameterDescription: "file or path",
	},
}

var TableGrammar = []cli.Option{
	{
		LongName:             "sql",
		Description:          "Directly execute a SQL command",
		OptionType:           cli.Subcommand,
		Action:               commands.TableSQL,
		ParametersExpected:   -99,
		ParameterDescription: "sql-text",
		Value: []cli.Option{
			{
				LongName:    "sql-file",
				ShortName:   "f",
				Aliases:     []string{"file"},
				Description: "Filename of SQL command text",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:           "permissions",
		Aliases:            []string{"perms"},
		Description:        "List all table permissions (requires admin privileges)",
		OptionType:         cli.Subcommand,
		Action:             commands.TablePermissions,
		ParametersExpected: 0,
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "If specified, list only this user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:             "show-permission",
		Aliases:              []string{"permission", "perm"},
		Description:          "List table permissions",
		OptionType:           cli.Subcommand,
		Action:               commands.TableShowPermission,
		ParametersExpected:   1,
		ParameterDescription: "table-name",
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "User (if other than current user) to list)",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:             "grant",
		Aliases:              []string{"permission"},
		Description:          "Set permissions for a given user and table",
		OptionType:           cli.Subcommand,
		Action:               commands.TableGrant,
		ParametersExpected:   1,
		ParameterDescription: "table-name",
		Value: []cli.Option{
			{
				LongName:    "permission",
				Aliases:     []string{"permission", "permissions", "perms", "perm"},
				ShortName:   "p",
				Description: "Permissions to set for this table updated",
				OptionType:  cli.StringListType,
				Required:    true,
			},
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "User (if other than current user) to update",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:    "list",
		Description: "List tables",
		OptionType:  cli.Subcommand,
		Action:      commands.TableList,
		Value: []cli.Option{
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "If specified, limit the result set to this many rows",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "If specified, start result set at this row",
				OptionType:  cli.IntType,
			},
		},
	},
	{
		LongName:           "show-table",
		Aliases:            []string{"show", "metadata", "columns"},
		Description:        "Show table metadata",
		OptionType:         cli.Subcommand,
		Action:             commands.TableShow,
		ParametersExpected: 1,
	},
	{
		LongName:             "drop",
		Description:          "Delete one or more tables",
		OptionType:           cli.Subcommand,
		Action:               commands.TableDrop,
		ParametersExpected:   -99,
		ParameterDescription: "table-name [table-name...]",
	},
	{
		LongName:           "read",
		Aliases:            []string{"select", "print", "get", "show-contents", "contents"},
		Description:        "Read contents of a table",
		OptionType:         cli.Subcommand,
		Action:             commands.TableContents,
		ParametersExpected: 1,
		Value: []cli.Option{
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "Include the row UUID column in the output",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "columns",
				ShortName:   "c",
				Aliases:     []string{"column"},
				Description: "List of optional column names to display. If not specified, all columns are returned.",
				OptionType:  cli.StringListType,
			},

			{
				LongName:    "order-by",
				ShortName:   "o",
				Aliases:     []string{"sort", "order"},
				Description: "List of optional columns use to sort output",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "List of optional filter clauses",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "If specified, limit the result set to this many rows",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "If specified, start result set at this row",
				OptionType:  cli.IntType,
			},
		},
	},
	{
		LongName:           "delete",
		Description:        "Delete rows from a table",
		OptionType:         cli.Subcommand,
		Action:             commands.TableDelete,
		ParametersExpected: 1,
		Value: []cli.Option{
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "Filter for rows to delete. If not specified, all rows are deleted",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "insert",
		Aliases:              []string{"write", "append"},
		Description:          "Insert a row to a table",
		OptionType:           cli.Subcommand,
		Action:               commands.TableInsert,
		ParametersExpected:   -99,
		ParameterDescription: "table-name [column=value...]",
		Value: []cli.Option{
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "File name containing JSON row info",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "update",
		Description:          "Update rows to a table",
		OptionType:           cli.Subcommand,
		Action:               commands.TableUpdate,
		ParametersExpected:   -99,
		ParameterDescription: "table-name column=value [column=value...]",
		Value: []cli.Option{
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "Filter for rows to update. If not specified, all rows are updated",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:             "create",
		Description:          "Create a new table",
		OptionType:           cli.Subcommand,
		Action:               commands.TableCreate,
		ParametersExpected:   -999,
		ParameterDescription: "table-name column:type [column:type...]",
		Value: []cli.Option{
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "File name containing JSON column info",
				OptionType:  cli.StringListType,
			},
		},
	},
}

var ServerDeleteGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "Username to delete",
		OptionType:  cli.StringType,
	},
}
var ServerUserGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "Username to create or update",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "password",
		ShortName:   "p",
		Description: "Password to assign to user",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "permissions",
		Description: "Permissions to grant to user",
		OptionType:  cli.StringListType,
	},
}

// UserGrammar contains the grammar for SERVER USERS subcommands.
var UserGrammar = []cli.Option{
	{
		LongName:    "set",
		Description: "Create or update user information",
		OptionType:  cli.Subcommand,
		Action:      commands.AddUser,
		Value:       ServerUserGrammar,
	},
	{
		LongName:    "delete",
		Description: "Delete a user from the server's user database",
		OptionType:  cli.Subcommand,
		Action:      commands.DeleteUser,
		Value:       ServerDeleteGrammar,
	},
	{
		LongName:    "list",
		Description: "List users in the server's user database",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
	},
}

// CachesGrammar defines the grammar for the SERVER CACHES subcommands.
var CachesGrammar = []cli.Option{
	{
		LongName:    "flush",
		Description: "Flush service caches",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushServerCaches,
		Value:       ServerStateGrammar,
	},
	{
		LongName:    "list",
		Aliases:     []string{"show"},
		Description: "List service caches",
		OptionType:  cli.Subcommand,
		Action:      commands.ListServerCaches,
		Value:       ServerStateGrammar,
	},
	{
		LongName:             "set-size",
		Description:          "Set the server cache size",
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
		Description: "List of loggers to enable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "disable",
		Aliases:     []string{"clear"},
		Description: "List of loggers to disable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "file",
		ShortName:   "f",
		Description: "Show only the active log file name",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "status",
		Description: "Display the state of each logger",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "limit",
		ShortName:   "l",
		Description: "Limit display to this many lines of text",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "keep",
		Description: "Specify how many log files to keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "Specify port number of server",
	},
}

// ServerGrammar contains the grammar of SERVER subcommands.
var ServerGrammar = []cli.Option{
	{
		LongName:             "logging",
		Aliases:              []string{"logger", "log", "logs"},
		Description:          "Display or configure server logging",
		OptionType:           cli.Subcommand,
		Value:                LoggingGrammar,
		ParametersExpected:   -1,
		ParameterDescription: "address:port",
		Action:               commands.Logging,
	},
	{
		LongName:    "users",
		Aliases:     []string{"user"},
		Description: "Manage server user database",
		OptionType:  cli.Subcommand,
		Value:       UserGrammar,
	},
	{
		LongName:    "caches",
		Aliases:     []string{"cache"},
		Description: "Manage server caches",
		OptionType:  cli.Subcommand,
		Value:       CachesGrammar,
	},
	{
		LongName:    "run",
		Description: "Run the rest server",
		OptionType:  cli.Subcommand,
		Action:      commands.RunServer,
		// Run and Start share a grammar, but Run has additional option
		Value: append(ServerRunGrammar, cli.Option{
			LongName:    "debug",
			ShortName:   "d",
			Description: "Service endpoint to debug",
			OptionType:  cli.StringType,
		}),
	},
	{
		LongName:    "restart",
		Description: "Restart an existing server",
		OptionType:  cli.Subcommand,
		Action:      commands.Restart,
		Value:       ServerStateGrammar,
	},
	{
		LongName:             "status",
		Description:          "Display server status",
		OptionType:           cli.Subcommand,
		Action:               commands.Status,
		ParametersExpected:   -1,
		ParameterDescription: "address[:port]",
		Value:                ServerStateGrammar,
	},
	{
		LongName:    "start",
		Description: "Start the rest server as a detached process",
		OptionType:  cli.Subcommand,
		Action:      commands.Start,
		Value:       ServerRunGrammar,
	},
	{
		LongName:    "stop",
		Description: "Stop the detached rest server",
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
		Description:         "Specify port number of server to stop",
		EnvironmentVariable: "EGO_PORT",
	},
}

// ServerStateGrammar  is a common sub-grammar for specifying a port and/or UUID.
var ServerStateGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "Specify port number of server",
		EnvironmentVariable: "EGO_PORT",
	},
	{
		LongName:    "session-uuid",
		Description: "Sets the optional session UUID value",
		OptionType:  cli.UUIDType,
	},
}

// ServerRunGrammar handles command line options for the server subcommand.
var ServerRunGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "Specify port number to listen on",
		EnvironmentVariable: "EGO_PORT",
	},
	{
		LongName:            "not-secure",
		ShortName:           "k",
		OptionType:          cli.BooleanType,
		Description:         "If set, use HTTP instead of HTTPS",
		EnvironmentVariable: "EGO_INSECURE",
	},
	{
		LongName:    "is-detached",
		OptionType:  cli.BooleanType,
		Description: "If set, server assumes it is already detached",
		Private:     true,
	},
	{
		LongName:    "force",
		ShortName:   "f",
		OptionType:  cli.BooleanType,
		Description: "If set, override existing PID file",
		Private:     true,
	},
	{
		LongName:    "log",
		Description: "File path of server log",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "keep-logs",
		Description: "The number of log files to keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "sandbox-path",
		Description: "File path of sandboxed area for file I/O",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "no-log",
		Description: "Suppress server log",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "Display trace of bytecode execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
	{
		LongName:    "full-symbol-scope",
		Description: "Blocks can access any symbol in call stack",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:            "static-types",
		Description:         "Enforce static typing on program execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_STATIC_TYPES",
	},
	{
		LongName:            "realm",
		ShortName:           "r",
		Description:         "Name of authentication realm",
		OptionType:          cli.StringType,
		EnvironmentVariable: "EGO_REALM",
	},
	{
		LongName:    "cache-size",
		Description: "Number of service programs to cache in memory",
		OptionType:  cli.IntType,
	},
	{
		LongName:            "users",
		Aliases:             []string{"user-database"},
		ShortName:           "u",
		Description:         "File with authentication JSON data",
		OptionType:          cli.StringType,
		EnvironmentVariable: "EGO_USERS",
	},
	{
		LongName:    "superuser",
		Description: "Designate this user as a super-user with ROOT privileges",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "code",
		ShortName:   "c",
		Description: "Enable /code endpoint",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "session-uuid",
		Description: "Sets the optional session UUID value",
		OptionType:  cli.UUIDType,
	},
}

// RunGrammar handles the command line options.
var RunGrammar = []cli.Option{
	{
		LongName:            "disassemble",
		Aliases:             []string{"disasm"},
		Description:         "Display a disassembly of the bytecode before execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_DISASM",
	},
	{
		LongName:    "log",
		Description: "Direct log output to this file instead of stdout",
		OptionType:  cli.StringType,
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "Display trace of bytecode execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
	{
		LongName:            "static-types",
		Description:         "Enforce static typing on program execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_STATIC_TYPES",
	},
	{
		LongName:    "debug",
		ShortName:   "d",
		Description: "Run with interactive debugger",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "full-symbol-scope",
		Description: "Blocks can access any symbol in call stack",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "symbols",
		ShortName:   "s",
		Description: "Display symbol table",
		OptionType:  cli.BooleanType,
		Private:     true,
	},
	{
		LongName:    "symbol-table-size",
		Description: "Maximum number of symbols at any given scope",
		OptionType:  cli.IntType,
	},
	{
		LongName:            "auto-import",
		Description:         "Override auto-import configuration setting",
		OptionType:          cli.BooleanValueType,
		EnvironmentVariable: "EGO_AUTOIMPORT",
	},
	{
		LongName:    "entry-point",
		ShortName:   "e",
		Description: "Name of entrypoint function (defaults to main)",
		OptionType:  cli.StringType,
	},
}
