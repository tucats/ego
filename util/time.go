package util

import (
	"strconv"
	"strings"
	"time"
)

// ParseDuration parses a duration string and returns the corresponding time.Duration.
// It handles Ego extensions that allow specifying days as hours.
func ParseDuration(durationString string) (time.Duration, error) {
	// Is this duration string specified in hours?  For example, "1d" would be
	// equivalent to "1440h".
	if days, err := strconv.Atoi(strings.TrimSuffix(durationString, "d")); err == nil {
		durationString = strconv.Itoa(days*24) + "h"
	}

	// Convert the interval string to a time.Duration value
	duration, err := time.ParseDuration(durationString)

	return duration, err
}
