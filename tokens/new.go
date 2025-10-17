package tokens

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// newToken creates a new token with a username and a data payload.
func New(name, data, interval string, instanceUUID string, session int) (string, error) {
	var (
		err error
	)

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := Token{
		Name:    name,
		Data:    data,
		Created: time.Now(),
		TokenID: uuid.New(),
	}

	t.AuthID, err = uuid.Parse(instanceUUID)
	if err != nil {
		return "", errors.New(err)
	}

	// Fetch the interval, or use 15 minutes as the default.
	if interval == "" {
		interval = "15m"
	}

	// Convert the interval string to a time.Duration value
	duration, err := util.ParseDuration(interval)
	if err != nil {
		return "", errors.New(err)
	}

	// Calculate the expiration time for the token based on the interval duration.
	t.Expires = time.Now().Add(duration)

	// Log that we just created a token.
	ui.Log(ui.AuthLogger, "auth.new.token", ui.A{
		"session": session,
		"id":      t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	// Make the token into a json string
	b, err := json.Marshal(t)
	if err != nil {
		return "", errors.New(err)
	}

	// Encrypt the string value
	encryptedString, err := util.Encrypt(string(b), getTokenKey())
	if err != nil {
		return string(b), errors.New(err)
	}

	return hex.EncodeToString([]byte(encryptedString)), nil
}
