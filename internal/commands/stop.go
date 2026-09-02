package commands

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/runtime/profile"
	"github.com/tucats/ego/internal/runtime/rest"
)

// Stop shuts down a running detached ego server. By default it sends a polite REST
// shutdown request and waits up to five seconds for the server to stop, extended by
// --grace if given. With --force it kills the process directly using the PID stored
// in the PID file.
//
// Not supported on Windows (detached processes use Unix-style process management).
//
// Invoked by:
//
//	Traditional: ego server stop
//	Verb:        ego stop server
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

	status, err := router.ReadPidFile(c)
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

	_ = router.RemovePidFile(c)

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// localServerURL builds the http(s)://localhost:port base URL for the locally
// managed detached server, using the scheme and port recorded in its PID file
// arguments (as they were passed to "server start"). Stop and Restart only ever
// target the local machine (see the hostname check in killExistingServer), so
// this base URL must reflect how this specific server instance was actually
// launched -- not the CLI's ApplicationServerSetting/LogonServerSetting, which
// point at whatever remote server the user last logged into and have no
// relation to this server's port or (in)security.
func localServerURL(status *defs.ServerStatus) string {
	insecure := false
	port := 0

	if status != nil {
		args := status.Args
		for i, v := range args {
			if v == "-k" || v == "--not-secure" {
				insecure = true
			}

			if (v == "--port" || v == "-p") && i+1 < len(args) {
				if p, err := strconv.Atoi(args[i+1]); err == nil {
					port = p
				}
			}
		}
	}

	if port == 0 {
		port = settings.GetInt(defs.ServerDefaultPortSetting)
	}

	if port == 0 {
		if insecure {
			port = 80
		} else {
			port = 443
		}
	}

	scheme := "https"
	if insecure {
		scheme = "http"
	}

	return fmt.Sprintf("%s://localhost:%d", scheme, port)
}

// politeStop uses the REST APU to attempt to request that the server stop, and polls to
// see if it has stopped.
func politeStop(c *cli.Context) (*defs.ServerStatus, error) {
	var (
		err    error
		status *defs.ServerStatus
	)

	status, _ = router.ReadPidFile(c)

	base := localServerURL(status)
	url := base + defs.ServicesDownPath

	// The client normally waits up to five seconds for the server to stop. If the
	// caller asked for a longer grace period, extend the wait so we don't give up
	// -- and remove the PID file -- while the server is still draining requests.
	waitSeconds := 5

	if grace, found := c.String("grace"); found {
		g, err := time.ParseDuration(grace)
		if err != nil || g < 0 {
			return nil, errors.ErrInvalidDuration.Context(grace)
		}

		url += "?grace=" + grace
		waitSeconds += int(g.Seconds())
	}

	resp := defs.RestStatusResponse{}

	err = rest.Exchange(url, http.MethodPost, nil, &resp, defs.AdminAgent)
	if err != nil {
		return nil, errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		if c.Boolean(defs.VerboseOption) {
			ui.Say("msg.server.stopped.id", ui.A{
				"id":      resp.ID,
				"session": resp.Session,
			})
		}

		ui.Say("msg.server.stopping", ui.A{
			"status": resp.Status})
	}

	// We'll wait up to waitSeconds for the server to stop. This normally takes only
	// one second or so, unless a longer --grace period was requested above.
	retries := waitSeconds

	for retries > 0 {
		retries--
		resp = defs.RestStatusResponse{}

		// Pause for one second to give the server time to stop.
		time.Sleep(1 * time.Second)

		// See if the server is still running. If not, it will throw an error and we can report
		// on this and get out of dodge.
		err = rest.Exchange(base+defs.AdminHeartbeatPath, http.MethodGet, nil, &resp, defs.AdminAgent, "application/json")
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

	return status, router.RemovePidFile(c)
}
