package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
)

// The foramt of a JSON log entry.
type LogEntry struct {
	Timestamp string                 `json:"time"`
	ID        string                 `json:"id"`
	Sequence  int                    `json:"seq"`
	Session   int                    `json:"session,omitempty"`
	Class     string                 `json:"class"`
	Message   string                 `json:"msg"`
	Args      map[string]interface{} `json:"args,omitempty"`
}

// OutputFormat is the default output format if not overridden by a global option
// or explicit call from the user.
var OutputFormat = TextFormat

// Format for logging messages. (text or JSON).
var LogFormat = TextFormat

func FormatJSONLogEntryAsText(text string) string {
	var (
		entry LogEntry
		msg   string
	)

	// Is this a JSON log entry? If not, just return the raw text.
	jsonText := strings.TrimSpace(text)
	if !strings.HasPrefix(jsonText, "{") || !strings.HasSuffix(jsonText, "}") {
		return text
	}

	// Convert the JSON text to a byte array, and unmarshal it into a LogEntry struct.
	err := json.Unmarshal([]byte(jsonText), &entry)
	if err != nil {
		return fmt.Sprintf("Error unmarshalling JSON log entry: %v", err)
	}

	// Format the message with the arguments.
	if len(entry.Args) > 0 {
		msg = i18n.T(entry.Message, entry.Args)
	} else {
		msg = i18n.T(entry.Message)
	}

	// Format the log entry as a string.
	if entry.Session > 0 {
		return fmt.Sprintf("[%s] %4d %10s : [%d] %s", entry.Timestamp, entry.Sequence, strings.ToUpper(entry.Class), entry.Session, msg)
	}

	return fmt.Sprintf("[%s] %4d %10s : %s", entry.Timestamp, entry.Sequence, strings.ToUpper(entry.Class), msg)
}

// formatLogMessage displays a message to stdout.
func formatLogMessage(class int, format string, args ...interface{}) string {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid LogMessage() class %d", class)

		return ""
	}

	// If the format string contains a localization key value but does not start with
	// "log.", add it. This is a convenience feature to allow the code to have shorter
	// localization string keys.
	if strings.Count(format, ".") > 1 && strings.Count(format, " ") == 0 && !strings.HasPrefix(format, "log.") {
		format = "log." + format
	}

	sequenceMux.Lock()
	defer sequenceMux.Unlock()

	sequence = sequence + 1
	sequenceString := fmt.Sprintf("%d", sequence)

	if LogTimeStampFormat == "" {
		LogTimeStampFormat = "2006-01-02 15:04:05"
	}

	if LogFormat != TextFormat {
		text, err := formatJSONLogEntry(class, format, args)
		if err != nil {
			return fmt.Sprintf("Error formatting JSON log entry: %v", err)
		}

		return text
	}

	// Not JSON logging, but let's get the localized version of the log message if there is one.
	// If the argument is a map, use it to localize the message.
	if len(args) > 0 {
		if argMap := getArgMap(args); argMap != nil {
			format = i18n.T(format, argMap)
			args = nil
		} else if argsMap, ok := args[0].(map[string]interface{}); ok {
			format = i18n.T(format, argsMap)
			args = args[1:]
		}
	} else {
		// IF this is a localization string, we'll need to check for an argument map.
		if i18n.T(format) != format {
			if len(args) > 0 {
				if argsMap, ok := args[0].(map[string]interface{}); ok {
					format = i18n.T(format, argsMap)
					args = args[1:]
				}
			} else {
				format = i18n.T(format)
			}
		} else {
			format = i18n.T(format)
		}
	}

	className := loggers[class].name
	s := fmt.Sprintf(format, args...)

	s = fmt.Sprintf("[%s] %-5s %-7s: %s", time.Now().Format(LogTimeStampFormat), sequenceString, className, s)

	return s
}

func formatJSONLogEntry(class int, format string, args []interface{}) (string, error) {
	var (
		err       error
		jsonBytes []byte
	)

	entry := LogEntry{
		Timestamp: time.Now().Format(LogTimeStampFormat),
		Class:     strings.ToLower(loggers[class].name),
		ID:        defs.InstanceID,
		Sequence:  sequence,
	}

	// Is this a message with a localized value?
	if i18n.T(format) != format {
		entry.Message = format
		hasImpliedArgumentMap := false

		// Did the caller give us an even number of arguments where the even number items are
		// all strings? If so, this is treated as a parameter map.
		if len(args)%2 == 0 && len(args) > 1 {
			hasImpliedArgumentMap = true

			for n, arg := range args {
				if n%2 == 0 {
					if _, ok := arg.(string); !ok {
						hasImpliedArgumentMap = false

						break
					}
				}
			}

			if hasImpliedArgumentMap {
				entry.Args = make(map[string]interface{})

				for n := 0; n < len(args); n += 2 {
					entry.Args[args[n].(string)] = args[n+1]
				}
			}
		}

		// Look for a map which will be used for args, or an integer which is the thread number
		if !hasImpliedArgumentMap {
			for n, arg := range args {
				if argsMap, ok := arg.(map[string]interface{}); ok {
					entry.Args = argsMap

					continue
				}

				if thread, ok := arg.(int); ok {
					entry.Session = thread

					continue
				}

				if entry.Args == nil {
					entry.Args = make(map[string]interface{})
				}

				entry.Args[fmt.Sprintf("arg%d", n+1)] = arg
			}
		}
	} else {
		// Not a formatted log message. But, if it starts with a thread id, then extract it and format the message accordingly
		format = strings.TrimSpace(format)
		if strings.HasPrefix(format, "[%d] ") {
			if thread, ok := args[0].(int); ok {
				format = format[3:]
				entry.Session = thread
				args = args[1:]
			}
		}

		entry.Message = fmt.Sprintf(format, args...)
	}

	// Format the entry as a JSON string
	if LogFormat == JSONFormat {
		jsonBytes, err = json.Marshal(entry)
	} else {
		jsonBytes, err = json.MarshalIndent(entry, "", "  ")
	}

	return string(jsonBytes), err
}

// Helper function to determine if the given argument list can be used as a parameter map.
func getArgMap(args []interface{}) map[string]interface{} {
	var key string

	if len(args) < 2 || len(args)%2 != 0 {
		return nil
	}

	result := make(map[string]interface{})

	for i, arg := range args {
		if i%2 == 0 {
			if s, ok := arg.(string); ok {
				key = s
			} else {
				return nil
			}
		}

		result[key] = arg
	}

	return result
}
