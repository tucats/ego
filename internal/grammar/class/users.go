package class

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
	"github.com/tucats/ego/internal/grammar/common"
)

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
		Value:         common.ServerUserGrammar,
	},
	{
		LongName:      "update",
		Description:   "ego.server.user.update",
		Aliases:       []string{"modify", "alter"},
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.UpdateUser,
		Value:         common.ServerUserGrammar,
	},
	{
		LongName:      "show",
		Description:   "ego.server.user.show",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.ShowUser,
		Value:         common.ServerShowUserGrammar,
	},
	{
		LongName:      "delete",
		Description:   "ego.server.user.delete",
		OptionType:    cli.Subcommand,
		ParmDesc:      "username",
		ExpectedParms: -1,
		Action:        commands.DeleteUser,
		Value:         common.ServerDeleteUserGrammar,
	},
	{
		LongName:    "list",
		Description: "ego.server.user.list",
		OptionType:  cli.Subcommand,
		Action:      commands.ListUsers,
		Value:       common.ServerListUsersGrammar,
		DefaultVerb: true,
	},
}
