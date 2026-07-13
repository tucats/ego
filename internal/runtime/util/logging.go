package util

import (
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// setLogger implements the util.SetLogger() function. This sets a logger to
// be enabled or disabled, and returns the previous state of the logger. It is
// an error to specify a non-existent logger name. Logger names are not case
// sensitive.
func setLogger(symbols *symbols.SymbolTable, args data.List) (any, error) {
	name := strings.TrimSpace(data.String(args.Get(0)))

	enabled, err := data.Bool(args.Get(1))
	if err != nil {
		err = errors.New(err).In("SetLogger")

		return data.NewList(nil, err), err
	}

	loggerID := ui.LoggerByName(name)
	if loggerID < 0 {
		err = errors.ErrInvalidLoggerName.Context(name)

		return data.NewList(nil, err), err
	}

	oldSetting := ui.IsActive(loggerID)

	ui.Active(loggerID, enabled)

	return data.NewList(oldSetting, nil), nil
}

// getLogContents implements the util.Log(n[, session]) function, which returns the
// last n lines from the log file as an Ego string array.
//
// The optional second argument, session, restricts results to lines from a specific
// log session ID. Pass 0 (or omit the argument) to return lines from all sessions.
//
// When the log buffer is empty or logging is not configured to retain lines,
// an empty array is returned rather than nil.
func getLogContents(s *symbols.SymbolTable, args data.List) (any, error) {
	count, err := data.Int(args.Get(0))
	if err != nil {
		err = errors.New(err).In("Log")

		return data.NewList(nil, err), err
	}

	filter := 0

	if args.Len() > 1 {
		filter, err = data.Int(args.Get(1))
		if err != nil {
			err = errors.New(err).In("Log")

			return data.NewList(nil, err), err
		}
	}

	lines, err := ui.Tail(count, filter)
	if err != nil {
		err = errors.New(err).Context("Log()")

		return data.NewList(nil, err), err
	}

	if lines == nil {
		return data.NewList(data.NewArray(data.StringType, 0), nil), nil
	}

	// ui.Tail returns []string, but data.NewArrayFromInterfaces requires []any,
	// so we copy each element into a new slice with the interface type.
	xLines := make([]any, len(lines))
	for i, j := range lines {
		xLines[i] = j
	}

	return data.NewList(data.NewArrayFromInterfaces(data.StringType, xLines...), nil), nil
}
