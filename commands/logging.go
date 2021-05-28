package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
)

func Logging(c *cli.Context) *errors.EgoError {
	addr := persistence.Get(defs.ApplicationServerSetting)
	if addr == "" {
		addr = persistence.Get(defs.LogonServerSetting)
		if addr == "" {
			addr = "localhost"
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

	err := ResolveServerName(addr)
	if !errors.Nil(err) {
		return err
	}

	loggers := defs.LoggingItem{Loggers: map[string]bool{}}
	response := defs.LoggingResponse{}

	if c.WasFound("enable") || c.WasFound("disable") {
		if c.WasFound("enable") {
			loggerNames, _ := c.GetStringList("enable")

			for _, loggerName := range loggerNames {
				logger := ui.Logger(loggerName)
				if logger < 0 {
					return errors.New(errors.ErrInvalidLoggerName).Context(strings.ToUpper(loggerName))
				}

				if logger == ui.ServerLogger {
					continue
				}

				loggers.Loggers[loggerName] = true
			}
		}

		if c.WasFound("disable") {
			loggerNames, _ := c.GetStringList("disable")

			for _, loggerName := range loggerNames {
				logger := ui.Logger(loggerName)
				if logger < 0 || logger == ui.ServerLogger {
					return errors.New(errors.ErrInvalidLoggerName).Context(strings.ToUpper(loggerName))
				}

				if _, ok := loggers.Loggers[loggerName]; ok {
					return errors.New(errors.ErrLoggerConflict).Context(loggerName)
				}

				loggers.Loggers[loggerName] = false
			}
		}

		// Send the update, get a reply
		err := runtime.Exchange("/admin/loggers/", "POST", &loggers, &response)
		if !errors.Nil(err) {
			return err
		}
	} else if c.WasFound("tail") {
		// Was it a --tail request?
		count, _ := c.GetInteger("tail")
		if count < 1 {
			count = 50
		}

		url := fmt.Sprintf("/services/admin/log/?tail=%d", count)
		lines := []string{}

		err := runtime.Exchange(url, "GET", nil, &lines)
		if !errors.Nil(err) {
			return err
		}

		switch ui.OutputFormat {
		case "text":
			for _, line := range lines {
				fmt.Println(line)
			}

		case "json":
			b, _ := json.Marshal(lines)
			fmt.Println(string(b))

		case "indented":
			b, _ := json.MarshalIndent(lines, "", "  ")
			fmt.Println(string(b))
		}

		return nil
	} else {
		// No changes, just ask for status
		err := runtime.Exchange("/admin/loggers/", "GET", nil, &response)
		if !errors.Nil(err) {
			return err
		}
	}

	// Formulate the output.

	if ui.QuietMode {
		return nil
	}

	fileOnly := c.GetBool("file")

	switch ui.OutputFormat {
	case "text":
		if fileOnly {
			ui.Say("%s", response.Filename)
		} else {
			fmt.Printf("Logging Status, hostname %s, ID %s\n\n", response.Hostname, response.ID)
			t, _ := tables.New([]string{"Logger", "Active"})

			for k, v := range response.Loggers {
				_ = t.AddRowItems(k, v)
			}

			_ = t.SortRows(0, true)
			_ = t.SetIndent(2)
			t.Print(ui.OutputFormat)

			if response.Filename != "" {
				fmt.Printf("\nServer log file is %s\n", response.Filename)
			}
		}

	case "json":
		if fileOnly {
			ui.Say("\"%s\"", response.Filename)
		} else {
			b, _ := json.Marshal(response)
			ui.Say(string(b))
		}

	case "indented":
		if fileOnly {
			ui.Say("\"%s\"", response.Filename)
		} else {
			b, _ := json.MarshalIndent(response, "", "   ")
			ui.Say(string(b))
		}
	}

	return nil
}
