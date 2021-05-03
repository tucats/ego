package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server"
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
			if e2 == nil {
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

		// Find the log file from the command-line args. If it's not
		// found, use the default just so we can keep going.
		logFileName := "ego-server.log"

		for i, v := range args {
			if v == "--log" {
				logFileName = args[i+1]
			}
		}

		logFileName, _ = filepath.Abs(logFileName)

		logf, err := os.Create(logFileName)
		if !errors.Nil(err) {
			return errors.New(err)
		}

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

		if _, err = logf.WriteString(fmt.Sprintf("*** Log file re-initialized %s ***\n",
			time.Now().Format(time.UnixDate)),
		); !errors.Nil(err) {
			return errors.New(err)
		}

		pid, err := runExec(args[0], args, logf)
		if errors.Nil(err) {
			status.PID = pid
			status.LogID = logID
			status.Args = args
			err = server.WritePidFile(c, *status)

			ui.Say("Server re-started as process %d", pid)
		} else {
			_ = server.RemovePidFile(c)
		}

		return errors.New(err)
	}

	return err
}
