package util

import (
	"strings"
	"time"

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

// getLogContents implements the util.Log(n[, session[, class[, message]]])
// function, which returns the last n lines from the log file as an Ego string
// array.
//
// All the filtering arguments are optional, and each one narrows the result
// further:
//
//	session  restricts results to one log session ID; 0 means every session.
//	class    is a comma-separated list of logger classes ("REST,AUTH");
//	         an empty string means every class.
//	message  is a glob pattern matched against the message identifier
//	         ("rest.*"); an empty string means every message.
//
// The class and message filters read fields that exist only in a JSON-format
// log file. Asking for them while the server writes a text-format log is an
// error rather than a silently unfiltered result.
//
// Note that the filter is applied before the count: asking for 50 lines of
// class REST yields 50 REST lines if that many exist anywhere in the log, not
// whichever REST lines happen to fall in the last 50 lines of the file.
//
// When the log buffer is empty or logging is not configured to retain lines,
// an empty array is returned rather than nil.
func getLogContents(s *symbols.SymbolTable, args data.List) (any, error) {
	count, err := data.Int(args.Get(0))
	if err != nil {
		err = errors.New(err).In("Log")

		return data.NewList(nil, err), err
	}

	filter := ui.LogFilter{}

	if args.Len() > 1 {
		filter.Session, err = data.Int(args.Get(1))
		if err != nil {
			err = errors.New(err).In("Log")

			return data.NewList(nil, err), err
		}
	}

	if args.Len() > 2 {
		filter.Classes = ui.SplitClassList(data.String(args.Get(2)))
	}

	if args.Len() > 3 {
		filter.Message = strings.TrimSpace(data.String(args.Get(3)))
	}

	if args.Len() > 4 {
		filter.Archive, err = data.Bool(args.Get(4))
		if err != nil {
			err = errors.New(err).In("Log")

			return data.NewList(nil, err), err
		}
	}

	// Since and until arrive as Unix seconds rather than a structured time
	// value, because that is what can cross the boundary into an Ego
	// builtin call cleanly; zero means "no bound" on either end.
	if args.Len() > 5 {
		since, err := data.Int64(args.Get(5))
		if err != nil {
			err = errors.New(err).In("Log")

			return data.NewList(nil, err), err
		}

		if since != 0 {
			filter.Since = time.Unix(since, 0)
		}
	}

	if args.Len() > 6 {
		until, err := data.Int64(args.Get(6))
		if err != nil {
			err = errors.New(err).In("Log")

			return data.NewList(nil, err), err
		}

		if until != 0 {
			filter.Until = time.Unix(until, 0)
		}
	}

	lines, err := ui.TailFiltered(count, filter)
	if err != nil {
		// Context() replaces whatever context the error already carried, so only
		// add the "Log()" call site when there is nothing better there. A
		// rejected filter arrives naming the offending class or pattern, and
		// that is far more use to the caller than being told which function
		// they already know they called.
		wrapped := errors.New(err)
		if wrapped.GetContext() == "" {
			wrapped = wrapped.Context("Log()")
		}

		return data.NewList(nil, wrapped), wrapped
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
