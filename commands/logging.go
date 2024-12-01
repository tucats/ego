package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

const (
	httpsPrefix = "https://"
)

// Logging is the CLI action that enables or disables logging for a remote server.
func Logging(c *cli.Context) error {
	// Validate server address and port supplied on the command line.
	if err := validateServerAddressAndPort(c); err != nil {
		return err
	}

	loggers := defs.LoggingItem{Loggers: map[string]bool{}}
	response := defs.LoggingResponse{}

	if c.WasFound("keep") {
		if err := setLogKeepValue(c); err != nil {
			return err
		}
	}

	showStatus := c.Boolean("status")

	if c.WasFound("enable") || c.WasFound("disable") {
		if c.WasFound("enable") {
			loggerNames, _ := c.StringList("enable")

			for _, loggerName := range loggerNames {
				logger := ui.LoggerByName(loggerName)
				if logger < 0 {
					return errors.ErrInvalidLoggerName.Context(strings.ToUpper(loggerName))
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
				logger := ui.LoggerByName(loggerName)
				if logger < 0 || logger == ui.ServerLogger {
					return errors.ErrInvalidLoggerName.Context(strings.ToUpper(loggerName))
				}

				if _, ok := loggers.Loggers[loggerName]; ok {
					return errors.ErrLoggerConflict.Context(loggerName)
				}

				loggers.Loggers[loggerName] = false
			}
		}

		// Send the update, get a reply
		err := rest.Exchange(defs.AdminLoggersPath, http.MethodPost, &loggers, &response, defs.AdminAgent)
		if err != nil {
			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(response)
			}

			return err
		}

		if !showStatus && ui.OutputFormat == "text" {
			reportLoggerStatusUpdate(response)

			return nil
		} else {
			showStatus = true
		}
	}

	fileOnly := c.Boolean("file")

	if showStatus || fileOnly {
		// No changes, just ask for status
		err := rest.Exchange(defs.AdminLoggersPath, http.MethodGet, nil, &response, defs.AdminAgent)
		if err != nil {
			return err
		}
	} else {
		return reportServerLog(c)
	}

	// Formulate the output.
	if ui.QuietMode {
		return nil
	}

	if ui.OutputFormat == ui.TextFormat {
		if fileOnly {
			ui.Say("%s", response.Filename)
		} else {
			reportFullLoggerStatus(response)
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

func reportFullLoggerStatus(response defs.LoggingResponse) {
	fmt.Printf("%s\n\n", i18n.M("server.logs.status", map[string]interface{}{
		"host": response.Hostname,
		"id":   response.ID,
	}))

	keys := []string{}
	for k := range response.Loggers {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	enabled := strings.Builder{}
	disabled := strings.Builder{}

	for _, key := range keys {
		if response.Loggers[key] {
			if enabled.Len() > 0 {
				enabled.WriteString(", ")
			}

			enabled.WriteString(key)
		} else {
			if disabled.Len() > 0 {
				disabled.WriteString(", ")
			}

			disabled.WriteString(key)
		}
	}

	enabledLabel := i18n.L("logs.enabled")
	disabledLabel := i18n.L("logs.disabled")

	fmt.Printf("%-10s: %s\n%-10s: %s\n",
		enabledLabel, enabled.String(),
		disabledLabel, disabled.String())

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

func reportServerLog(c *cli.Context) error {
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

	err := rest.Exchange(url, http.MethodGet, nil, &lines, defs.AdminAgent)
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

func reportLoggerStatusUpdate(response defs.LoggingResponse) {
	report := strings.Builder{}

	keys := []string{}

	for k, v := range response.Loggers {
		if v {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for n, k := range keys {
		if n > 0 {
			report.WriteString(", ")
		}

		report.WriteString(k)
	}

	fmt.Println(i18n.L("active.loggers"), report.String())
}

func setLogKeepValue(c *cli.Context) error {
	keep, _ := c.Integer("keep")
	u := rest.URLBuilder("/admin/loggers/?keep=%d", keep)
	count := defs.DBRowCount{}

	err := rest.Exchange(u.String(), http.MethodDelete, nil, &count, defs.AdminAgent)
	if err != nil {
		return err
	}

	if count.Count > 0 {
		ui.Say("msg.server.logs.purged", map[string]interface{}{
			"count": count.Count,
		})
	}

	return nil
}

func validateServerAddressAndPort(c *cli.Context) error {
	addr := settings.Get(defs.ApplicationServerSetting)
	if addr == "" {
		addr = settings.Get(defs.LogonServerSetting)
		if addr == "" {
			addr = defs.LocalHost
		}
	}

	if c.ParameterCount() > 0 {
		addr = c.Parameter(0)

		if u, err := url.Parse(httpsPrefix + addr); err == nil {
			if u.Port() == "" && !c.WasFound("port") {
				addr = addr + ":443"
			}
		}
	}

	if _, err := ResolveServerName(addr); err != nil {
		return err
	}

	return nil
}
