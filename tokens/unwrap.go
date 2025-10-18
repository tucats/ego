package tokens

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// extract extracts the data from a token and returns it as a struct.
func Unwrap(tokenString string, session int) (*Token, error) {
	var (
		err         error
		blacklisted bool
		t           = Token{}
	)

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(tokenString)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	// Decrypt the token into a json string. We use the token key stored in
	// the preferences data. If there isn't one, generate a new random key.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.decrypt", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	if len(j) == 0 {
		return nil, errors.ErrInvalidTokenEncryption.In("Extract")
	}

	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired", ui.A{
			"session": session,
			"id":      t.TokenID})

		err = errors.ErrExpiredToken.In("Extract")
	}

	// See if this is blacklisted before we continue.
	if err == nil {
		blacklisted, err = IsBlacklisted(t)
		if blacklisted {
			err = errors.ErrInvalidTokenEncryption.In("Validate")

			ui.Log(ui.AuthLogger, "auth.blacklisted", ui.A{
				"session": session,
				"id":      t.TokenID.String()})
		}
	}

	if err != nil {
		return nil, errors.New(err)
	}

	return &t, err
}
