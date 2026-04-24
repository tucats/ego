package tokens

import (
	"crypto/rand"
	"math/big"
	"os"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

const (
	// keyAlphabet is the set of characters allowed in a generated token key.
	keyAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	// keyLength is the number of characters in a generated token key.
	// 128 characters from a 62-character alphabet gives ~760 bits of entropy.
	keyLength = 128
)

// getTokenKey returns the symmetric encryption key used to encrypt and decrypt
// all tokens. The key is resolved in this priority order:
//
//  1. The EGO_SERVER_TOKEN_KEY environment variable — highest priority; useful
//     for injecting a key in containerized or CI environments without touching
//     settings files.
//
//  2. The defs.ServerTokenKeySetting application setting — persisted across
//     server restarts in the user's settings file.
//
//  3. A freshly generated random key — if neither of the above is set, a
//     128-character string is generated using crypto/rand, sampling from the
//     62-character alphanumeric alphabet (~760 bits of entropy). The key is
//     saved to the settings file so the same key is reused on the next call.
//
// Why a stable key matters: tokens are encrypted with this key. If the key
// changes between the call to New() and a later call to Unwrap() or
// Validate(), the decryption will fail and every token issued under the old
// key becomes invalid. In production, pin the key via the environment variable
// or ensure the settings file is persisted.
func getTokenKey() string {
	// Check the environment first so that callers can override without touching
	// the persisted settings.
	key := os.Getenv("EGO_SERVER_TOKEN_KEY")
	if key == "" {
		key = settings.Get(defs.ServerTokenKeySetting)
	}

	// No key found anywhere — generate a new one and save it so that the same
	// key is reused on the next call within this process and after a restart.
	if key == "" {
		key = randomKey(keyLength)

		settings.Set(defs.ServerTokenKeySetting, key)
		_ = settings.Save() // best-effort; failure is non-fatal
	}

	return key
}

// randomKey generates a cryptographically random string of exactly n characters
// drawn from keyAlphabet (A-Z, a-z, 0-9). It panics only if the system's
// random source is broken — a condition that indicates a fundamental OS failure.
func randomKey(n int) string {
	alphabetLen := big.NewInt(int64(len(keyAlphabet)))
	buf := make([]byte, n)

	for i := range buf {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			panic("tokens: crypto/rand unavailable: " + err.Error())
		}

		buf[i] = keyAlphabet[idx.Int64()]
	}

	return string(buf)
}
