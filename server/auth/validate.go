package auth

import (
	"crypto/sha256"
	"strconv"
	"strings"
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
			realPass = HashString(realPass[1 : len(realPass)-1])
		}

		hashPass := HashString(pass)
		ok = realPass == hashPass

		if findPermission(u, "root") < 0 && findPermission(u, "logon") < 0 {
			ok = false
		}
	}

	return ok
}

// HashString converts a given string to it's hash. This is used to manage
// passwords as opaque objects.
func HashString(s string) string {
	var r strings.Builder

	h := sha256.New()
	_, _ = h.Write([]byte(s))

	v := h.Sum(nil)
	for _, b := range v {
		// Format the byte. It must be two digits long, so if it was a
		// value less than 0x10, add a leading zero.
		byteString := strconv.FormatInt(int64(b), 16)
		if len(byteString) < 2 {
			byteString = "0" + byteString
		}

		r.WriteString(byteString)
	}

	return r.String()
}
