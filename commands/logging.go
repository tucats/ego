package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
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

	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")

	if c.GetParameterCount() > 0 {
		addr = c.GetParameter(0)
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
	}

	_, err := getProtocol(addr)
	if !errors.Nil(err) {
		return err
	}

	loggers := map[string]int{}

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

			loggers[loggerName] = 1
		}
	}

	if c.WasFound("disable") {
		loggerNames, _ := c.GetStringList("disable")

		for _, loggerName := range loggerNames {
			logger := ui.Logger(loggerName)
			if logger < 0 || logger == ui.ServerLogger {
				return errors.New(errors.ErrInvalidLoggerName).Context(strings.ToUpper(loggerName))
			}

			if _, ok := loggers[loggerName]; ok {
				return errors.New(errors.ErrLoggerConflict).Context(loggerName)
			}

			loggers[loggerName] = 0
		}
	}

	// Loop over the loggers, requesting the status change(s)
	for logger, mode := range loggers {
		err := runtime.Exchange("/admin/loggers/"+logger, "POST", &mode, nil)
		if !errors.Nil(err) {
			return err
		}
	}

	return nil
}

func getProtocol(addr string) (string, *errors.EgoError) {
	protocol := ""

	resp := struct {
		Pid     int    `json:"pid"`
		Session string `json:"session"`
		Since   string `json:"since"`
	}{}

	if _, err := url.Parse("https://" + addr); err != nil {
		return protocol, errors.New(err)
	}

	protocol = "https"

	persistence.SetDefault(defs.ApplicationServerSetting, "https://"+addr)

	err := runtime.Exchange("/services/up/", "GET", nil, &resp)
	if !errors.Nil(err) {
		protocol = "http"

		persistence.SetDefault(defs.ApplicationServerSetting, "http://"+addr)

		err := runtime.Exchange("/services/up/", "GET", nil, &resp)
		if !errors.Nil(err) {
			return "", nil
		}
	}

	return protocol, nil
}
