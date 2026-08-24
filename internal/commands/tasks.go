package commands

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
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

// TaskReload tells the server to re-scan its tasks directory and merge
// what it finds into the running registry, by calling POST
// /admin/tasks/@reload. This lets an admin add, edit, or reactivate a task
// by editing its file, without restarting the server. You must be an admin
// user to perform this command.
//
// Invoked by:
//
//	Traditional: ego task reload
//	Verb:        ego reload tasks
func TaskReload(c *cli.Context) error {
	url := rest.URLBuilder(defs.AdminTasksIDPath, defs.TaskReloadPseudoID)
	resp := defs.TasksResponse{}

	err := rest.Exchange(url.String(), http.MethodPost, nil, &resp, defs.AdminAgent, defs.TasksMediaType)
	if err == nil {
		if ui.OutputFormat != ui.TextFormat {
			_ = c.Output(resp)
		} else {
			msg := i18n.M("task.reloaded", map[string]any{"count": resp.Count})
			ui.Say(msg)
		}
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

// TaskShow retrieves and displays everything known about one scheduled
// task -- its full definition (as loaded from its JSON file) together with
// its current execution state -- by calling GET /admin/tasks/{id}. You
// must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego task show <task-id>
//	Verb:        ego show task <task-id>
func TaskShow(c *cli.Context) error {
	id := c.FindGlobal().Parameter(0)

	url := rest.URLBuilder(defs.AdminTasksIDPath, id)
	resp := defs.TaskDetailResponse{}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.AdminAgent, defs.TaskMediaType)
	if err != nil {
		return errors.New(err)
	}

	if ui.OutputFormat != ui.TextFormat {
		return c.Output(resp)
	}

	displayTaskDetail(&resp)

	return nil
}

// displayTaskDetail renders a defs.TaskDetailResponse as a two-column
// "Item"/"Value" table, covering every field of the task's definition
// followed by its current execution state.
func displayTaskDetail(resp *defs.TaskDetailResponse) {
	t, _ := tables.New([]string{i18n.L("Item"), i18n.L("Value")})

	lastRun := ""
	if !resp.LastRun.IsZero() {
		lastRun = resp.LastRun.Local().Format(time.RFC822)
	}

	loadedAt := ""
	if !resp.LoadedAt.IsZero() {
		loadedAt = resp.LoadedAt.Local().Format(time.RFC822)
	}

	body := string(resp.Body)
	if body == "" {
		body = i18n.L("none")
	}

	failedTest := resp.FailedTest
	if failedTest == "" {
		failedTest = i18n.L("none")
	}

	_ = t.AddRowItems(i18n.L("Task"), resp.Task)
	_ = t.AddRowItems(i18n.L("ID"), resp.ID)
	_ = t.AddRowItems(i18n.L("Active"), strconv.FormatBool(resp.Active))
	_ = t.AddRowItems(i18n.L("User"), resp.User)
	_ = t.AddRowItems(i18n.L("Method"), resp.Method)
	_ = t.AddRowItems(i18n.L("Endpoint"), resp.Endpoint)
	_ = t.AddRowItems(i18n.L("Parameters"), formatTaskStringMap(resp.Parameters))
	_ = t.AddRowItems(i18n.L("Body"), body)
	_ = t.AddRowItems(i18n.L("ExpectedStatus"), strconv.Itoa(resp.ExpectedStatus))
	_ = t.AddRowItems(i18n.L("Save"), formatTaskStringMap(resp.Save))
	_ = t.AddRowItems(i18n.L("Tests"), formatTaskChecks(resp.Tests))
	_ = t.AddRowItems(i18n.L("Timeout"), resp.Timeout)
	_ = t.AddRowItems(i18n.L("Interval"), resp.Interval)
	_ = t.AddRowItems(i18n.L("Count"), strconv.Itoa(resp.Count))
	_ = t.AddRowItems(i18n.L("After"), resp.After)
	_ = t.AddRowItems(i18n.L("Path"), resp.Path)
	_ = t.AddRowItems(i18n.L("Running"), strconv.FormatBool(resp.Running))
	_ = t.AddRowItems(i18n.L("LastRun"), lastRun)
	_ = t.AddRowItems(i18n.L("LastStatus"), strconv.Itoa(resp.LastStatus))
	_ = t.AddRowItems(i18n.L("Success"), strconv.FormatBool(resp.Success))
	_ = t.AddRowItems(i18n.L("RunCount"), strconv.Itoa(resp.RunCount))
	_ = t.AddRowItems(i18n.L("FailedTest"), failedTest)
	_ = t.AddRowItems(i18n.L("LoadedAt"), loadedAt)

	t.Print(ui.TextFormat)
}

// formatTaskStringMap renders a map[string]string (a task's "parameters" or
// "save" field) as a comma-separated list of "key=value" pairs, sorted by
// key for deterministic output, or "none" if the map is empty.
func formatTaskStringMap(m map[string]string) string {
	if len(m) == 0 {
		return i18n.L("none")
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}

	return strings.Join(parts, ", ")
}

// formatTaskChecks renders a task's "tests" block as a semicolon-separated
// summary of each check ("name (query op value)"), or "none" if there are
// no checks.
func formatTaskChecks(checks []defs.TaskCheck) string {
	if len(checks) == 0 {
		return i18n.L("none")
	}

	parts := make([]string, 0, len(checks))

	for _, check := range checks {
		op := check.Operator
		if op == "" {
			op = "eq"
		}

		parts = append(parts, fmt.Sprintf("%s (%s %s %s)", check.Name, check.Query, op, check.Value))
	}

	return strings.Join(parts, "; ")
}
