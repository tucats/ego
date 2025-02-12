package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// A is the type of the argument list to a log entry. This is a map of each argument with
// an arbitrary value that will be stored in the log entry.
type A map[string]interface{}

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

	// If there is a session number, add it to the args map.
	if entry.Session > 0 {
		if entry.Args == nil {
			entry.Args = make(map[string]interface{})
		}

		entry.Args["session"] = strconv.Itoa(entry.Session)
	}

	// Format the log entry as a string.
	// Format the message with the arguments.
	if len(entry.Args) > 0 {
		msg = i18n.T(entry.Message, entry.Args)
	} else {
		msg = i18n.T(entry.Message)
	}

	// If there's a session number in the log entry but not provided in the message text, add it now.
	if entry.Session > 0 && !strings.HasPrefix(msg, "[") {
		msg = fmt.Sprintf("[%d] %s", entry.Session, msg)
	}

	// Format the log entry as a string.
	return fmt.Sprintf("[%s] %4d %10s : %s", entry.Timestamp, entry.Sequence, strings.ToUpper(entry.Class), msg)
}

// formatLogMessage displays a message to stdout.
func formatLogMessage(class int, message string, args A) string {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid LogMessage() class %d", A{
			"class": class})

		return ""
	}

	// If the format string contains a localization key value but does not start with
	// "log.", add it. This is a convenience feature to allow the code to have shorter
	// localization string keys.
	if strings.Count(message, ".") > 0 && strings.Count(message, " ") == 0 && !strings.HasPrefix(message, "log.") {
		message = "log." + message
	}

	sequenceMux.Lock()
	defer sequenceMux.Unlock()

	sequence = sequence + 1
	sequenceString := fmt.Sprintf("%d", sequence)

	if LogTimeStampFormat == "" {
		LogTimeStampFormat = "2006-01-02 15:04:05"
	}

	if LogFormat != TextFormat {
		text, err := formatJSONLogEntry(class, message, args)
		if err != nil {
			return fmt.Sprintf("Error formatting JSON log entry: %v", err)
		}

		return text
	}

	// Not JSON logging, but let's get the localized version of the log message if there is one.
	// If the argument is a map, use it to localize the message.
	if len(args) > 0 {
		message = i18n.T(message, args)
	} else {
		message = i18n.T(message)
	}

	// Was there a session argument without expicitly putting it at the start of the text? If so,
	// let's add it now.
	if session, found := args["session"]; found {
		if sessionInt, ok := session.(int); ok && !strings.HasPrefix(message, "[") {
			message = fmt.Sprintf("[%d] %s", sessionInt, message)
		}
	}

	// Format the message with timestammp, sequence, class, and message.
	s := fmt.Sprintf("[%s] %-5s %-7s: %s", time.Now().Format(LogTimeStampFormat), sequenceString, loggers[class].name, message)

	return s
}

func formatJSONLogEntry(class int, format string, args A) (string, error) {
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

		// Look for a map which will be used for args, or an integer which is the thread number
		if len(args) > 0 {
			entry.Args = map[string]interface{}{}
			for k, v := range args {
				entry.Args[k] = v
			}
		} else {
			entry.Args = make(map[string]interface{})
		}
	} else {
		// Not a formatted log message.
		format = strings.TrimSpace(format)
		entry.Message = fmt.Sprintf(format, args)
	}

	// Was there a session argument? If so, we need to hoist that to the entry object as an integer session id
	if entry.Session == 0 && len(entry.Args) > 0 {
		session, ok := entry.Args["session"]
		if ok {
			s := fmt.Sprintf("%v", session)
			i, err := strconv.Atoi(s)

			entry.Session = i
			if err == nil {
				delete(entry.Args, "session")
			}
		}
	}

	// Format the entry as a JSON string
	if LogFormat == JSONFormat {
		jsonBytes, err = json.Marshal(entry)
	} else {
		jsonBytes, err = json.MarshalIndent(entry, JSONIndentPrefix, JSONIndentSpacer)
	}

	return string(jsonBytes), err
}

// Helper function to determine if the given argument list can be used as a parameter map.
func getArgMap(args []interface{}) map[string]interface{} {
	var key string

	if len(args) == 1 {
		if m, ok := args[0].(map[string]interface{}); ok {
			return m
		}

		// It's in the hidden type of A, convert to a conventional map.
		if m, ok := args[0].(A); ok {
			result := make(map[string]interface{})
			for k, v := range m {
				result[k] = v
			}

			return result
		}
	}

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
