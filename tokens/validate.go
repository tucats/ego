package tokens

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// Validate determines whether a token string represents a currently-valid
// credential and returns (true, nil) if it does. Use this function when you
// only need a yes/no answer; call Unwrap() when you also need the token's
// contents (username, data payload, etc.).
//
// Parameters:
//
//	tokenString — the hex-encoded, encrypted token string as produced by New().
//	session     — a log-correlation integer written to the auth log; not stored
//	              in or used by the token itself.
//
// Returns:
//
//	(true,  nil)   — the token is valid, unexpired, and not blacklisted.
//	(false, error) — the token string is not valid hex, decryption failed, the
//	                 token is expired, the token is blacklisted, or the JSON
//	                 could not be parsed.
//
// How it works, step by step:
//
//  1. Hex-decode the token string.
//  2. Decrypt the bytes using the server's symmetric key to recover JSON.
//  3. Unmarshal the JSON into a Token struct.
//  4. Check the Expires field; mark the token invalid if it has passed.
//  5. Check the blacklist; mark the token invalid if its ID has been revoked.
//  6. Return the validity flag and any error.
//
// Difference from Unwrap: Validate does not return the Token struct, making it
// a lighter-weight check for callers that only need to gate access. Both
// functions perform the same expiration and blacklist checks.
func Validate(tokenString string, session int) (bool, error) {
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
			"error":   errors.New(err)})

		return false, errors.New(err)
	}

	// Step 2: decrypt the raw bytes to recover the JSON payload.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil || len(j) == 0 {
		// Either decryption itself failed, or it succeeded but produced no
		// usable output. Both conditions are treated as an invalid token.
		ui.Log(ui.AuthLogger, "auth.invalid.decryption", ui.A{
			"session": session,
			"error":   errors.New(err)})

		err = errors.ErrInvalidTokenEncryption.In("Validate")
	}

	if err != nil {
		return false, errors.New(err)
	}

	// Step 3: parse the JSON back into a Token struct.
	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"session": session,
			"error":   errors.New(err)})

		return false, errors.New(err)
	}

	valid := true

	// Step 4: check expiration. time.Since returns a positive duration when
	// t.Expires is in the past (i.e., the token has expired).
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired.token", ui.A{
			"session": session,
			"id":      t.TokenID})

		err = errors.ErrExpiredToken.In("Validate")
		valid = false
	}

	// Step 5: only consult the blacklist when the token is otherwise valid, to
	// avoid an unnecessary database round-trip for already-rejected tokens.
	if valid {
		blacklisted, err = IsBlacklisted(t)
		if blacklisted {
			err = errors.ErrBlacklisted.Clone().Context(t.TokenID).In("Validate")
			valid = false

			ui.Log(ui.AuthLogger, "auth.blacklisted", ui.A{
				"session": session,
				"id":      t.TokenID.String()})
		}
	}

	// Log a success message only when everything checked out.
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
