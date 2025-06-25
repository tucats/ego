package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tucats/ego/defs"
)

func Tail(count int, session int) ([]string, error) {
	// If there is no active log file, manufacture empty log message saying so.
	if logFile == nil {
		if count > 3 {
			count = 3
		}

		if count == 0 {
			count = 1
		}

		if LogTimeStampFormat == "" {
			LogTimeStampFormat = "2006-01-02 15:04:05"
		}

		result := []string{}

		for i := range count {
			var entry struct {
				Time  string `json:"time"`
				ID    string `json:"id"`
				Seq   int    `json:"seq"`
				Class string `json:"class"`
				Msg   string `json:"msg"`
			}

			entry.Time = time.Now().Format(LogTimeStampFormat)
			entry.ID = defs.InstanceID
			entry.Seq = i + 1
			entry.Class = "server"
			entry.Msg = "no.log"

			b, _ := json.Marshal(entry)
			result = append(result, string(b))
		}

		return result, nil
	}

	file, err := os.OpenFile(logFile.Name(), os.O_RDONLY, 0700)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	text := []string{}
	scanner := bufio.NewScanner(file)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()

		if session > 0 {
			// More common case; the log line is a JSON representation of a log entry. If so,
			// unmarshal it and see if the session ID matches.
			if LogFormat == JSONFormat {
				var entry LogEntry

				err := json.Unmarshal([]byte(line), &entry)
				if err == nil && entry.Session != session {
					continue
				}
			} else {
				// Brute force search of the log line to see if it has the tell-tale text session ID.
				pattern := fmt.Sprintf(": [%d] ", session)

				if !strings.Contains(line, pattern) {
					continue
				}
			}
		}

		text = append(text, scanner.Text())
	}

	position := len(text) - count
	if position < 0 {
		position = 0
	}

	return text[position:], nil
}
