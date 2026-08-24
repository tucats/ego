package commands

import (
	"net/http"
	"strconv"
	"time"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/tables"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/runtime/rest"
)

// TaskStart starts a scheduled task immediately, outside its normal
// schedule, by calling POST /admin/tasks/{id}. The task's repeat timer
// restarts from when this run finishes, exactly like a normal scheduled
// run. You must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego task start <task-id>
//	Verb:        ego start task <task-id>
func TaskStart(c *cli.Context) error {
	id := c.FindGlobal().Parameter(0)

	url := rest.URLBuilder(defs.AdminTasksIDPath, id)
	resp := defs.RestStatusResponse{}

	err := rest.Exchange(url.String(), http.MethodPost, nil, &resp, defs.AdminAgent, defs.TasksMediaType)
	if err == nil {
		msg := i18n.M("task.started", map[string]any{"id": id})
		ui.Say(msg)
	} else {
		if ui.OutputFormat != ui.TextFormat {
			_ = c.Output(resp)
		} else {
			ui.Say(resp.Message)
		}
	}

	return err
}

// TaskDelete deactivates a scheduled task by calling DELETE
// /admin/tasks/{id}. The task remains loaded and still reportable via
// "ego task list", just inactive, until the task file is edited and the
// server is reloaded. You must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego task delete <task-id>
//	Verb:        ego delete task <task-id>
func TaskDelete(c *cli.Context) error {
	id := c.FindGlobal().Parameter(0)

	url := rest.URLBuilder(defs.AdminTasksIDPath, id)
	resp := defs.RestStatusResponse{}

	err := rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent, defs.TasksMediaType)
	if err == nil {
		msg := i18n.M("task.deleted", map[string]any{"id": id})
		ui.Say(msg)
	} else {
		if ui.OutputFormat != ui.TextFormat {
			_ = c.Output(resp)
		} else {
			ui.Say(resp.Message)
		}
	}

	return err
}

// TaskList retrieves and displays every scheduled task known to the
// server, along with its active/running state and the outcome of its
// last run. You must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego task list
//	Verb:        ego list tasks
func TaskList(c *cli.Context) error {
	resp := defs.TasksResponse{}

	url := rest.URLBuilder(defs.AdminTasksPath)

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.AdminAgent, defs.TasksMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			t, _ := tables.New([]string{
				i18n.L("ID"),
				i18n.L("Task"),
				i18n.L("Active"),
				i18n.L("Running"),
				i18n.L("LastRun"),
				i18n.L("LastStatus"),
				i18n.L("Success"),
				i18n.L("RunCount"),
			})

			for _, item := range resp.Items {
				lastRun := ""
				if !item.LastRun.IsZero() {
					lastRun = item.LastRun.Local().Format(time.RFC822)
				}

				_ = t.AddRow([]string{
					item.ID,
					item.Task,
					strconv.FormatBool(item.Active),
					strconv.FormatBool(item.Running),
					lastRun,
					strconv.Itoa(item.LastStatus),
					strconv.FormatBool(item.Success),
					strconv.Itoa(item.RunCount),
				})
			}

			t.Print(ui.TextFormat)
		} else {
			c.Output(resp)
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}
