package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// Status displays the status of a running server if it exists.
func Status(c *cli.Context) error {
	// If there is a parameter, it's the server address to query. If there isn't
	// try the application server, and if not specified, the login server. If neither
	// is present in the configuration, default to the current hostname (last resort)
	addr := settings.Get(defs.ApplicationServerSetting)
	if addr == "" {
		addr = settings.Get(defs.LogonServerSetting)
	}

	if addr == "" {
		addr, _ = os.Hostname()
	}

	if c.ParameterCount() > 0 {
		addr = c.Parameter(0)
	}

	if !c.Boolean("local") {
		return remoteStatus(addr, c.Boolean("verbose"))
	}

	// Otherwise, it's the local pidfile, based on the port number.
	msg := i18n.M("server.not.running")

	status, err := server.ReadPidFile(c)
	if err == nil {
		if server.IsRunning(status.PID) {
			since := "(" + util.FormatDuration(time.Since(status.Started), true) + ")"

			msg = fmt.Sprintf("UP (%s) %s %s %s",
				i18n.M("server.status", map[string]interface{}{
					"version": status.Version,
					"pid":     status.PID,
					"host":    status.Hostname,
					"id":      status.LogID,
				}),
				i18n.L("since"),
				status.Started.Format(time.UnixDate), since)
		} else {
			_ = server.RemovePidFile(c)
		}
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", msg)
	} else if err == nil {
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
func remoteStatus(addr string, verbose bool) error {
	resp := defs.RemoteStatusResponse{}

	name, err := ResolveServerName(addr)
	if err != nil {
		// This is not a good idea, comparing against text literal.
		// However, not sure how else to do it at this point, since the error
		// contains data about connection, etc. that we don't want to use in
		// the comparison.
		if strings.Contains(err.Error(), "connect: connection refused") {
			if ui.OutputFormat == ui.TextFormat {
				fmt.Println("DOWN")
			} else {
				var b []byte

				s := defs.RestStatusResponse{Status: 500, Message: err.Error()}

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

	err = rest.Exchange(defs.ServicesUpPath, http.MethodGet, nil, &resp, defs.StatusAgent)
	if err != nil {
		if ui.OutputFormat == ui.TextFormat {
			fmt.Println("DOWN")
		} else {
			_ = commandOutput(defs.RestStatusResponse{Status: 500, Message: err.Error()})
		}

		os.Exit(3)
	}

	if ui.OutputFormat == ui.TextFormat {
		var msg, since string

		if startTime, err := time.Parse(time.UnixDate, resp.Since); err == nil {
			if verbose {
				since = " (" + util.FormatDuration(time.Since(startTime), true) + ")"
			} else {
				since = " for " + util.FormatDuration(time.Since(startTime), true)
			}
		}

		if verbose {
			msg = fmt.Sprintf("UP (%s) %s %s%s, %s",
				i18n.M("server.status", map[string]interface{}{
					"version": resp.Version,
					"pid":     resp.Pid,
					"host":    resp.Hostname,
					"id":      resp.ID,
				}),
				i18n.L("since"),
				resp.Since, since,
				name)
		} else {
			msg = fmt.Sprintf("UP%s as %s", since, name)
		}
		
		ui.Say(msg)
	} else {
		_ = commandOutput(resp)
	}

	return nil
}
