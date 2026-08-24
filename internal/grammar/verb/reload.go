package verb

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
)

var ReloadVerbGrammar = []cli.Option{
	{
		LongName:    "tasks",
		Description: "ego.verb.reload.tasks",
		OptionType:  cli.Subcommand,
		Action:      commands.TaskReload,
	},
}
