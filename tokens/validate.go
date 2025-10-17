package tokens

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// validate determines if a token is valid and returns true/false.
func Validate(tokenString string, session int) (bool, error) {
	var (
		err         error
		blacklisted bool
		reportErr   bool
		t           = Token{}
	)

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(tokenString)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"session": session,
			"error":   errors.New(err)})

		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	// Decrypt the token into a json string
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil || len(j) == 0 {
		ui.Log(ui.AuthLogger, "auth.invalid.decryption", ui.A{
			"session": session,
			"error":   errors.New(err)})

		err = errors.ErrInvalidTokenEncryption.In("Validate")
	}

	if err != nil {
		return false, errors.New(err)
	}

	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"session": session,
			"error":   errors.New(err)})

		return false, errors.New(err)
	}

	valid := true

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired.token", ui.A{
			"session": session,
			"id":      t.TokenID})

		err = errors.ErrExpiredToken.In("Validate")
		valid = false
	}

	// See if this is blacklisted before we continue.
	if valid {
		blacklisted, err = IsBlacklisted(t)
		if blacklisted {
			err = errors.ErrInvalidTokenEncryption.In("Validate")
			valid = false

			ui.Log(ui.AuthLogger, "auth.blacklisted", ui.A{
				"session": session,
				"id":      t.TokenID.String()})
		}
	}

	if err != nil {
		err = errors.New(err)
	} else {
		ui.Log(ui.AuthLogger, "auth.valid.token", ui.A{
			"session": session,
			"id":      t.TokenID.String(),
			"user":    t.Name,
			"expires": util.FormatDuration(time.Until(t.Expires), true)})
	}

	return valid, err
}
