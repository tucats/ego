package main

import (
	"github.com/tucats/ego/commands"
	"github.com/tucats/gopackages/app-cli/cli"
)

// EgoGrammar handles the command line options
var EgoGrammar = []cli.Option{
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
		Description: "Accept REST calls",
		OptionType:  cli.Subcommand,
		Value:       ServerGrammar,
	},
	{
		LongName:    "logon",
		Description: "Request logon token",
		OptionType:  cli.Subcommand,
		Action:      commands.Logon,
		Value:       LogonGrammar,
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

// LogonGrammar describes the login subcommand
var LogonGrammar = []cli.Option{
	{
		LongName:            "username",
		ShortName:           "u",
		OptionType:          cli.StringType,
		Description:         "Username for login",
		EnvironmentVariable: "EGO_USERNAME",
	},
	{
		LongName:            "password",
		ShortName:           "p",
		OptionType:          cli.StringType,
		Description:         "Password for login",
		EnvironmentVariable: "EGO_PASSWORD",
	},
	{
		LongName:            "logon-server",
		ShortName:           "l",
		OptionType:          cli.StringType,
		Description:         "URL of logon server",
		EnvironmentVariable: "EGO_LOGON_SERVER",
	},
	{
		LongName:   "hash",
		ShortName:  "h",
		OptionType: cli.BooleanType,
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

// ServerGrammar contains the grammar of SERVER subcommands
var ServerGrammar = []cli.Option{

	{
		LongName:    "add-user",
		Description: "Add a new user to the server's user database",
		OptionType:  cli.Subcommand,
		Action:      commands.AddUser,
		Value:       ServerUserGrammar,
	},
	{
		LongName:    "delete-user",
		Description: "Delete a user to the server's user database",
		OptionType:  cli.Subcommand,
		Action:      commands.DeleteUser,
		Value:       ServerDeleteGrammar,
	},
	{
		LongName:    "list-users",
		Description: "List users in the server's user database",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
	},
	{
		LongName:    "run",
		Description: "Run the rest server",
		OptionType:  cli.Subcommand,
		Action:      commands.Server,
		Value:       ServerRunGrammar,
	},
	{
		LongName:    "flush-caches",
		Description: "Flush service caches",
		OptionType:  cli.Subcommand,
		Action:      commands.FlushServerCaches,
		Value:       ServerStateGrammar,
	},

	{
		LongName:    "restart",
		Description: "Restart an existing server",
		OptionType:  cli.Subcommand,
		Action:      commands.Restart,
		Value:       ServerStateGrammar,
	},
	{
		LongName:    "status",
		Description: "Display server status",
		OptionType:  cli.Subcommand,
		Action:      commands.Status,
		Value:       ServerStateGrammar,
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

// ServerStopGrammar handles command line options for the server subcommand
var ServerStopGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "Specify port number of server to stop",
		EnvironmentVariable: "EGO_PORT",
	},
}

var ServerStateGrammar = []cli.Option{
	{
		LongName:            "port",
		ShortName:           "p",
		OptionType:          cli.IntType,
		Description:         "Specify port number of server",
		EnvironmentVariable: "EGO_PORT",
	},
}

// ServerRunGrammar handles command line options for the server subcommand
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
}

// RunGrammar handles the command line options
var RunGrammar = []cli.Option{
	{
		LongName:            "disassemble",
		ShortName:           "d",
		Description:         "Display a disassembly of the bytecode before execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_DISASM",
	},
	{
		LongName:            "trace",
		ShortName:           "t",
		Description:         "Display trace of bytecode execution",
		OptionType:          cli.BooleanType,
		EnvironmentVariable: "EGO_TRACE",
	},
	{
		LongName:    "symbols",
		ShortName:   "s",
		Description: "Display symbol table",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:            "auto-import",
		Description:         "Override auto-import profile setting",
		OptionType:          cli.BooleanValueType,
		EnvironmentVariable: "EGO_AUTOIMPORT",
	},
	{
		LongName:    "environment",
		ShortName:   "e",
		Description: "Automatically add environment vars as symbols",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "source-tracing",
		ShortName:   "x",
		Description: "Print source lines as they are executed",
		OptionType:  cli.BooleanType,
	},
}
