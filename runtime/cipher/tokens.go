package cipher

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
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
func validate(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	reportErr := false
	if args.Len() > 1 {
		reportErr = data.Bool(args.Get(1))
	}

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		if reportErr {
			return false, errors.NewError(err)
		}

		return false, nil
	}

	// Decrypt the token into a json string
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err == nil && len(j) == 0 {
		err = errors.ErrInvalidTokenEncryption.In("Validate")
	}

	if err != nil {
		if reportErr {
			return false, errors.NewError(err)
		}

		return false, nil
	}

	var t = authToken{}

	err = json.Unmarshal([]byte(j), &t)
	if err != nil {
		if reportErr {
			return false, errors.NewError(err)
		}

		return false, nil
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		if reportErr {
			err = errors.ErrExpiredToken.In("Validate")
		} else {
			return false, nil
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return true, err
}

// extract extracts the data from a token and returns it as a struct.
func extract(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		return nil, errors.NewError(err)
	}

	// Decrypt the token into a json string. We use the token key stored in
	// the preferences data. If there isn't one, generate a new random key.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if err != nil {
		return nil, errors.NewError(err)
	}

	if len(j) == 0 {
		return nil, errors.ErrInvalidTokenEncryption.In("Extract")
	}

	var t = authToken{}

	err = json.Unmarshal([]byte(j), &t)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		err = errors.ErrExpiredToken.In("Extract")
	}

	r := map[string]interface{}{}
	r["Expires"] = t.Expires.String()
	r["Name"] = t.Name
	r["Data"] = t.Data
	r["AuthID"] = t.AuthID.String()
	r["TokenID"] = t.TokenID.String()
	r[data.TypeMDKey] = authType

	if err != nil {
		err = errors.NewError(err)
	}

	return data.NewStructFromMap(r), err
}

// newToken creates a newToken token with a username and a data payload.
func newToken(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := authToken{
		Name:    data.String(args.Get(0)),
		TokenID: uuid.New(),
	}

	if args.Len() == 2 {
		t.Data = data.String(args.Get(1))
	}

	// Get the session ID of the current Ego program and add it to
	// the token. A token can only be validated on the same system
	// that created it.
	if session, ok := s.Get(defs.InstanceUUIDVariable); ok {
		t.AuthID, err = uuid.Parse(data.String(session))
		if err != nil {
			return nil, errors.NewError(err)
		}
	}

	// Fetch the default interval, or use 15 minutes as the default.
	// Calculate a time value for when this token expires
	interval := settings.Get(defs.ServerTokenExpirationSetting)
	if interval == "" {
		interval = "15m"
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, errors.NewError(err)
	}

	t.Expires = time.Now().Add(duration)

	// Make the token into a json string
	b, err := json.Marshal(t)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// Encrypt the string value
	encryptedString, err := util.Encrypt(string(b), getTokenKey())
	if err != nil {
		return b, errors.NewError(err)
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
