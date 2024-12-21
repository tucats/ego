package util

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tucats/ego/data"
)

// Hostname gets a short form of the host namme (i.e. the first part of an FQDN).
// For example, this is used as the server name in REST response payloads.
func Hostname() string {
	if hostName, err := os.Hostname(); err == nil {
		// If this is a multipart name, use only the first part.
		// If this is not a multipart name, return the entire name.
		// Note: This assumes that the FQDN will not contain multiple
		// dots, which is a common scenario.
		if strings.Count(hostName, ".") > 1 {
			parts := strings.SplitN(hostName, ".", 2)

			return parts[0]
		} else {
			return hostName
		}
	} else {
		return "<unknown hostname>"
	}
}

// Given a list of strings, convert them to a sorted list in Ego array format.
// The array of strings is sorted in lexicographical order and then copied
// to an array of interface{} values. Finally, the generic function for creating
// an Ego array from an array of interface{} values is called, specifying that the
// array is a string array.
func MakeSortedArray(array []string) *data.Array {
	sort.Strings(array)

	intermediateArray := make([]interface{}, len(array))

	for i, v := range array {
		intermediateArray[i] = v
	}

	result := data.NewArrayFromInterfaces(data.StringType, intermediateArray...)

	return result
}

// ConvertMapKeys returns a sorted list of keys from a map where the keys
// are all strings. The keys are sorted in lexicographical order and returned
// as a string array.
func InterfaceMapKeys(data map[string]interface{}) []string {
	keys := make([]string, 0)

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// SessionLog is used to take a multi-line message for the server log,
// and insert prefixes on each line with the session number so the log
// lines will be tagged with the appropriate session identifier, and
// can be read with the server log query for a specific session.
//
// This allows messages that are quite complex to appear in the log
// as if they were discrete log entries.
func SessionLog(id int, text string) string {
	lines := strings.Split(text, "\n")

	for n := 0; n < len(lines); n++ {
		lines[n] = fmt.Sprintf("%35s: [%d] %s", " ", id, lines[n])
	}

	return strings.Join(lines, "\n")
}
