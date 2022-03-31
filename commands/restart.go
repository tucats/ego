package commands

import (
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	server "github.com/tucats/ego/server"
)

// Restart stops and then starts a server, using the information
// from the previous start that was stored in the pidfile.
func Restart(c *cli.Context) *errors.EgoError {
	var proc *os.Process

	var e2 error

	status, err := server.ReadPidFile(c)
	if errors.Nil(err) {
		proc, e2 = os.FindProcess(status.PID)
		if e2 == nil {
			e2 = proc.Kill()
			// If successful, and in text mode, report the stop to the console.
			if e2 == nil && ui.OutputFormat == ui.TextFormat {
				ui.Say("Server (pid %d) stopped", status.PID)
			}
		}

		if e2 != nil {
			err = errors.New(e2)
		}
	}

	_ = server.RemovePidFile(c)

	if errors.Nil(err) {
		args := status.Args

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

		// Sleep for one second. This guarantees that the log file stamp of the new log
		// will not be the same as the old log stamp.
		time.Sleep(1 * time.Second)

		// Launch the new process
		pid, err := runExec(args[0], args)
		if errors.Nil(err) {
			status.PID = pid
			status.LogID = logID
			status.Args = args
			err = server.WritePidFile(c, *status)

			if ui.OutputFormat == ui.TextFormat {
				ui.Say("Server started as process %d", pid)
			} else {
				serverState, _ := server.ReadPidFile(c)
				_ = commandOutput(serverState)
			}
		} else {
			_ = server.RemovePidFile(c)
		}

		return errors.New(err)
	}

	return err
}
