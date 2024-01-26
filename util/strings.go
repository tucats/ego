package util

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/tucats/ego/data"
)

// Unquote removes quotation marks from a string if present.
func Unquote(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = strings.TrimPrefix(strings.TrimSuffix(s, "\""), "\"")
	}

	return s
}

// HasCapitalizedName returns true if the first rune/character of the
// string is considered a capital letter in Unicode.
func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

// Hostname gets a short form of the host namme (i.e. the first part of an FQDN).
func Hostname() string {
	if hostName, err := os.Hostname(); err == nil {
		result := ""

		for _, ch := range hostName {
			if ch == '.' {
				break
			}

			result = result + string(ch)
		}

		return result
	} else {
		return "<unknown hostname>"
	}
}

// Given a list of strings, convert them to a sorted list in
// Ego array format.
func MakeSortedArray(array []string) *data.Array {
	sort.Strings(array)

	intermediateArray := make([]interface{}, len(array))

	for i, v := range array {
		intermediateArray[i] = v
	}

	result := data.NewArrayFromInterfaces(data.StringType, intermediateArray...)

	return result
}

func InterfaceMapKeys(data map[string]interface{}) []string {
	keys := make([]string, 0)

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// Session log is used to take a multi-line message for the server log,
// and insert prefixes on each line with the session number so the log
// lines will be tagged with the appropriate session identifier, and
// can be read with the server log query for a specific session.
func SessionLog(id int, text string) string {
	lines := strings.Split(text, "\n")

	for n := 0; n < len(lines); n++ {
		lines[n] = fmt.Sprintf("%35s: [%d] %s", " ", id, lines[n])
	}

	return strings.Join(lines, "\n")
}

// Gibberish returns a string of lower-case characters and digits representing the
// UUID value converted to base 32. The resulting string will never contain the 
// letters "o" or "l" to avoid confusion with digits 0 and 1.
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
