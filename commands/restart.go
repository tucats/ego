package commands

import (
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/fork"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/server/server"
)

// Restart stops and then starts a server, using the information
// from the previous start that was stored in the pid file.
func Restart(c *cli.Context) error {
	if err := profile.InitProfileDefaults(profile.RuntimeDefaults); err != nil {
		return err
	}

	serverStatus, err := killExistingServer(c)
	if !errors.Nil(err) {
		msg := err.Error()
		if strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") {
			err = errors.ErrServerDown
		}

		return err
	}

	args := serverStatus.Args

	// Set up the new ID. If there was one already (because this might be
	// a restart operation) then update the UUID value. If not, add the uuid
	// command line option.
	logID := uuid.New()
	found := false

	for i, v := range args {
		if v == "--session-uuid" {
			args[i+1] = logID.String()
			found = true

			break
		}
	}

	if !found {
		args = append(args, "--session-uuid", logID.String())
	}

	if c.Boolean("new-token") {
		args = append(args, "--new-token")
	}

	// If output is in chatty text format and verbose was requested, output an extra
	// line describing the server id, image path, and log file name.
	if c.Boolean("verbose") && ui.OutputFormat == ui.TextFormat {
		logFile := "ego-server.log"

		for i, v := range args {
			if v == "--log-file" && i+1 <= len(args) {
				logFile = args[i+1]

				break
			}
		}

		ui.Say("msg.server.start.verbose", ui.A{
			"id":   logID.String(),
			"path": args[0],
			"log":  logFile,
		})
	}

	// Sleep for one second. This guarantees that the log file stamp of the new log
	// will not be the same as the old log stamp.
	time.Sleep(1 * time.Second)

	// Launch the new process
	pid, err := fork.Run(args[0], args)
	if err == nil {
		serverStatus.PID = pid
		serverStatus.ID = logID.String()

		// Scan over args and remove any instance of "--new-token". These are
		// not saved in the pid file, so this option is only a "one-shot"
		for i, v := range args {
			if v == "--new-token" {
				args = append(args[:i], args[i+1:]...)
			}
		}

		// Write the new status to the pid file.
		// We need to write it again, because the log file name might have changed.
		// Note that the log file name is not included in the status.Args slice.
		serverStatus.Args = args
		err = server.WritePidFile(c, *serverStatus)

		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.server.started", map[string]any{
				"pid": pid,
			})
		} else {
			serverState, _ := server.ReadPidFile(c)
			_ = c.Output(serverState)
		}
	} else {
		_ = server.RemovePidFile(c)
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Kill off any existing instance of the server, if any. Returns the server status
// of the server if it was running, and an error code indicating if the server
// was killed. If the server was not running, returns nil and no error.
func killExistingServer(c *cli.Context) (*defs.ServerStatus, error) {
	if c.Boolean("force") {
		status, err := server.ReadPidFile(c)
		if err == nil {
			proc, e2 := os.FindProcess(status.PID)
			if e2 == nil {
				e2 = proc.Kill()
				// If successful, and in text mode, report the stop to the console.
				if e2 == nil && ui.OutputFormat == ui.TextFormat {
					ui.Say("msg.server.stopped", map[string]any{
						"pid": status.PID,
					})
				}
			}

			if e2 != nil {
				err = errors.New(e2)
			}
		}

		_ = server.RemovePidFile(c)

		return status, err
	}

	// Not a force, let's try it the polite way. The current server (app.server
	// or logon.server) must be the current server or this operation is invalid
	// (cannot do a restart on another server)

	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.New(err)
	}

	server := settings.Get(defs.ApplicationServerSetting)
	if server == "" {
		server = settings.Get(defs.LogonServerSetting)
	}

	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")

	server, _, _ = strings.Cut(server, ":")
	if strings.EqualFold(server, hostname) {
		return nil, errors.ErrNotLocalServer.Context(server)
	}

	// All good, let's try to send a REST stop command to the local server.
	status, err := politeStop(c)

	return status, err
}
