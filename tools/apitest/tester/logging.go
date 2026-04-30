package tester

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tucats/apitest/logging"
)

type contentType int

const (
	unknownContent contentType = iota
	jsonContent
	textContent
)

func restLog(heading string, b []byte, kind contentType) {
	if !logging.Rest || len(b) == 0 {
		return
	}

	switch kind {
	case jsonContent:
		restJSONLog(heading, b)
	case textContent:
		restTextLog(heading, b)
	default:
		restBytesLog(heading, b)
	}
}

func restBytesLog(heading string, b []byte) {
	lineCount := 0
	position := 0
	start := 0

	fmt.Printf("  Binary %s\n", heading)

	for lineCount < 4 {
		buffer := ""
		for len(buffer) < 80 && position < len(b) {
			buffer += fmt.Sprintf("%02x", b[position])
		}

		fmt.Printf("    %3d: %s\n", start, buffer)
		start = position
	}
}

func restTextLog(heading string, b []byte) {
	var lines []string

	lines = strings.Split(string(b), "\n")

	fmt.Printf("  Text %s\n", heading)

	for _, line := range lines {
		fmt.Printf("    %s\n", line)
	}
}
func restJSONLog(heading string, b []byte) {
	var data interface{}

	err := json.Unmarshal(b, &data)
	if err == nil {
		formatted, _ := json.MarshalIndent(data, "    ", "  ")

		fmt.Printf("  JSON %s:\n    %s\n", heading, formatted)
	}
}
