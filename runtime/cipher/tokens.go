package cipher

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// authToken is the Go native expression of a token value, which contains
// the identity of the creator, an arbitrary data payload, an expiration
// time after which the token is no longer valid, a unique ID for this
// token, and the unique ID of the Ego session that created the token.
type authToken struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Expires time.Time
	AuthID  uuid.UUID
}

// validate determines if a token is valid and returns true/false.
func Validate(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err       error
		reportErr bool
		t         = authToken{}
	)

	if args.Len() > 1 {
		reportErr, err = data.Bool(args.Get(1))
		if err != nil {
			return nil, errors.New(err).In("cipher.Validate")
		}
	}

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"error": err})

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
			"error": err})

		err = errors.ErrInvalidTokenEncryption.In("Validate")
	}

	if err != nil {
		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"error": err})

		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired.token", ui.A{
			"id": t.TokenID})

		if reportErr {
			err = errors.ErrExpiredToken.In("Validate")
		} else {
			return false, nil
		}
	}

	if err != nil {
		err = errors.New(err)
	} else {
		ui.Log(ui.AuthLogger, "auth.valid.token", ui.A{
			"id":      t.TokenID.String(),
			"user":    t.Name,
			"expires": util.FormatDuration(time.Until(t.Expires), true)})
	}

	return true, err
}

// extract extracts the data from a token and returns it as a struct.
func Extract(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err error
		t   = authToken{}
	)

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"error": err})

		return nil, errors.New(err)
	}

	// Decrypt the token into a json string. We use the token key stored in
	// the preferences data. If there isn't one, generate a new random key.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.decrypt", ui.A{
			"error": err})

		return nil, errors.New(err)
	}

	if len(j) == 0 {
		return nil, errors.ErrInvalidTokenEncryption.In("Extract")
	}

	if err = json.Unmarshal([]byte(j), &t); err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.json", ui.A{
			"error": err})

		return nil, errors.New(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired", ui.A{
			"id": t.TokenID})

		err = errors.ErrExpiredToken.In("Extract")
	}

	ui.Log(ui.AuthLogger, "auth.valid.token", ui.A{
		"id":      t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	if err != nil {
		return nil, errors.New(err)
	}

	return data.NewStructOfTypeFromMap(CipherAuthType, map[string]interface{}{
		"Expires": t.Expires.Format(time.RFC822Z),
		"Name":    t.Name,
		"Data":    t.Data,
		"AuthID":  t.AuthID.String(),
		"TokenID": t.TokenID.String(),
	}), err
}

// newToken creates a new token with a username and a data payload.
func NewToken(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err      error
		interval string
	)

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := authToken{
		Name:    data.String(args.Get(0)),
		TokenID: uuid.New(),
	}

	if args.Len() >= 2 {
		t.Data = data.String(args.Get(1))
	}

	if args.Len() >= 3 {
		interval = data.String(args.Get(2))
	}

	// Get the session ID of the current Ego program and add it to
	// the token. A token can only be validated on the same system
	// that created it.
	if session, ok := s.Get(defs.InstanceUUIDVariable); ok {
		t.AuthID, err = uuid.Parse(data.String(session))
		if err != nil {
			return nil, errors.New(err)
		}
	}

	// Fetch the default interval, or use 15 minutes as the default. If the
	// duration is expresssed in days, convert it to hours. Calculate a time
	// value for when this token expires
	defaultDuration := settings.Get(defs.ServerTokenExpirationSetting)
	if interval == "" {
		interval = defaultDuration
		if interval == "" {
			interval = "15m"
		}
	} else {
		if days, err := egostrings.Atoi(strings.TrimSuffix(interval, "d")); err == nil {
			interval = strconv.Itoa(days*24) + "h"
		}
	}

	// Convert the interval string to a time.Duration value
	duration, err := util.ParseDuration(interval)
	if err != nil {
		return nil, errors.New(err)
	}

	// Make sure the resulting interval is not greater than the maximum
	// allowed value from the configuration. If there is a default expiration
	// setting, ensure the interval is not greater than that.
	if defaultDuration != "" {
		maxDuration, err := util.ParseDuration(defaultDuration)
		if err == nil {
			if duration > maxDuration {
				duration = maxDuration
			}
		}
	}

	// Calculate the expiration time for the token based on the interval duration.
	t.Expires = time.Now().Add(duration)

	// Log that we just created a token.
	ui.Log(ui.AuthLogger, "auth.new.token", ui.A{
		"id":      t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	// Make the token into a json string
	b, err := json.Marshal(t)
	if err != nil {
		return nil, errors.New(err)
	}

	// Encrypt the string value
	encryptedString, err := util.Encrypt(string(b), getTokenKey())
	if err != nil {
		return b, errors.New(err)
	}

	return hex.EncodeToString([]byte(encryptedString)), nil
}

// getTokenKey fetches the key used to encrypt tokens. If it
// was not already set up, a new random one is generated.
func getTokenKey() string {
	key := settings.Get(defs.ServerTokenKeySetting)
	if key == "" {
		key = uuid.New().String() + "-" + uuid.New().String()

		settings.Set(defs.ServerTokenKeySetting, key)
		_ = settings.Save()
	}

	return key
}
