package commands

import (
	"fmt"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server"
)

// Status displays the status of a running server if it exists.
func Status(c *cli.Context) *errors.EgoError {
	running := false
	msg := "Server not running"

	status, err := server.ReadPidFile(c)
	if errors.Nil(err) {
		if server.IsRunning(status.PID) {
			running = true
			msg = fmt.Sprintf("Server is running (pid %d, session %s) since %v",
				status.PID,
				status.LogID,
				status.Started)
		} else {
			_ = server.RemovePidFile(c)
		}
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", msg)
	} else {
		// no difference for json vs indented
		fmt.Printf("%v\n", running)
	}

	return nil
}
