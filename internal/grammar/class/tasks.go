package class

import (
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/commands"
)

// TaskGrammar specifies the command line options for the "task" Ego command.
var TaskGrammar = []cli.Option{
	{
		LongName:      "start",
		Description:   "ego.task.start",
		OptionType:    cli.Subcommand,
		Action:        commands.TaskStart,
		ParmDesc:      "task-id",
		ExpectedParms: 1,
	},
	{
		LongName:      "delete",
		Description:   "ego.task.delete",
		OptionType:    cli.Subcommand,
		Action:        commands.TaskDelete,
		ParmDesc:      "task-id",
		ExpectedParms: 1,
	},
	{
		LongName:    "list",
		Description: "ego.tasks.list",
		OptionType:  cli.Subcommand,
		Action:      commands.TaskList,
		DefaultVerb: true,
	},
}
