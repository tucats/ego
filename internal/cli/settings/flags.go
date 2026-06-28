package settings

import (
	"strconv"
	"strings"
)

var flags map[int]bool

// For a given numeric feature flag value, return true if the corresponding
// flag was set in the eog.feature.flags environment variable.
func Flag(index int) bool {
	if flags == nil {
		flags = make(map[int]bool)

		flagString := Get("ego.feature.flags")
		flagSet := strings.Split(flagString, ",")

		for _, flag := range flagSet {
			if flagID, err := strconv.Atoi(flag); err == nil {
				flags[flagID] = true
			}
		}
	}

	return flags[index]
}
