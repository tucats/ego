package config

import "github.com/tucats/ego/app-cli/cli"

// Grammar describes the "config" subcommands.
var Grammar = []cli.Option{
	{
		LongName:    "list",
		Description: "ego.config.list",
		Action:      ListAction,
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
		LongName:      "show",
		Description:   "ego.config.show",
		Action:        ShowAction,
		ParmDesc:      "parm.key",
		ExpectedParms: -1,
		OptionType:    cli.Subcommand,
		DefaultVerb:   true,
		Value: []cli.Option{
			{
				LongName:    "version",
				ShortName:   "v",
				OptionType:  cli.BooleanType,
				Description: "config.version",
			},
			{
				LongName:   "verbose",
				OptionType: cli.BooleanType,
				Private:    true,
			},
		},
	},
	{
		LongName:      "set-output",
		OptionType:    cli.Subcommand,
		Description:   "ego.config.set.output",
		ParmDesc:      "<type>",
		Action:        SetOutputAction,
		ExpectedParms: 1,
	},
	{
		LongName:      "set-description",
		OptionType:    cli.Subcommand,
		Description:   "ego.config.set.description",
		ParmDesc:      "<text>",
		ExpectedParms: 1,
		Action:        SetDescriptionAction,
	},
	{
		LongName:      "delete",
		Aliases:       []string{"unset"},
		OptionType:    cli.Subcommand,
		Description:   "ego.config.delete",
		Action:        DeleteAction,
		ExpectedParms: 1,
		ParmDesc:      "parm.key",
		Value: []cli.Option{
			{
				LongName:    "force",
				ShortName:   "f",
				OptionType:  cli.BooleanType,
				Description: "config.force",
			},
		},
	},
	{
		LongName:      "remove",
		OptionType:    cli.Subcommand,
		Description:   "ego.config.remove",
		Action:        DeleteProfileAction,
		ExpectedParms: 1,
		ParmDesc:      "parm.name",
	},
	{
		LongName:      "set",
		Description:   "ego.config.set",
		Action:        SetAction,
		OptionType:    cli.Subcommand,
		ExpectedParms: 1,
		ParmDesc:      "parm.config.key.value",
	},
	{
		LongName:    "describe",
		Description: "ego.config.describe",
		Action:      DescribeAction,
		OptionType:  cli.Subcommand,
		Value: []cli.Option{
			{
				LongName:    "verbose",
				ShortName:   "v",
				OptionType:  cli.BooleanType,
				Description: "config.verbose",
			},
		},
	},
}
