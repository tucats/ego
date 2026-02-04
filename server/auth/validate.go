package auth

import (
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
)

// ValidatePassword checks a username and password against the database and
// returns true if the user exists and the password is valid.
func ValidatePassword(session int, user, pass string) bool {
	ok := false

	if user == "" || pass == "" {
		return false
	}

	if u, userExists := AuthService.ReadUser(session, user, false); userExists == nil {
		realPass := u.Password
		// If the password in the database is quoted, do a local hash
		if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
			realPass = egostrings.HashString(realPass[1 : len(realPass)-1])
		}

		hashPass := egostrings.HashString(pass)
		ok = realPass == hashPass

		if findPermission(u, defs.RootPermission) < 0 && findPermission(u, defs.LogonPermission) < 0 {
			ok = false
		}
	}

	return ok
}
