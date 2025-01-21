package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func Tail(count int, session int) ([]string, error) {
	if logFile == nil {
		return []string{}, nil
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
			// More common case; the logline is a JSON representation of a log entry. If so,
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
