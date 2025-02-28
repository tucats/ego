package errors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/i18n"
)

// Format an Ego Error as a string for human consumption.
func (e *Error) Error() string {
	var (
		b         strings.Builder
		predicate bool
	)

	if e == nil || e.err == nil {
		return ""
	}

	// Format the underlying error message text. Apply error localizations if available.
	errText := e.err.Error()
	text := i18n.E(strings.TrimPrefix(errText, "error."))

	// If this is part of a chain, format the linked message first. If the
	// message is getting kind of long, add a break to it.
	if e.next != nil {
		errorString := i18n.L("error") + ": "

		b.WriteString(e.next.Error())
		b.WriteString(",\n")
		b.WriteString(strings.Repeat(" ", len(errorString)))
	}

	// If we have a location, report that as module or module/line number
	if e.location != nil {
		if predicate {
			b.WriteString(", ")
		}

		if e.location.line > 0 || e.location.column > 0 {
			lineStr := strconv.Itoa(e.location.line)

			if e.location.column > 0 {
				lineStr = lineStr + ":" + strconv.Itoa(e.location.column)
			}

			b.WriteString("at ")

			if len(e.location.name) > 0 {
				b.WriteString(fmt.Sprintf("%s(line %s)", e.location.name, lineStr))
			} else {
				b.WriteString(fmt.Sprintf("line %s", lineStr))
			}

			predicate = true
		} else {
			if e.location.name != "" {
				b.WriteString("in ")
				b.WriteString(e.location.name)

				predicate = true
			}
		}
	}

	// If we have an underlying error, report the string value for that
	if !Nil(e.err) {
		if !e.Is(ErrUserDefined) {
			if predicate {
				b.WriteString(", ")
			}

			msg := strings.TrimPrefix(text, "error.")
			predicate = true

			b.WriteString(msg)
		}
	}

	// If we have additional context, report that
	if e.context != "" {
		if predicate {
			b.WriteString(": ")
		}

		b.WriteString(e.context)
	}

	return b.String()
}
