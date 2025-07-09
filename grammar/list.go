package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/commands"
)

var ListVerbGrammar = []cli.Option{
	{
		LongName:    "profiles",
		Aliases:     []string{"profile"},
		Description: "ego.config.list",
		Action:      config.ListAction,
		OptionType:  cli.Subcommand,
		Value: []cli.Option{
			{
				LongName:    "version",
				ShortName:   "v",
				OptionType:  cli.BooleanType,
				Description: "config.version",
			},
		},
	},
	{
		LongName:    "dsns",
		Aliases:     []string{"dsn", "data-source", "data-sources"},
		Description: "ego.dsns.list",
		OptionType:  cli.Subcommand,
		Action:      commands.DSNSList,
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
	{
		LongName:      "tables",
		Aliases:       []string{"table"},
		Description:   "ego.table.list",
		OptionType:    cli.Subcommand,
		Action:        commands.TableList,
		ExpectedParms: -1,
		ParmDesc:      "dsn",
		Value: []cli.Option{
			{
				LongName:    "dsn",
				ShortName:   "d",
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
		LongName:    "users",
		Description: "ego.server.user.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
		Value:       ServerListUsersGrammar,
	},
}
