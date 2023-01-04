package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
)

func Logging(c *cli.Context) error {
	addr := settings.Get(defs.ApplicationServerSetting)
	if addr == "" {
		addr = settings.Get(defs.LogonServerSetting)
		if addr == "" {
			addr = defs.LocalHost
		}
	}

	if c.GetParameterCount() > 0 {
		addr = c.GetParameter(0)
		// If it's valid but has no port number, and --port was not
		// given on the command line, assume the default port 8080
		if u, err := url.Parse("https://" + addr); err == nil {
			if u.Port() == "" && !c.WasFound("port") {
				addr = addr + ":8080"
			}
		}
	}

	_, err := ResolveServerName(addr)
	if err != nil {
		return err
	}

	loggers := defs.LoggingItem{Loggers: map[string]bool{}}
	response := defs.LoggingResponse{}

	if c.WasFound("keep") {
		keep, _ := c.Integer("keep")
		u := runtime.URLBuilder("/admin/loggers/?keep=%d", keep)
		count := defs.DBRowCount{}

		err := runtime.Exchange(u.String(), http.MethodDelete, nil, &count, defs.AdminAgent)
		if err != nil {
			return err
		}

		if count.Count > 0 {
			ui.Say("msg.server.logs.purged", map[string]interface{}{
				"count": count.Count,
			})
		}
	}

	showStatus := c.Boolean("status")

	if c.WasFound("enable") || c.WasFound("disable") {
		showStatus = true

		if c.WasFound("enable") {
			loggerNames, _ := c.StringList("enable")

			for _, loggerName := range loggerNames {
				logger := ui.Logger(loggerName)
				if logger < 0 {
					return errors.EgoError(errors.ErrInvalidLoggerName).Context(strings.ToUpper(loggerName))
				}

				if logger == ui.ServerLogger {
					continue
				}

				loggers.Loggers[loggerName] = true
			}
		}

		if c.WasFound("disable") {
			loggerNames, _ := c.StringList("disable")

			for _, loggerName := range loggerNames {
				logger := ui.Logger(loggerName)
				if logger < 0 || logger == ui.ServerLogger {
					return errors.EgoError(errors.ErrInvalidLoggerName).Context(strings.ToUpper(loggerName))
				}

				if _, ok := loggers.Loggers[loggerName]; ok {
					return errors.EgoError(errors.ErrLoggerConflict).Context(loggerName)
				}

				loggers.Loggers[loggerName] = false
			}
		}

		// Send the update, get a reply
		err := runtime.Exchange(defs.AdminLoggersPath, http.MethodPost, &loggers, &response, defs.AdminAgent)
		if err != nil {
			return err
		}
	}

	fileOnly := c.Boolean("file")

	if showStatus || fileOnly {
		// No changes, just ask for status
		err := runtime.Exchange(defs.AdminLoggersPath, http.MethodGet, nil, &response, defs.AdminAgent)
		if err != nil {
			return err
		}
	} else {
		// Was a number of lines specified?
		count, _ := c.Integer("limit")
		if count < 1 {
			count = 50
		}

		url := fmt.Sprintf("/services/admin/log/?tail=%d", count)
		session, _ := c.Integer("session")
		if session > 0 {
			url = fmt.Sprintf("%s&session=%d", url, session)
		}

		lines := defs.LogTextResponse{}

		err := runtime.Exchange(url, http.MethodGet, nil, &lines, defs.AdminAgent)
		if err != nil {
			return err
		}

		if ui.OutputFormat == ui.TextFormat {
			for _, line := range lines.Lines {
				fmt.Println(line)
			}
		} else {
			_ = commandOutput(lines)
		}

		return nil
	}

	// Formulate the output.
	if ui.QuietMode {
		return nil
	}

	if ui.OutputFormat == ui.TextFormat {
		if fileOnly {
			ui.Say("%s", response.Filename)
		} else {
			fmt.Printf("%s\n\n", i18n.M("server.logs.status", map[string]interface{}{
				"host": response.Hostname,
				"id":   response.ID,
			}))

			t, _ := tables.New([]string{i18n.L("Logger"), i18n.L("Active")})

			for k, v := range response.Loggers {
				_ = t.AddRowItems(k, v)
			}

			_ = t.SortRows(0, true)
			_ = t.SetIndent(2)
			t.SetPagination(0, 0)

			t.Print(ui.OutputFormat)

			if response.Filename != "" {
				fmt.Printf("\n%s\n", i18n.M("server.logs.file", map[string]interface{}{
					"name": response.Filename,
				}))

				if response.RetainCount > 0 {
					if response.RetainCount == 1 {
						fmt.Printf("%s\n", i18n.M("server.logs.no.retain"))
					} else {
						fmt.Printf("%s\n", i18n.M("server.logs.retains", map[string]interface{}{
							"count": response.RetainCount - 1,
						}))
					}
				}
			}
		}
	} else {
		if fileOnly {
			_ = commandOutput(response.Filename)
		} else {
			_ = commandOutput(response)
		}
	}

	return nil
}
