package commands

import (
	"net/http"
	"os"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/server/server"
)

// Stop stops a running server if it exists.
func Stop(c *cli.Context) error {
	if err := profile.InitProfileDefaults(profile.RuntimeDefaults); err != nil {
		return err
	}

	// Are we doing this as a "--force" operation?
	if c.Boolean("force") {
		return forceStop(c)
	}

	_, err := politeStop(c)

	return err
}

// Force a stop operation on a running server process on the current machine. Kills the process
// (if found) and deletes the pid file.
func forceStop(c *cli.Context) error {
	var proc *os.Process

	status, err := server.ReadPidFile(c)
	if err == nil {
		var e2 error

		proc, e2 = os.FindProcess(status.PID)
		if e2 == nil {
			e2 = proc.Kill()
			if e2 == nil {
				if ui.OutputFormat == ui.TextFormat {
					ui.Say("msg.server.stopped", map[string]any{
						"pid": status.PID,
					})
				} else {
					_ = c.Output(status)
				}
			}
		}
	}

	_ = server.RemovePidFile(c)

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// politeStop uses the REST APU to attempt to request that the server stop, and polls to
// see if it has stopped.
func politeStop(c *cli.Context) (*defs.ServerStatus, error) {
	var (
		err    error
		status *defs.ServerStatus
	)

	status, _ = server.ReadPidFile(c)

	url := defs.ServicesDownPath
	resp := defs.RestStatusResponse{}

	err = rest.Exchange(url, http.MethodPost, nil, &resp, defs.AdminAgent)
	if err != nil {
		return nil, errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		if c.Boolean("verbose") {
			ui.Say("msg.server.stopped.id", ui.A{
				"id":      resp.ID,
				"session": resp.Session,
			})
		}

		ui.Say("msg.server.stopping", ui.A{
			"status": resp.Status})
	}

	// We'll wait five seconds for the server to stop. This normally takes only one second or so.
	retries := 5

	for retries > 0 {
		retries--
		resp = defs.RestStatusResponse{}

		// Pause for one second to give the server time to stop.
		time.Sleep(1 * time.Second)

		// See if the server is still running. If not, it will throw an error and we can report
		// on this and get out of dodge.
		err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, &resp, defs.AdminAgent)
		if err != nil {
			ui.Log(ui.RestLogger, "server.admin.stopping", ui.A{
				"error": err.Error(),
			})

			if ui.OutputFormat == ui.TextFormat {
				if status == nil || status.PID == 0 {
					ui.Say("msg.server.stopped.rest")
				} else {
					ui.Say("msg.server.stopped", ui.A{
						"pid": status.PID})
				}
			}

			break
		}

		// Still waiting for the server to stop, so we'll say we're waiting.
		ui.Log(ui.InternalLogger, "server.admin.waiting", nil)
	}

	return status, server.RemovePidFile(c)
}
