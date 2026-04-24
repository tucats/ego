package tokens

import (
	"os"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
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
//  3. A freshly generated random key — if neither of the above is set, two
//     random UUIDs are concatenated (giving a 73-character string) and saved
//     to the settings file for future use.
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
		key = uuid.New().String() + "-" + uuid.New().String()

		settings.Set(defs.ServerTokenKeySetting, key)
		_ = settings.Save() // best-effort; failure is non-fatal
	}

	return key
}
