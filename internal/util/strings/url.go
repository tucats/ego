package egostrings

import (
	"net/url"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
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

	err = errors.New(errors.ErrUnsupportedURLScheme).Context(s)

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

// URLPassword looks for a password in the URL string and returns it if found,
// along with a boolean indicating whether it was found. The password is expected
// to be in the form "username:password@" within the URL. IF the URL isn't a
// valid URL string, we return an empty string and false.
func URLPassword(s string) (string, bool) {
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}

	if u.User != nil {
		if p, found := u.User.Password(); found {
			return p, true
		}
	}

	return "", false
}
