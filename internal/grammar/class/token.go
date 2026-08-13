package class

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/grammar/common"
	"github.com/tucats/ego/internal/i18n"
)

var TokenGrammar = []cli.Option{
	{
		LongName:    "list",
		Description: "ego.token.list",
		OptionType:  cli.Subcommand,
		Action:      commands.TokenList,
	},
	{
		LongName:      defs.RevokeOption,
		Description:   "ego.verb.token.revoke",
		OptionType:    cli.Subcommand,
		Action:        commands.TokenRevoke,
		ExpectedParms: -99,
		MinParams:     1,
		ParmDesc:      "token-id",
	},
	{
		LongName:      "delete",
		Description:   "ego.verb.delete.token",
		OptionType:    cli.Subcommand,
		ParmDesc:      "token-id [token-id...]",
		ExpectedParms: -99,
		MinParams:     1,
		Action:        commands.TokenDelete,
		Prompts:       []string{i18n.L("prompt.token.id")},
	},
	{
		LongName:    "flush",
		Description: "ego.verb.flush.tokens",
		OptionType:  cli.Subcommand,
		Action:      commands.TokenFlush,
		Value:       common.CacheFlushGrammar,
	},
}
