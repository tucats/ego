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
		LongName:      "show",
		Description:   "ego.task.show",
		OptionType:    cli.Subcommand,
		Action:        commands.TaskShow,
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
	{
		LongName:    "reload",
		Description: "ego.task.reload",
		OptionType:  cli.Subcommand,
		Action:      commands.TaskReload,
	},
}
