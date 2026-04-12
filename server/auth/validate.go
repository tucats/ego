package auth

import (
	"crypto/subtle"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"golang.org/x/crypto/bcrypt"
)

// ValidatePassword checks a username and password against the database and
// returns true if the user exists and the password is valid.
//
// Stored passwords may be in one of three legacy formats that are recognized
// and transparently migrated to bcrypt on first successful login:
//
//   - bcrypt hash ($2a$/$2b$/$2y$ prefix) — compared with bcrypt.CompareHashAndPassword.
//   - legacy {quoted} plaintext — inner value is SHA-256 hashed then compared.
//   - legacy bare SHA-256 hex — compared using constant-time equality.
//
// When a legacy hash matches, the record is immediately upgraded to bcrypt and
// written back to the auth store so the user's next login uses the stronger
// algorithm.
func ValidatePassword(session int, user, pass string) bool {
	ok := false

	if user == "" || pass == "" {
		return false
	}

	user = strings.ToLower(user)

	if u, userExists := AuthService.ReadUser(session, user, false); userExists == nil {
		realPass := u.Password

		if IsBcryptHash(realPass) {
			// Current format: let bcrypt handle the comparison.
			ok = bcrypt.CompareHashAndPassword([]byte(realPass), []byte(pass)) == nil
		} else {
			// Legacy format. normalize the stored value to a SHA-256 hex string
			// so the comparison below has a uniform shape.
			if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
				// Quoted plaintext: warn that the account carries a legacy format,
				// then hash the inner value for comparison. A successful login below
				// will immediately migrate the stored hash to bcrypt (one-time use).
				ui.Log(ui.AuthLogger, "auth.password.plaintext", ui.A{
					"session": session,
					"user":    user})

				realPass = egostrings.HashString(realPass[1 : len(realPass)-1])
			}

			hashPass := egostrings.HashString(pass)
			ok = subtle.ConstantTimeCompare([]byte(realPass), []byte(hashPass)) == 1

			// On a successful legacy match, upgrade the stored hash to bcrypt so
			// the user's next login uses the stronger algorithm. A failure here is
			// non-fatal: the login still succeeds; the upgrade will be retried on
			// the next login.
			if ok {
				if newHash, err := HashPassword(pass); err == nil {
					u.Password = newHash

					if writeErr := AuthService.WriteUser(session, u); writeErr == nil {
						_ = AuthService.Flush()

						ui.Log(ui.AuthLogger, "auth.password.migrated", ui.A{
							"session": session,
							"user":    user})
					}
				}
			}
		}

		if ok && findPermission(u, defs.RootPermission) < 0 && findPermission(u, defs.LogonPermission) < 0 {
			ok = false
		}
	}

	return ok
}
