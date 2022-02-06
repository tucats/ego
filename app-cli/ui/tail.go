package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Tail(count int, session int) []string {
	if logFile == nil {
		return nil
	}

	file, err := os.OpenFile(logFile.Name(), os.O_RDWR, 0700)
	if err != nil {
		return nil
	}

	defer file.Close()

	text := []string{}
	scanner := bufio.NewScanner(file)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()

		if session > 0 {
			pattern := fmt.Sprintf(": [%d] ", session)

			if !strings.Contains(line, pattern) {
				continue
			}
		}

		text = append(text, scanner.Text())
	}

	position := len(text) - count
	if position < 0 {
		position = 0
	}

	return text[position:]
}
