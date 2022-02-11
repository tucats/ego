package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
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

		return remoteStatus(addr)
	}

	// Otherwise, it's the local server by port number.
	msg := "Server not running"

	status, err := server.ReadPidFile(c)
	if errors.Nil(err) {
		if server.IsRunning(status.PID) {
			msg = fmt.Sprintf("UP (pid %d, host %s, session %s) since %s, LOCAL",
				status.PID,
				status.Hostname,
				status.LogID,
				status.Started.Format(time.UnixDate))
		} else {
			_ = server.RemovePidFile(c)
		}
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", msg)
	} else if errors.Nil(err) {
		if ui.OutputFormat == ui.JSONIndentedFormat {
			b, _ := json.MarshalIndent(status, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			fmt.Print(string(b))
		} else {
			b, _ := json.Marshal(status)
			fmt.Print(string(b))
		}
	} else {
		s := defs.RestStatusResponse{Status: 500, Message: msg}
		b, _ := json.Marshal(s)
		fmt.Print(string(b))
	}

	return nil
}

// Ping a remote server's "up" service to see its status.
func remoteStatus(addr string) *errors.EgoError {
	resp := defs.RemoteStatusResponse{}

	if err := ResolveServerName(addr); !errors.Nil(err) {
		if strings.Contains(err.Error(), "connect: connection refused") {
			if ui.OutputFormat == ui.TextFormat {
				fmt.Println("DOWN")
			} else {
				s := defs.RestStatusResponse{Status: 500, Message: err.Error()}
				var b []byte

				if ui.OutputFormat == ui.JSONIndentedFormat {
					b, _ = json.MarshalIndent(s, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
				} else {
					b, _ = json.Marshal(s)
				}

				fmt.Print(string(b))
			}

			os.Exit(3)
		}

		return err
	}

	err := runtime.Exchange(defs.ServicesUpPath, http.MethodGet, nil, &resp, defs.AdminAgent)
	if !errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			fmt.Println("DOWN")
		} else {
			s := defs.RestStatusResponse{Status: 500, Message: err.Error()}
			var b []byte

			if ui.OutputFormat == ui.JSONIndentedFormat {
				b, _ = json.MarshalIndent(s, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			} else {
				b, _ = json.Marshal(s)
			}

			fmt.Print(string(b))
		}

		os.Exit(3)
	}

	if ui.OutputFormat == ui.TextFormat {
		ui.Say("UP (pid %d, host %s, session %s) since %s, %s", resp.Pid, resp.Hostname, resp.ServerInfo.ID, resp.Since, addr)
	} else if ui.OutputFormat == ui.JSONIndentedFormat {
		b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Print(string(b))
	} else {
		b, _ := json.Marshal(resp)
		fmt.Print(string(b))
	}

	return nil
}
