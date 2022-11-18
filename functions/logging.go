package functions

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// SetLogger implements the util.SetLogger() function. This sets a logger to
// be enabled or disabled, and returns the previous state of the logger. It is
// an error to specify a non-existent logger name. Logger names are not case
// sensitive.
func SetLogger(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	name := strings.TrimSpace(datatypes.GetString(args[0]))
	enabled := datatypes.GetBool(args[1])

	loggerID := ui.Logger(name)
	if loggerID <= 0 {
		return nil, errors.New(errors.ErrInvalidLoggerName).Context(name)
	}

	oldSetting := ui.LoggerIsActive(loggerID)

	ui.SetLogger(loggerID, enabled)

	return oldSetting, nil
}

// LogTail implements the util.Log(n) function, which returns the last 'n' lines
// from the current.
func LogTail(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	count := datatypes.GetInt(args[0])
	filter := 0

	if len(args) > 1 {
		filter = datatypes.GetInt(args[1])
	}

	lines := ui.Tail(count, filter)

	if lines == nil {
		return []interface{}{}, nil
	}

	xLines := make([]interface{}, len(lines))
	for i, j := range lines {
		xLines[i] = j
	}

	return datatypes.NewArrayFromArray(&datatypes.StringType, xLines), nil
}
