package ui

import (
	"bufio"
	"os"
)

func Tail(count int) []string {
	if logFile == nil {
		return nil
	}

	file, err := os.OpenFile(logFile.Name(), os.O_RDWR, 0700)

	if err != nil {
		return nil
	}

	text := []string{}
	scanner := bufio.NewScanner(file)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	// The method os.File.Close() is called
	// on the os.File object to close the file
	file.Close()

	position := len(text) - count
	if position < 0 {
		position = 0
	}

	return text[position:]
}
