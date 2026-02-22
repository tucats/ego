package grammar

import (
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/commands"
)

var RestGrammar = []cli.Option{
	{
		LongName:    "get",
		Description: "ego.server.rest.get",
		OptionType:  cli.Subcommand,
		Action:      commands.RestGet,
		Value: append(RestMethodGrammar, cli.Option{
			LongName:    "accepts",
			Aliases:     []string{"accept", "media"},
			Description: "server.rest.method.accepts",
			OptionType:  cli.StringListType,
			Keywords:    []string{"json", "text", "application/json", "application/text"},
		}),
		ParmDesc:      "parm.server.rest.method.url",
		MinParams:     1,
		ExpectedParms: 1,
		Prompts:       []string{"prompt.rest.url"},
	},
	{
		LongName:    "post",
		Description: "ego.server.rest.post",
		OptionType:  cli.Subcommand,
		Action:      commands.RestPost,
		Value: append(RestMethodGrammar, []cli.Option{
			{
				LongName:    "field",
				ShortName:   "f",
				Description: "server.rest.method.field",
				OptionType:  cli.StringListType,
				Excludes:    []string{"data"},
			},
			{
				LongName:    "data",
				ShortName:   "d",
				Description: "server.rest.method.data",
				OptionType:  cli.StringType,
				Excludes:    []string{"field"},
			},
		}...),
		ParmDesc:      "parm.server.rest.method.url",
		MinParams:     1,
		ExpectedParms: 1,
		Prompts:       []string{"prompt.rest.url"},
	},
	{
		LongName:    "put",
		Description: "ego.server.rest.put",
		OptionType:  cli.Subcommand,
		Action:      commands.RestPut,
		Value: append(RestMethodGrammar, []cli.Option{
			{
				LongName:    "field",
				ShortName:   "f",
				Description: "server.rest.method.field",
				OptionType:  cli.StringListType,
				Excludes:    []string{"data"},
			},
			{
				LongName:    "data",
				ShortName:   "d",
				Description: "server.rest.method.data",
				OptionType:  cli.StringType,
				Excludes:    []string{"field"},
			},
		}...),
		ParmDesc:      "parm.server.rest.method.url",
		MinParams:     1,
		ExpectedParms: 1,
		Prompts:       []string{"prompt.rest.url"},
	},
	{
		LongName:      "delete",
		Description:   "ego.server.rest.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.RestDelete,
		Value:         RestMethodGrammar,
		ParmDesc:      "server.rest.method.url",
		MinParams:     1,
		ExpectedParms: 1,
		Prompts:       []string{"prompt.rest.url"},
	},
	{
		LongName:    "patch",
		Description: "ego.server.rest.patch",
		OptionType:  cli.Subcommand,
		Action:      commands.RestPatch,
		Value: append(RestMethodGrammar, []cli.Option{
			{
				LongName:    "field",
				ShortName:   "f",
				Description: "server.rest.method.field",
				OptionType:  cli.StringListType,
				Excludes:    []string{"data"},
			},
			{
				LongName:    "data",
				ShortName:   "d",
				Description: "server.rest.method.data",
				OptionType:  cli.StringType,
				Excludes:    []string{"field"},
			},
		}...),
		ParmDesc:      "parm.server.rest.method.url",
		MinParams:     1,
		ExpectedParms: 1,
		Prompts:       []string{"prompt.rest.url"},
	},
}

var RestMethodGrammar = []cli.Option{
	{
		LongName:    "verbose",
		ShortName:   "v",
		Description: "server.rest.method.verbose",
		OptionType:  cli.BooleanType,
	},
	{
		LongName:    "params",
		ShortName:   "p",
		Aliases:     []string{"param", "parameter", "parameters"},
		Description: "server.rest.method.headers",
		OptionType:  cli.StringListType,
	},
	{
		LongName:    "no-token",
		ShortName:   "k",
		Description: "server.rest.method.no-token",
		OptionType:  cli.BooleanType,
	},
}
