package util

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/tucats/ego/data"
)

// Escape escapes special characters in a string. This uses the
// JSON marshaller to create a suitable string value.
func Escape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	// Trim the beginning and trailing " character
	return string(b[1 : len(b)-1])
}

// Unquote removes quotation marks from a string if present. This
// identifies any escaped quotes (preceded by a backslash) and removes
// the backslashes.
func Unquote(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = strings.TrimPrefix(strings.TrimSuffix(s, "\""), "\"")
	}

	return s
}

// HasCapitalizedName returns true if the first rune/character of the
// string is considered a capital letter in Unicode. This works even
// with Unicode characters that are not letters.
func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

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
// as if they were discrete log entries
func SessionLog(id int, text string) string {
	lines := strings.Split(text, "\n")

	for n := 0; n < len(lines); n++ {
		lines[n] = fmt.Sprintf("%35s: [%d] %s", " ", id, lines[n])
	}

	return strings.Join(lines, "\n")
}

// Gibberish returns a string of lower-case characters and digits representing the
// UUID value converted to base 32.
//
// The resulting string is meant to be human-readible with minimum errors, but also to
// take up the fewest number of characters. As such, the resulting string will never
// contain the letters "o" or "l" to avoid confusion with digits 0 and 1.
func Gibberish(u uuid.UUID) string {
	var result strings.Builder

	digits := []byte("abcdefghjkmnpqrstuvwxyz23456789")
	radix := uint64(len(digits))

	// Make two 64-bit integers from the UUID value
	var hi, low uint64

	for i := 0; i < 8; i++ {
		hi = hi<<8 + uint64(u[i])
	}

	for i := 9; i < 16; i++ {
		low = low<<8 + uint64(u[i])
	}

	for low > 0 {
		result.WriteByte(digits[low%radix])
		low = low / radix
	}

	for hi > 0 {
		result.WriteByte(digits[hi%radix])
		hi = hi / radix
	}

	text := result.String()
	if len(text) == 0 {
		return "-empty-"
	}

	return text
}
