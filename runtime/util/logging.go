package util

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// setLogger implements the util.SetLogger() function. This sets a logger to
// be enabled or disabled, and returns the previous state of the logger. It is
// an error to specify a non-existent logger name. Logger names are not case
// sensitive.
func setLogger(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	name := strings.TrimSpace(data.String(args.Get(0)))
	enabled := data.Bool(args.Get(1))

	loggerID := ui.LoggerByName(name)
	if loggerID <= 0 {
		return nil, errors.ErrInvalidLoggerName.Context(name)
	}

	oldSetting := ui.IsActive(loggerID)

	ui.Active(loggerID, enabled)

	return oldSetting, nil
}

// getLogContents implements the util.Log(n) function, which returns the last 'n' lines
// from the current.
func getLogContents(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	count := data.Int(args.Get(0))
	filter := 0

	if args.Len() > 1 {
		filter = data.Int(args.Get(1))
	}

	lines, err := ui.Tail(count, filter)
	if err != nil {
		return nil, errors.New(err).Context("Log()")
	}

	if lines == nil {
		return []interface{}{}, nil
	}

	xLines := make([]interface{}, len(lines))
	for i, j := range lines {
		xLines[i] = j
	}

	return data.NewArrayFromInterfaces(data.StringType, xLines...), nil
}
