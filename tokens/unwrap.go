package tokens

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// Unwrap decodes, decrypts, and validates a token string, then returns the
// embedded Token struct so the caller can read the owner's name, data payload,
// and other fields. Use this function when you need the token's contents, not
// just a yes/no validity answer; for a simple validity check, call Validate()
// instead.
//
// Parameters:
//
//	tokenString — the hex-encoded, encrypted token string as produced by New().
//	session     — a log-correlation integer written to the auth log; not stored
//	              in or used by the token itself.
//
// Returns:
//
//	A pointer to a Token struct and a nil error when the token is valid.
//	A nil pointer and a non-nil error when anything goes wrong. Specific errors:
//
//	  - hex.DecodeString failure → the string is not valid hexadecimal.
//	  - util.Decrypt failure → the bytes could not be decrypted (wrong key,
//	    corrupted data, or the token was issued by a different server).
//	  - ErrInvalidTokenEncryption → decryption succeeded but produced an empty
//	    result, meaning the token content is unusable.
//	  - json.Unmarshal failure → the decrypted bytes are not valid JSON.
//	  - ErrExpiredToken → the token's Expires timestamp is in the past.
//	  - ErrBlacklisted → the token ID is in the revocation list.
//
// How it works, step by step:
//
//  1. Hex-decode the token string back to raw bytes.
//  2. Decrypt those bytes using the server's symmetric key to recover JSON.
//  3. Unmarshal the JSON into a Token struct.
//  4. Check the Expires field; reject the token if it has passed.
//  5. Check the blacklist; reject the token if its ID has been revoked.
//  6. Return a pointer to the struct on success.
func Unwrap(tokenString string, session int) (*Token, error) {
	var (
		err         error
		blacklisted bool
		t           = Token{}
	)

	// Step 1: reverse the hex encoding applied by New().
	b, err := hex.DecodeString(tokenString)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	// Step 2: decrypt using the server's token key.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.decrypt", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err).In("Extract.decrypt")
	}

	// A successful decryption that yields an empty string means the token
	// bytes were valid but contained nothing usable.
	if len(j) == 0 {
		return nil, errors.ErrInvalidTokenEncryption.In("Extract.no-data")
	}

	// Step 3: parse the JSON back into a Token struct.
	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	// Step 4: check expiration. time.Since returns a positive duration when
	// t.Expires is in the past (i.e., the token is expired).
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired", ui.A{
			"session": session,
			"id":      t.TokenID})

		err = errors.ErrExpiredToken.In("Extract.expired")
	}

	// Step 5: only check the blacklist if the token has not already been
	// rejected for expiration, to avoid unnecessary database round-trips.
	if err == nil {
		blacklisted, err = IsBlacklisted(t)
		if blacklisted {
			err = errors.ErrBlacklisted.Clone().Context(t.TokenID).In("Extract.blacklisted")

			ui.Log(ui.AuthLogger, "auth.blacklisted", ui.A{
				"session": session,
				"id":      t.TokenID.String()})
		}
	}

	// If either check failed, return the error. We do not return the partial
	// Token struct because the caller should not act on data from an invalid token.
	if err != nil {
		return nil, errors.New(err)
	}

	ui.Log(ui.AuthLogger, "auth.decrypted", ui.A{
		"session": session,
		"tokenID": t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	return &t, err
}
