package dsns

import (
	"encoding/hex"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
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

// decrypt implements the password decryption function, which decrypts an encrypted
// hex string back to a plaintext password, using the server's private encryption token.
func decrypt(text string) (string, error) {
	b, err := hex.DecodeString(text)
	if err != nil {
		return "", errors.New(err)
	}

	key := settings.Get(defs.ServerTokenKeySetting) + salt
	result, err := util.Decrypt(string(b), key)

	return result, err
}
