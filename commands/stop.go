package commands

import (
	"os"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
)

// Stop stops a running server if it exists.
func Stop(c *cli.Context) error {
	var proc *os.Process

	status, err := server.ReadPidFile(c)
	if err == nil {
		var e2 error

		proc, e2 = os.FindProcess(status.PID)
		if e2 == nil {
			e2 = proc.Kill()
			if e2 == nil {
				if ui.OutputFormat == ui.TextFormat {
					ui.Say("msg.server.stopped", map[string]interface{}{
						"pid": status.PID,
					})
				} else {
					_ = commandOutput(status)
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
