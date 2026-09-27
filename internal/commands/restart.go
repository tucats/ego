package commands

import (
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util/fork"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/runtime/profile"
)

// Restart stops the currently-running detached server and immediately starts a new
// instance using the same arguments that were stored in the PID file when the server
// was first started. A new session UUID is generated for the restarted server so that
// log entries from the new run are distinguishable from the previous run.
//
// Not supported on Windows (detached processes use Unix-style process management).
//
// Invoked by:
//
//	Traditional: ego server restart
//	Verb:        ego restart server
func Restart(c *cli.Context) error {
	if err := profile.InitProfileDefaults(profile.RuntimeDefaults); err != nil {
		return err
	}

	// The server is identified by its port, and restarting it depends on the
	// command line arguments saved in its pid file. Check for them before
	// stopping anything, so a server that could not be started again is left
	// running rather than stopped. (A missing pid file used to be found only
	// after the server was stopped, and then crashed the restart by indexing an
	// empty argument list.)
	port := managedServerPort(c)

	if status, err := router.ReadPidFile(port); err != nil || len(status.Args) == 0 {
		return errors.ErrNoServerPidFile.Clone().Context(port)
	}

	serverStatus, err := killExistingServer(c, port)
	if !errors.Nil(err) {
		msg := err.Error()
		if strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") {
			err = errors.ErrServerDown
		}

		return err
	}

	if serverStatus == nil || len(serverStatus.Args) == 0 {
		return errors.ErrNoServerPidFile.Clone().Context(port)
	}

	args := serverStatus.Args

	// Set up the new ID. If there was one already (because this might be
	// a restart operation) then update the UUID value. If not, add the uuid
	// command line option.
	logID := uuid.New()
	found := false

	// INDEX-13: args is the command line read back from the server status file,
	// so a flag can appear as the very last element with no value after it.
	// Indexing args[i+1] unconditionally panicked in that case; a trailing
	// "--session-uuid" is now treated as "not present" so the option is
	// appended with its value below.
	for i, v := range args {
		if v == "--session-uuid" && i+1 < len(args) {
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
	if c.Boolean(defs.VerboseOption) && ui.OutputFormat == ui.TextFormat {
		logFile := "ego-server.log"

		// INDEX-13: the bound was "i+1 <= len(args)", which is off by one --
		// it admits i+1 == len(args) and so still read one past the end when
		// "--log-file" was the final argument. The valid index test is "<".
		for i, v := range args {
			if v == "--log-file" && i+1 < len(args) {
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
		err = router.WritePidFile(c, port, *serverStatus)

		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.server.started", map[string]any{
				"pid":  pid,
				"port": port,
			})
		} else {
			serverState, _ := router.ReadPidFile(port)
			_ = c.Output(serverState)
		}
	} else {
		_ = router.RemovePidFile(port)
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Kill off any existing instance of the server on the given port, if any. Returns
// the server status of the server if it was running, and an error code indicating
// if the server was killed. If the server was not running, returns nil and no error.
func killExistingServer(c *cli.Context, port int) (*defs.ServerStatus, error) {
	if c.Boolean("force") {
		status, err := router.ReadPidFile(port)
		if err == nil {
			proc, e2 := os.FindProcess(status.PID)
			if e2 == nil {
				e2 = proc.Kill()
				// If successful, and in text mode, report the stop to the console.
				if e2 == nil && ui.OutputFormat == ui.TextFormat {
					ui.Say("msg.server.stopped", map[string]any{
						"pid":  status.PID,
						"port": port,
					})
				}
			}

			if e2 != nil {
				err = errors.New(e2)
			}
		}

		_ = router.RemovePidFile(port)

		return status, err
	}

	// Not a force, so ask the server to stop politely via its REST API. The
	// request always goes to this machine: politeStop uses the configured
	// application (or logon) server's host name only when it names this machine.
	//
	// This used to compare the configured server's host name with os.Hostname()
	// and report ErrNotLocalServer when they were EQUAL -- the test was inverted,
	// so it refused exactly the case it meant to allow, and it never matched a
	// name like "myhost.local" against a host name of "MyHost" anyway.
	status, err := politeStop(c, port)

	return status, err
}
