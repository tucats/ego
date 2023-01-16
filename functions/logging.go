package functions

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// SetLogger implements the util.SetLogger() function. This sets a logger to
// be enabled or disabled, and returns the previous state of the logger. It is
// an error to specify a non-existent logger name. Logger names are not case
// sensitive.
func SetLogger(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	name := strings.TrimSpace(data.String(args[0]))
	enabled := data.Bool(args[1])

	loggerID := ui.LoggerByName(name)
	if loggerID <= 0 {
		return nil, errors.ErrInvalidLoggerName.Context(name)
	}

	oldSetting := ui.IsActive(loggerID)

	ui.Active(loggerID, enabled)

	return oldSetting, nil
}

// LogTail implements the util.Log(n) function, which returns the last 'n' lines
// from the current.
func LogTail(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	count := data.Int(args[0])
	filter := 0

	if len(args) > 1 {
		filter = data.Int(args[1])
	}

	lines := ui.Tail(count, filter)

	if lines == nil {
		return []interface{}{}, nil
	}

	xLines := make([]interface{}, len(lines))
	for i, j := range lines {
		xLines[i] = j
	}

	return data.NewArrayFromArray(data.StringType, xLines), nil
}
