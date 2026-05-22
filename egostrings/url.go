package egostrings

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/defs"
)

// These are the URL schemes that Ego ever expects to encounter in connection strings.
var supportedSchemes = []string{
	defs.PostgresProvider,
	defs.SqliteProvider,
	defs.DeprecatedSqliteProvider,
	"http",
	"https",
	"file",
	"dsn",
	"config",
}

// FindScheme looks for a supported URL scheme in the string and returns it if found. If no
// supported scheme is found, an error is returned.
func FindScheme(s string) (string, error) {
	var (
		err error
	)

	s = strings.TrimSpace(strings.ToLower(s))

	for _, scheme := range supportedSchemes {
		if strings.HasPrefix(s, scheme+"://") {
			return scheme, nil
		}
	}

	err = fmt.Errorf("unsupported URL scheme in %q", s)

	return "", err
}

// StripScheme removes the URL scheme prefix from the string and returns the remainder.
// If the string does not have a supported URL scheme, the string is returned as-is.
func StripScheme(s string) string {
	s = strings.TrimSpace(s)

	scheme, err := FindScheme(s)
	if err != nil {
		return s
	}

	// The string could be oddly-cased, so strip off by length.
	return s[len(scheme)+len("://"):]
}
