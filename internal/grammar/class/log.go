package class

import "github.com/tucats/ego/internal/cli/cli"

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
		LongName:    "archive",
		Description: "server.logging.archive",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "since",
		Description: "server.logging.since",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "until",
		Description: "server.logging.until",
		OptionType:  cli.StringType,
	},
	{
		LongName:    "server-id",
		Aliases:     []string{"server-uuid", "uuid", "id"},
		Description: "server.logging.serverid",
		OptionType:  cli.StringType,
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
