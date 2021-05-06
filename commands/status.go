package commands

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/server"
)

// Status displays the status of a running server if it exists.
func Status(c *cli.Context) *errors.EgoError {
	// If there is a parameter, it's the server address to query.
	if c.GetParameterCount() > 0 {
		addr := c.GetParameter(0)
		// If it's valid but has no port number, and --port was not
		// given on the command line, assume the default port 8080
		if u, err := url.Parse("https://" + addr); err == nil {
			if u.Port() == "" && !c.WasFound("port") {
				addr = addr + ":8080"
			}
		}

		if c.WasFound("port") {
			port, _ := c.GetInteger("port")
			addr = fmt.Sprintf("%s:%d", addr, port)
		}

		return remoteStatus(addr)
	}

	// Otherwise, it's the local server by port number.
	running := false
	msg := "Server not running"

	status, err := server.ReadPidFile(c)
	if errors.Nil(err) {
		if server.IsRunning(status.PID) {
			running = true
			msg = fmt.Sprintf("UP (pid %d, session %s) since %s, LOCAL",
				status.PID,
				status.LogID,
				status.Started.Format(time.UnixDate))
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

// Ping a remote server's "up" service to see its status.
func remoteStatus(addr string) *errors.EgoError {
	resp := struct {
		Pid     int    `json:"pid"`
		Session string `json:"session"`
		Since   string `json:"since"`
	}{}

	if _, err := url.Parse("https://" + addr); err != nil {
		return errors.New(err)
	}

	persistence.Set(defs.ApplicationServerSetting, "https://"+addr)

	err := runtime.Exchange("/services/up/", "GET", nil, &resp)
	if !errors.Nil(err) {
		persistence.Set(defs.ApplicationServerSetting, "http://"+addr)

		err := runtime.Exchange("/services/up/", "GET", nil, &resp)
		if !errors.Nil(err) {
			fmt.Println("DOWN")
			os.Exit(3)
		}
	}

	ui.Say("UP (pid %d, session %s) since %s", resp.Pid, resp.Session, resp.Since)

	return nil
}
