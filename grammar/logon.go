package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/defs"
)

// LogonGrammar describes the login subcommand options.
var LogonVerbGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		OptionType:  cli.StringType,
		Description: "username",
		EnvVar:      defs.EgoUserEnv,
	},
	{
		LongName:    "password",
		ShortName:   "p",
		OptionType:  cli.StringType,
		Description: "password",
		EnvVar:      defs.EgoPasswordEnv,
	},
	{
		LongName:    "logon-server",
		ShortName:   "l",
		Aliases:     []string{"server"},
		OptionType:  cli.StringType,
		Description: "logon.server",
		EnvVar:      defs.EgoLogonServerEnv,
	},
	{
		LongName:    "expiration",
		ShortName:   "e",
		Aliases:     []string{"expires"},
		OptionType:  cli.StringType,
		Description: "logon.expiration",
	},
}
