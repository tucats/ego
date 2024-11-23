package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/tucats/ego/errors"
)

// ParseDuration parses a duration string and returns the corresponding time.Duration.
// It handles Ego extensions that allow specifying days in a duration, which are
// converted to hours before using the builtin time.ParseDuration function.
func ParseDuration(durationString string) (time.Duration, error) {
	var (
		days  int
		hours int
		mins  int
		secs  int
		chars string
		err   error
	)

	for _, ch := range durationString {
		switch ch {
		case 'd':
			if chars != "" {
				days, err = strconv.Atoi(chars)
				if err != nil {
					return 0, errors.New(err).Context(chars)
				}
				chars = ""
			}

		case 'h':
			if chars != "" {
				hours, err = strconv.Atoi(chars)
				if err != nil {
					return 0, errors.New(err).Context(chars)
				}
				chars = ""
			}

		case 'm':
			if chars != "" {
				mins, err = strconv.Atoi(chars)
				if err != nil {
					return 0, errors.New(err).Context(chars)
				}
				chars = ""
			}

		case 's':
			if chars != "" {
				secs, err = strconv.Atoi(chars)
				if err != nil {
					return 0, errors.New(err).Context(chars)
				}
				chars = ""
			}

		default:
			if !unicode.IsSpace(ch) {
				chars += string(ch)
			}
		}
	}

	// Now reconstruct a Go native duration string using the parsed values. First,
	// merge days into the hours value.

	hours += days * 24
	result := fmt.Sprintf("%dh%dm%ds", hours, mins, secs)

	// Convert the interval string to a time.Duration value
	duration, err := time.ParseDuration(result)

	return duration, err
}

// Function to format a duration. By default, this behaves like the standard
// time.Duration string formatter. If the extendedFormat flag is used and the
// duration is greater than or equal to one second, the  output is formatted
// with days as well as hours, minutes, and  seconds.
//
// Additionally, in this format, additional spacing is used between the terms
// for improved readability. This function is used to express how long a server
// has been running, for example.
func FormatDuration(d time.Duration, extendedFormat bool) string {
	if !extendedFormat {
		return d.String()
	}

	if d == 0 {
		return "0s"
	}

	// If this is a very small duration that is less than a second, use the
	// default formatter.
	if math.Abs(float64(d)) < float64(time.Second) {
		return d.String()
	}

	// Otherwise, lets build a formatted duration string.
	var result strings.Builder

	// if the duration is negative, add a sign and make the remaining duration
	// a positive value.
	if d < 0 {
		result.WriteRune('-')

		d = -d
	}

	// If the number of hours is greater than a day, extract the number of days
	// as a separate part of the duration string.
	if hours := int(d.Hours()); hours > 0 {
		if hours > 23 {
			result.WriteString(fmt.Sprintf("%dd", hours/24))
			hours = hours % 24
		}

		// If the remaining number of hours is greater than 0, add the hours
		if hours > 0 {
			if result.Len() > 1 {
				result.WriteRune(' ')
			}

			result.WriteString(fmt.Sprintf("%dh", hours))
		}
	}

	// If there are more than 0 minutes, add the minutes.
	minutes := int64(d.Minutes()) % 60
	if minutes >= 1 {
		if result.Len() > 1 {
			result.WriteRune(' ')
		}

		result.WriteString(fmt.Sprintf("%dm", minutes))
	}

	// If there are more than 1 second, add the seconds.
	seconds := int64(d.Seconds()) % 60
	if seconds >= 1 {
		if result.Len() > 1 {
			result.WriteRune(' ')
		}

		result.WriteString(fmt.Sprintf("%ds", seconds))
	}

	return result.String()
}
