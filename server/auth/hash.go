package auth

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor passed to bcrypt. 12 is the OWASP-recommended
// minimum for interactive logins as of 2024. Increasing this value makes each
// hash slower to compute (by a factor of 2 per increment), which protects
// against brute-force attacks at the cost of slightly slower logins.
const bcryptCost = 12

// HashPassword creates a bcrypt hash of the given plaintext password.
// An error is only returned when the platform random-number generator
// fails, which is a non-recoverable condition.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// IsBcryptHash reports whether hash is a bcrypt hash. bcrypt hashes always
// begin with the Blowfish version prefix "$2a$", "$2b$", or "$2y$".
func IsBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") ||
		strings.HasPrefix(hash, "$2b$") ||
		strings.HasPrefix(hash, "$2y$")
}

// mustHashPassword is a startup-only wrapper around HashPassword that panics
// on error. It is reserved for initialization paths (default user creation)
// where the random-number generator failing would prevent secure operation
// entirely.
func mustHashPassword(password string) string {
	h, err := HashPassword(password)
	if err != nil {
		panic("auth: failed to hash password: " + err.Error())
	}

	return h
}
