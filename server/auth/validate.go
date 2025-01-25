package auth

import (
	"crypto/sha256"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
)

// ValidatePassword checks a username and password against the database and
// returns true if the user exists and the password is valid.
func ValidatePassword(user, pass string) bool {
	ok := false

	if u, userExists := AuthService.ReadUser(user, false); userExists == nil {
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

// validateToken is a helper function that calls the builtin cipher.Validate().
func ValidateToken(t string) bool {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		_, err := remoteUser(authServer, t)

		return err == nil
	}

	// We must be the authority, so use our local authentication service.
	s := symbols.NewSymbolTable("validate")
	runtime.AddPackages(s)

	v, err := builtins.CallBuiltin(s, "cipher.Validate", t, true)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.token.error", ui.A{
			"error": err})

		return false
	}

	if v == nil {
		return false
	}

	return data.BoolOrFalse(v)
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
