package util

import (
	"os"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
)

// Hostname gets a short form of the host name (i.e. the first part of an FQDN).
// For example, this is used as the server name in REST response payloads.
func Hostname() string {
	if hostName, err := os.Hostname(); err == nil {
		// If this is a multipart name, use only the first part.
		// If this is not a multipart name, return the entire name.
		// Note: This assumes that the FQDN will contain multiple
		// dots, which is a common scenario.
		if !settings.GetBool(defs.ServerReportFQDNSetting) {
			hostName = strings.SplitN(hostName, ".", 2)[0]
		}

		return hostName
	} else {
		return "<unknown hostname>"
	}
}

// Given a list of strings, convert them to a sorted list in Ego array format.
// The array of strings is sorted in lexicographical order and then copied
// to an array of any values. Finally, the generic function for creating
// an Ego array from an array of any values is called, specifying that the
// array is a string array.
func MakeSortedArray(array []string) *data.Array {
	sort.Strings(array)

	intermediateArray := make([]any, len(array))

	for i, v := range array {
		intermediateArray[i] = v
	}

	result := data.NewArrayFromInterfaces(data.StringType, intermediateArray...)

	return result
}

// ConvertMapKeys returns a sorted list of keys from a map where the keys
// are all strings. The keys are sorted in lexicographical order and returned
// as a string array.
func InterfaceMapKeys(data map[string]any) []string {
	keys := make([]string, 0)

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
