package tokens

import (
	"os"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// getTokenKey fetches the key used to encrypt tokens. If it
// was not already set up, a new random one is generated.
func getTokenKey() string {
	key := os.Getenv("EGO_SERVER_TOKEN_KEY")
	if key == "" {
		key = settings.Get(defs.ServerTokenKeySetting)
	}

	if key == "" {
		key = uuid.New().String() + "-" + uuid.New().String()

		settings.Set(defs.ServerTokenKeySetting, key)
		_ = settings.Save()
	}

	return key
}
