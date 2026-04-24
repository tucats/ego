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

// New creates a new authentication token and returns it as an opaque,
// hex-encoded string that can be sent to the client (typically as a Bearer
// token in an HTTP Authorization header).
//
// Parameters:
//
//	name         — the identity of the token owner, usually a username.
//	data         — an arbitrary string payload embedded in the token; the
//	               caller can store role or permission information here.
//	interval     — how long the token should remain valid, expressed as a Go
//	               duration string such as "15m", "2h", or "24h". An empty
//	               string defaults to "15m" (fifteen minutes).
//	instanceUUID — the UUID string of the server instance issuing the token.
//	               Must be a valid UUID (e.g., "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx").
//	session      — an integer log-correlation ID written to the auth log; it
//	               is not stored in the token itself.
//
// Returns:
//
//	A non-empty hex string on success, or an empty string and a non-nil error
//	when something goes wrong. Possible errors:
//
//	  - instanceUUID cannot be parsed as a UUID.
//	  - interval cannot be parsed as a duration.
//	  - JSON marshaling of the token struct fails (extremely unlikely).
//	  - Encryption fails (indicates a problem with the encryption library).
//
// How it works, step by step:
//
//  1. Build a Token struct containing the caller-supplied fields plus a
//     freshly generated random TokenID and the current UTC time as Created.
//  2. Calculate Expires = now + interval.
//  3. Marshal the struct to JSON.
//  4. Encrypt the JSON using the server's symmetric token key.
//  5. Hex-encode the encrypted bytes so they are safe to transmit as text.
func New(name, data, interval string, instanceUUID string, session int) (string, error) {
	var (
		err error
	)

	// Build the token struct. uuid.New() generates a random, globally-unique
	// identifier for this specific token, used later for blacklisting.
	t := Token{
		Name:    name,
		Data:    data,
		Created: time.Now(),
		TokenID: uuid.New(),
	}

	// Parse the server-instance UUID supplied by the caller.
	t.AuthID, err = uuid.Parse(instanceUUID)
	if err != nil {
		return "", errors.New(err)
	}

	// Default to 15 minutes if no interval was provided.
	if interval == "" {
		interval = "15m"
	}

	// Convert the interval string to a time.Duration value.
	duration, err := util.ParseDuration(interval)
	if err != nil {
		return "", errors.New(err)
	}

	// Set the expiration time relative to now.
	t.Expires = time.Now().Add(duration)

	// Write an audit entry so administrators can trace token creation in logs.
	ui.Log(ui.AuthLogger, "auth.new.token", ui.A{
		"session": session,
		"id":      t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	// Serialize the token to JSON so it can be encrypted as a byte stream.
	b, err := json.Marshal(t)
	if err != nil {
		return "", errors.New(err)
	}

	// Encrypt the JSON string. The key comes from the environment, settings,
	// or a freshly generated value (see key.go).
	encryptedString, err := util.Encrypt(string(b), getTokenKey())
	if err != nil {
		return string(b), errors.New(err)
	}

	// Hex-encode the encrypted bytes so the result is printable ASCII and safe
	// to embed in HTTP headers.
	return hex.EncodeToString([]byte(encryptedString)), nil
}
