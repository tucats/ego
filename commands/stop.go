package commands

import (
	"os"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/server"
)

// Stop stops a running server if it exists.
func Stop(c *cli.Context) *errors.EgoError {
	var proc *os.Process

	status, err := server.ReadPidFile(c)
	if errors.Nil(err) {
		var e2 error

		proc, e2 = os.FindProcess(status.PID)
		if e2 == nil {
			e2 = proc.Kill()
			if e2 == nil {
				if ui.OutputFormat == ui.TextFormat {
					ui.Say("Server (pid %d) stopped", status.PID)
				} else {
					_ = commandOutput(status)
				}
			}
		}
	}

	_ = server.RemovePidFile(c)

	return errors.New(err)
}
