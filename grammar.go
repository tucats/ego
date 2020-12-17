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
		Action:      commands.Server,
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

// ServerGrammar handles command line options for the server subcommand
var ServerGrammar = []cli.Option{
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
