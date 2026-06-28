package dsns

import (
	"encoding/hex"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util"
)

const salt = "f4b3eead"

// encrypt implements the password encryption function, which encrypts a plaintext
// password into an encrypted hex string, using the server's private encryption token.
func encrypt(data string) (string, error) {
	key := settings.Get(defs.ServerTokenKeySetting) + salt

	b, err := util.Encrypt(data, key)
	if err != nil {
		return b, err
	}

	reply := hex.EncodeToString([]byte(b))

	return reply, nil
}
