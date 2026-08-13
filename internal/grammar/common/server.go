package common

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

var ServerShowUserGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		ShortName:   "u",
		Description: "server.user.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
}

var ServerUserGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		ShortName:   "u",
		Description: "server.user.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
	{
		LongName:    defs.PasswordOption,
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

var ServerDeleteUserGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		ShortName:   "u",
		Description: "server.delete.user",
		OptionType:  cli.StringType,
		Private:     true,
	},
}

var ServerListUsersGrammar = []cli.Option{
	{
		LongName:    "id",
		ShortName:   "i",
		Description: "server.show.id",
		OptionType:  cli.BooleanType,
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
		EnvVar:      defs.EgoInsecurePortEnv,
	},
	{
		LongName:    "not-secure",
		ShortName:   "k",
		OptionType:  cli.BooleanType,
		Description: "server.run.not.secure",
		EnvVar:      defs.EgoInsecureEnv,
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
		EnvVar:      defs.EgoTraceEnv,
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
		EnvVar:      defs.EgoTypesEnv,
	},
	{
		LongName:    "realm",
		ShortName:   "r",
		Description: "server.run.realm",
		OptionType:  cli.StringType,
		EnvVar:      defs.EgoRealmEnv,
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
		EnvVar:      defs.EgoUsersEnv,
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
	{
		LongName:    "cluster",
		ShortName:   "C",
		Description: "opt.server.cluster",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "oauth-server",
		Description: "server.run.oauth.server",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    defs.VerboseOption,
		ShortName:   "v",
		OptionType:  cli.BooleanType,
		Description: "verbose",
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
	{
		LongName:    defs.VerboseOption,
		ShortName:   "v",
		OptionType:  cli.BooleanType,
		Description: "verbose",
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
