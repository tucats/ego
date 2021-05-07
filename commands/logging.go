package commands

import (
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
)

func Logging(c *cli.Context) *errors.EgoError {
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

			loggers[loggerName] = 1
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
