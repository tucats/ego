package functions

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

const TokenExpirationSetting = "ego.token.expiration"
const TokenKeySetting = "ego.token.key"

type Token struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Expires time.Time
	AuthID  uuid.UUID
}

// Hash implements the cipher.hash() function.
func Hash(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return util.Hash(util.GetString(args[0])), nil
}

// Encrypt implements the cipher.hash() function.
func Encrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	b, err := util.Encrypt(util.GetString(args[0]), util.GetString(args[1]))
	if !errors.Nil(err) {
		return b, err
	}

	return hex.EncodeToString([]byte(b)), nil
}

// Decrypt implements the cipher.hash() function.
func Decrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	b, err := hex.DecodeString(util.GetString(args[0]))
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	return util.Decrypt(string(b), util.GetString(args[1]))
}

// Validate determines if a token is valid and returns true/false.
func Validate(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	reportErr := false
	if len(args) > 1 {
		reportErr = util.GetBool(args[1])
	}

	// Take the token value, and de-hexify it.
	b, err := hex.DecodeString(util.GetString(args[0]))
	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		} else {
			return false, nil
		}
	}

	// Decrypt the token into a json string
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if errors.Nil(err) && len(j) == 0 {
		err = errors.New(errors.InvalidTokenEncryption).In("validate()")
	}

	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		} else {
			return false, nil
		}
	}

	var t = Token{}

	err = json.Unmarshal([]byte(j), &t)
	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		} else {
			return false, nil
		}
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		if reportErr {
			err = errors.New(errors.ExpiredTokenError).In("validate()")
		} else {
			return false, nil
		}
	}

	return true, errors.New(err)
}

// Extract extracts the data from a token and returns it as a struct.
func Extract(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	// Take the token value, and de-hexify it.
	b, err := hex.DecodeString(util.GetString(args[0]))
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Decrypt the token into a json string. We use the token key stored in
	// the preferences data. If there isn't one, generate a new random key.
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	if len(j) == 0 {
		return nil, errors.New(errors.InvalidTokenEncryption).In("extract()")
	}

	var t = Token{}

	err = json.Unmarshal([]byte(j), &t)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		err = errors.New(errors.ExpiredTokenError).In("validate()")
	}

	r := map[string]interface{}{}
	r["expires"] = t.Expires.String()
	r["name"] = t.Name
	r["data"] = t.Data
	r["session"] = t.AuthID.String()
	r["id"] = t.TokenID.String()

	return r, errors.New(err)
}

// CreateToken creates a new token with a username and a data payload.
func CreateToken(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := Token{
		Name:    util.GetString(args[0]),
		TokenID: uuid.New(),
	}

	if len(args) == 2 {
		t.Data = util.GetString(args[1])
	}

	// Get the session ID of the current Ego program and add it to
	// the token. A token can only be validated on the same system
	// that created it.
	if session, ok := s.Get("_session"); ok {
		t.AuthID, err = uuid.Parse(util.GetString(session))
		if !errors.Nil(err) {
			return nil, errors.New(err)
		}
	}

	// Fetch the default interval, or use 15 minutes as the default.
	// Calculate a time value for when this token expires
	interval := persistence.Get(TokenExpirationSetting)
	if interval == "" {
		interval = "15m"
	}

	duration, err := time.ParseDuration(interval)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	t.Expires = time.Now().Add(duration)

	// Make the token into a json string
	b, err := json.Marshal(t)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Encrypt the string value
	encryptedString, err := util.Encrypt(string(b), getTokenKey())
	if !errors.Nil(err) {
		return b, errors.New(err)
	}

	return hex.EncodeToString([]byte(encryptedString)), nil
}

// getTokenKey fetches the key used to encrypt tokens. If it
// was not already set up, a new random one is generated.
func getTokenKey() string {
	key := persistence.Get(TokenKeySetting)
	if key == "" {
		key = uuid.New().String() + "-" + uuid.New().String()

		persistence.Set(TokenKeySetting, key)
		_ = persistence.Save()
	}

	return key
}
