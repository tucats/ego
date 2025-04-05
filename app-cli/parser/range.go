package parser

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

func parseSequence(s string) ([]int, error) {
	var err error

	result := make([]int, 0)

	// Step 1, normalize "-" and ":" characters.
	s = strings.ReplaceAll(s, "-", ":")

	// Step 2, determine sets of values or ranges.
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if strings.Contains(part, ":") {
			// This is a range.
			start, end, err := parseRange(part)
			if err != nil {
				return nil, err
			}

			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// This is a single value.
			i, err := strconv.Atoi(part)
			if err != nil {
				return nil, errors.ErrInvalidInteger.Clone().Context(part)
			}

			result = append(result, i)
		}
	}

	return result, err
}

func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.ErrInvalidRange.Context(s)
	}

	if len(strings.TrimSpace(parts[0])) == 0 {
		parts[0] = "1"
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[0])
	}

	if len(strings.TrimSpace(parts[1])) == 0 {
		parts[1] = strconv.FormatInt(int64(start+9), 10)
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[1])
	}

	if start > end {
		return 0, 0, errors.ErrInvalidRange.Clone().Context(s)
	}

	return start, end, nil
}
