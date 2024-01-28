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
		LongName:   "service",
		OptionType: cli.StringType,
		Action:     app.ChildService,
		Private:    true,
	},
	{
		LongName:    "dsns",
		Aliases:     []string{"dsn"},
		Description: "ego.dsns",
		OptionType:  cli.Subcommand,
		Value:       DSNSGrammar,
	},
	{
		LongName:      "sql",
		Description:   "ego.sql",
		OptionType:    cli.Subcommand,
		Action:        commands.TableSQL,
		ExpectedParms: -99,
		ParmDesc:      "sql-text",
		Value:         SQLGrammar,
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
		LongName:      "run",
		Description:   "ego.run",
		OptionType:    cli.Subcommand,
		Action:        commands.RunAction,
		Value:         RunGrammar,
		ExpectedParms: -99,
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
		Value:         TestGrammar,
		Action:        commands.TestAction,
		ExpectedParms: -99,
		ParmDesc:      "parm.file.or.path",
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

// DSNSGrammar specifies the command line options for the "dsns" Ego command.
var DSNSGrammar = []cli.Option{
	{
		LongName:      "show",
		Description:   "ego.dsns.show",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNShow,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
	},
	{
		LongName:      "delete",
		Description:   "ego.dsns.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSDelete,
		ParmDesc:      "dsn-name[ ds-name...]",
		ExpectedParms: -99,
		MinParams:     1,
	},
	{
		LongName:      "add",
		Description:   "ego.dsns.add",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSAdd,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "type",
				ShortName:   "t",
				Aliases:     []string{"provider"},
				Description: "dsns.add.type",
				OptionType:  cli.KeywordType,
				Keywords:    []string{"sqlite3", "postgres"},
				Required:    true,
			},
			{
				LongName:    "database",
				ShortName:   "d",
				Aliases:     []string{"db"},
				Description: "dsns.add.database",
				OptionType:  cli.StringType,
				Required:    true,
			},
			{
				LongName:    "host",
				Description: "dsns.add.host",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "port",
				Description: "dsns.add.port",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "username",
				Aliases:     []string{"user"},
				ShortName:   "u",
				Description: "dsns.add.username",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "password",
				Aliases:     []string{"pw"},
				ShortName:   "p",
				Description: "dsns.add.password",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "schema",
				Aliases:     []string{"user"},
				Description: "dsns.add.schema",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "secured",
				Aliases:     []string{"secure"},
				Description: "dsns.add.secured",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "native",
				Description: "dsns.add.native",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:      "grant",
		Description:   "ego.dsns.grant",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSGrant,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "username",
				Aliases:     []string{"user"},
				ShortName:   "u",
				Description: "dsns.grant.username",
				OptionType:  cli.StringType,
				Required:    true,
			},
			{
				LongName:    "permissions",
				Aliases:     []string{"perms"},
				ShortName:   "p",
				Description: "dsns.grant.permissions",
				OptionType:  cli.StringListType,
				Required:    true,
			},
		},
	},
	{
		LongName:      "revoke",
		Description:   "ego.dsns.revoke",
		OptionType:    cli.Subcommand,
		Action:        commands.DSNSRevoke,
		ParmDesc:      "dsn-name",
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "username",
				Aliases:     []string{"user"},
				ShortName:   "u",
				Description: "dsns.revoke.username",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "permissions",
				Aliases:     []string{"perms"},
				ShortName:   "p",
				Description: "dsns.revoke.permissions",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:    "list",
		Description: "ego.dsns.list",
		OptionType:  cli.Subcommand,
		Action:      commands.DSNSList,
		DefaultVerb: true,
		Value: []cli.Option{
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
		},
	},
}

// TableGrammar specifies the command line options for the "tables" Ego command.
var TableGrammar = []cli.Option{
	{
		LongName:      "sql",
		Description:   "ego.table.sql",
		OptionType:    cli.Subcommand,
		Action:        commands.TableSQL,
		ExpectedParms: -99,
		ParmDesc:      "parm.sql.text",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "sql-file",
				ShortName:   "f",
				Aliases:     []string{"file"},
				Description: "sql.file",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "sql.row.ids",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-numbers",
				ShortName:   "n",
				Aliases:     []string{"row-number", "row"},
				Description: "sql.row.numbers",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:      "permissions",
		Aliases:       []string{"perms"},
		Description:   "ego.table.permissions",
		OptionType:    cli.Subcommand,
		Action:        commands.TablePermissions,
		ExpectedParms: 0,
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "table.permissions.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "show-permission",
		Aliases:       []string{"permission", "perm"},
		Description:   "ego.table.permission",
		OptionType:    cli.Subcommand,
		Action:        commands.TableShowPermission,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "table.permission.user",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "grant",
		Aliases:       []string{"permission"},
		Description:   "ego.table.grant",
		OptionType:    cli.Subcommand,
		Action:        commands.TableGrant,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    "permission",
				Aliases:     []string{"permission", "permissions", "perms", "perm"},
				ShortName:   "p",
				Description: "table.grant.permission",
				OptionType:  cli.StringListType,
				Required:    true,
			},
			{
				LongName:    "user",
				ShortName:   "u",
				Description: "table.grant.user",
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
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "no-row-counts",
				Description: "table.list.no.row.counts",
				OptionType:  cli.BooleanType,
			},
		},
	},
	{
		LongName:      "show-table",
		Aliases:       []string{"show", "metadata", "columns"},
		Description:   "ego.table.show",
		OptionType:    cli.Subcommand,
		Action:        commands.TableShow,
		ExpectedParms: 1,
		ParmDesc:      "parm.table.name",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "drop",
		Description:   "ego.table.drop",
		OptionType:    cli.Subcommand,
		Action:        commands.TableDrop,
		ExpectedParms: -99,
		ParmDesc:      "table-name [table-name...]",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
		},
	},
	{
		LongName:      "read",
		Aliases:       []string{"select", "print", "get", "show-contents", "contents"},
		Description:   "ego.table.read",
		OptionType:    cli.Subcommand,
		Action:        commands.TableContents,
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "row-ids",
				ShortName:   "i",
				Aliases:     []string{"ids"},
				Description: "table.read.row.ids",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "row-numbers",
				ShortName:   "n",
				Aliases:     []string{"row-number", "row"},
				Description: "table.read.row.numbers",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "columns",
				ShortName:   "c",
				Aliases:     []string{"column"},
				Description: "table.read.columns",
				OptionType:  cli.StringListType,
			},

			{
				LongName:    "order-by",
				ShortName:   "o",
				Aliases:     []string{"sort", "order"},
				Description: "table.read.order.by",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "filter",
				OptionType:  cli.StringListType,
			},
			{
				LongName:    "limit",
				Aliases:     []string{"count"},
				Description: "limit",
				OptionType:  cli.IntType,
			},
			{
				LongName:    "start",
				Aliases:     []string{"offset"},
				Description: "start",
				OptionType:  cli.IntType,
			},
		},
	},
	{
		LongName:      "delete",
		Description:   "ego.table.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.TableDelete,
		ExpectedParms: 1,
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "filter",
				ShortName:   "f",
				Aliases:     []string{"where"},
				Description: "table.delete.filter",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:      "insert",
		Aliases:       []string{"write", "append"},
		Description:   "ego.table.insert",
		OptionType:    cli.Subcommand,
		Action:        commands.TableInsert,
		ExpectedParms: -99,
		ParmDesc:      "parm.table.insert",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "table.insert.file",
				OptionType:  cli.StringListType,
			},
		},
	},
	{
		LongName:      "update",
		Description:   "ego.table.update",
		OptionType:    cli.Subcommand,
		Action:        commands.TableUpdate,
		ExpectedParms: -99,
		ParmDesc:      "parm.table.update",
		Value: []cli.Option{
			{
				LongName:    "dsn",
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
		LongName:      "create",
		Description:   "ego.table.create",
		OptionType:    cli.Subcommand,
		Action:        commands.TableCreate,
		ExpectedParms: -999,
		ParmDesc:      "parm.table.create",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				Aliases:     []string{"ds", "datasource"},
				Description: "dsn",
				OptionType:  cli.StringType,
			},
			{
				LongName:    "file",
				Aliases:     []string{"json-file", "json"},
				ShortName:   "f",
				Description: "table.create.file",
				OptionType:  cli.StringListType,
			},
		},
	},
}

var ServerShowUserGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "server.user.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
}

var ServerDeleteUserGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "server.delete.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
}

var ServerUserGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		Description: "server.user.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
	{
		LongName:    "password",
		ShortName:   "p",
		Description: "server.user.pass",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "permissions",
		Aliases:     []string{"permission"},
		Description: "server.user.perms",
		OptionType:  cli.StringListType,
	},
}

var ServerListUsersGrammar = []cli.Option{
	{
		LongName:    "id",
		ShortName:   "i",
		Description: "server.show.id",
		OptionType:  cli.BooleanType,
	},
}

var ServerMemoryGrammar = []cli.Option{
	{
		LongName:    "megabytes",
		ShortName:   "m",
		Aliases:     []string{"mb"},
		Description: "server.memory.megabytes",
		OptionType:  cli.BooleanType,
	},
}

// UserGrammar contains the grammar for SERVER USERS subcommands.
var UserGrammar = []cli.Option{
	{
		LongName:      "create",
		Description:   "ego.server.user.create",
		Aliases:       []string{"add"},
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.AddUser,
		Value:         ServerUserGrammar,
	},
	{
		LongName:      "update",
		Description:   "ego.server.user.update",
		Aliases:       []string{"modify", "alter"},
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.UpdateUser,
		Value:         ServerUserGrammar,
	},
	{
		LongName:      "show",
		Description:   "ego.server.user.show",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.ShowUser,
		Value:         ServerShowUserGrammar,
	},
	{
		LongName:      "delete",
		Description:   "ego.server.user.delete",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.DeleteUser,
		Value:         ServerDeleteUserGrammar,
	},
	{
		LongName:    "list",
		Description: "ego.server.user.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
		Value:       ServerListUsersGrammar,
		DefaultVerb: true,
	},
}

// CachesGrammar defines the grammar for the SERVER CACHES subcommands.
var CachesGrammar = []cli.Option{
	{
		LongName:    "flush",
		Description: "ego.server.cache.flush",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushCaches,
		Value:       ServerStateGrammar,
	},
	{
		LongName:    "show",
		Aliases:     []string{"list"},
		Description: "ego.server.cache.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ShowCaches,
		Value: []cli.Option{
			{
				LongName:    "services",
				Aliases:     []string{"service"},
				ShortName:   "s",
				Description: "cache.list.services",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "assets",
				Aliases:     []string{"asset"},
				ShortName:   "a",
				Description: "cache.list.assets",
				OptionType:  cli.BooleanType,
			},
			{
				LongName:    "order-by",
				Aliases:     []string{"sort", "order"},
				Description: "cache.list.order.by",
				OptionType:  cli.KeywordType,
				Keywords:    []string{"url", "count", "last-used"},
			},
		},
		DefaultVerb: true,
	},
	{
		LongName:      "set-size",
		Description:   "ego.server.cache.set.size",
		ExpectedParms: 1,
		ParmDesc:      "limit",
		OptionType:    cli.Subcommand,
		Action:        commands.SetCacheSize,
		Value:         ServerStateGrammar,
	},
}

// LoggingGrammar is the ego server logging grammar.
var LoggingGrammar = []cli.Option{
	{
		LongName:    "enable",
		Aliases:     []string{"set"},
		Description: "server.logging.enable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "disable",
		Aliases:     []string{"clear"},
		Description: "server.logging.disable",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "file",
		ShortName:   "f",
		Description: "server.logging.file",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "status",
		ShortName:   "s",
		Description: "server.logging.status",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "limit",
		ShortName:   "l",
		Description: "limit",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "session",
		Description: "server.logging.session",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "keep",
		Description: "server.logging.keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "port",
	},
}

// ServerGrammar contains the grammar of SERVER subcommands.
var ServerGrammar = []cli.Option{
	{
		LongName:      "logging",
		Aliases:       []string{"logger", "log", "logs"},
		Description:   "ego.server.logging",
		OptionType:    cli.Subcommand,
		Value:         LoggingGrammar,
		ExpectedParms: cli.Variable,
		ParmDesc:      "parm.address.port",
		Action:        commands.Logging,
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
		LongName:    "memory",
		Description: "ego.server.memory",
		OptionType:  cli.Subcommand,
		Action:      commands.ServerMemory,
		Value:       ServerMemoryGrammar,
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
		Action:      commands.Server,
		// Run and Start share a grammar, but Run has additional options
		Value: append(ServerRunGrammar, []cli.Option{
			{
				LongName:    "debug-endpoint",
				ShortName:   "d",
				Description: "server.run.debug",
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
		Unsupported: []string{"windows"},
	},
	{
		LongName:      "status",
		Description:   "ego.server.status",
		OptionType:    cli.Subcommand,
		Action:        commands.Status,
		ExpectedParms: cli.Variable,
		ParmDesc:      "address.port",
		Value:         ServerStateGrammar,
	},
	{
		LongName:    "start",
		Description: "ego.server.start",
		OptionType:  cli.Subcommand,
		Action:      commands.Start,
		Value:       ServerRunGrammar,
		Unsupported: []string{"windows"},
	},
	{
		LongName:    "stop",
		Description: "ego.server.stop",
		OptionType:  cli.Subcommand,
		Action:      commands.Stop,
		Value:       ServerStopGrammar,
		Unsupported: []string{"windows"},
	},
}

// ServerStopGrammar handles command line options for the server subcommand.
var ServerStopGrammar = []cli.Option{
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "port",
		EnvVar:      defs.EgoPortEnv,
	},
}

// ServerStateGrammar  is a common sub-grammar for specifying a port.
var ServerStateGrammar = []cli.Option{
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "port",
		EnvVar:      defs.EgoPortEnv,
	},
	{
		LongName:    "local",
		ShortName:   "l",
		Aliases:     []string{"pid", "pidfile"},
		OptionType:  cli.BooleanType,
		Description: "local",
	},
}

// ServerRunGrammar handles command line options for the server subcommand.
var ServerRunGrammar = []cli.Option{
	{
		LongName:    "child-services",
		Description: "server.run.child.services",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "auth-server",
		Aliases:     []string{"auth"},
		Description: "server.auth.server",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "port",
		ShortName:   "p",
		OptionType:  cli.IntType,
		Description: "port",
		EnvVar:      defs.EgoPortEnv,
	},
	{
		LongName:    "insecure-port",
		OptionType:  cli.IntType,
		Description: "insecure.port",
		EnvVar:      "EGO_INSECURE_PORT",
	},
	{
		LongName:    "not-secure",
		ShortName:   "k",
		OptionType:  cli.BooleanType,
		Description: "server.run.not.secure",
		EnvVar:      "EGO_INSECURE",
	},
	{
		LongName:    "cert-dir",
		Aliases:     []string{"certs"},
		Description: "server.run.certs",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "is-detached",
		OptionType:  cli.BooleanType,
		Description: "server.run.is.detached",
		Private:     true,
	},
	{
		LongName:    "force",
		ShortName:   "f",
		OptionType:  cli.BooleanType,
		Description: "server.run.force",
		Private:     true,
	},
	{
		LongName:    "log-file",
		Description: "server.run.log",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "keep-logs",
		Description: "server.run.keep",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "sandbox-path",
		Description: "server.run.sandbox",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "no-log",
		Description: "server.run.no.log",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "trace",
		ShortName:   "t",
		Description: "trace",
		OptionType:  cli.BooleanType,
		EnvVar:      "EGO_TRACE",
	},
	{
		LongName:    "full-symbol-scope",
		Description: "scope",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "symbol-allocation",
		Description: "symbol.allocation",
		OptionType:  cli.IntType,
	},
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "server.run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      "EGO_TYPING",
	},
	{
		LongName:    "realm",
		ShortName:   "r",
		Description: "server.run.realm",
		OptionType:  cli.StringType,
		EnvVar:      "EGO_REALM",
	},
	{
		LongName:    "cache-size",
		Description: "server.run.cache",
		OptionType:  cli.IntType,
	},
	{
		LongName:    "users",
		Aliases:     []string{"user-database"},
		ShortName:   "u",
		Description: "server.run.users",
		OptionType:  cli.StringType,
		EnvVar:      "EGO_USERS",
	},
	{
		LongName:    "superuser",
		Description: "server.run.superuser",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "default-credential",
		Description: "server.run.default-credential",
		OptionType:  cli.StringType,
		Private:     true,
	},
	{
		LongName:    "session-uuid",
		Description: "server.run.uuid",
		OptionType:  cli.UUIDType,
	},
}

// RunGrammar handles the command line options.
var RunGrammar = []cli.Option{
	{
		LongName:    "disassemble",
		Aliases:     []string{"disasm"},
		Description: "run.disasm",
		OptionType:  cli.BooleanType,
		EnvVar:      "EGO_DISASM",
	},
	{
		LongName:    "project",
		ShortName:   "p",
		Description: "run.project",
		OptionType:  cli.BooleanType,
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
		EnvVar:      "EGO_TRACE",
	},
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      "EGO_TYPING",
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
		OptionType:  cli.BooleanType,
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
		EnvVar:      "EGO_AUTOIMPORT",
	},
	{
		LongName:    "entry-point",
		ShortName:   "e",
		Description: "run.entry.point",
		OptionType:  cli.StringType,
	},
}

// TestGrammar handles the command line options.
var TestGrammar = []cli.Option{
	{
		LongName:    defs.TypingOption,
		Aliases:     []string{"typing"},
		Description: "run.static",
		OptionType:  cli.KeywordType,
		Keywords:    []string{defs.Strict, defs.Relaxed, defs.Dynamic},
		EnvVar:      "EGO_TYPING",
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
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "trace",
		ShortName:   "t",
		Description: "trace",
		OptionType:  cli.BooleanType,
		EnvVar:      "EGO_TRACE",
	},
}
