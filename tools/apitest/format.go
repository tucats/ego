package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func FormatDuration(d time.Duration, extendedFormat bool) string {
	if !extendedFormat {
		return d.String()
	}

	if d == 0 {
		return "0s"
	}

	// If this is a very small duration that is less than a second, use the
	// default formatter.
	if v := math.Abs(float64(d)); v < float64(time.Second) {
		if v > float64(time.Second) {
			return fmt.Sprintf("%6.2fs", v/float64(time.Second))
		}

		if v > float64(time.Millisecond) {
			return fmt.Sprintf("%6.2fms", v/float64(time.Millisecond))
		}

		if v > float64(time.Microsecond) {
			return fmt.Sprintf("%6.2fµs", v/(float64(time.Microsecond)))
		}

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
