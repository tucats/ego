package functions

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// AuthToken is the Go native expression of a token value, which contains
// the identity of the creator, an arbitrary data payload, an expiration
// time after which the token is no longer valid, a unique ID for this
// token, and the unique ID of the Ego session that created the token.
type AuthToken struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Expires time.Time
	AuthID  uuid.UUID
}

// Hash implements the cipher.hash() function. For an arbitrary string
// value, it computes a crypotraphic hash of the value, and returns it
// as a 32-character string containing the hexadecimal hash value. Hashes
// are irreversible.
func Hash(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return util.Hash(datatypes.GetString(args[0])), nil
}

// Encrypt implements the cipher.Encrypt() function. This takes a string value and
// a string key, and encrypts the string using the key.
func Encrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	b, err := util.Encrypt(datatypes.GetString(args[0]), datatypes.GetString(args[1]))
	if !errors.Nil(err) {
		return b, err
	}

	return hex.EncodeToString([]byte(b)), nil
}

// Decrypt implements the cipher.Decrypt() function. It accepts an encrypted string
// and a key, and attempts to decode the string. If the string is not a valid encryption
// using the given key, an empty string is returned. It is an error if the string does
// not contain a valid hexadecimal character string.
func Decrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	b, err := hex.DecodeString(datatypes.GetString(args[0]))
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	return util.Decrypt(string(b), datatypes.GetString(args[1]))
}

// Validate determines if a token is valid and returns true/false.
func Validate(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	reportErr := false
	if len(args) > 1 {
		reportErr = datatypes.GetBool(args[1])
	}

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(datatypes.GetString(args[0]))
	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	// Decrypt the token into a json string
	key := getTokenKey()

	j, err := util.Decrypt(string(b), key)
	if errors.Nil(err) && len(j) == 0 {
		err = errors.New(errors.ErrInvalidTokenEncryption).In("validate()")
	}

	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	var t = AuthToken{}

	err = json.Unmarshal([]byte(j), &t)
	if !errors.Nil(err) {
		if reportErr {
			return false, errors.New(err)
		}

		return false, nil
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		if reportErr {
			err = errors.New(errors.ErrExpiredToken).In("validate()")
		} else {
			return false, nil
		}
	}

	return true, errors.New(err)
}

// Extract extracts the data from a token and returns it as a struct.
func Extract(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(datatypes.GetString(args[0]))
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
		return nil, errors.New(errors.ErrInvalidTokenEncryption).In("extract()")
	}

	var t = AuthToken{}

	err = json.Unmarshal([]byte(j), &t)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		err = errors.New(errors.ErrExpiredToken).In("validate()")
	}

	r := map[string]interface{}{}
	r["expires"] = t.Expires.String()
	r["name"] = t.Name
	r["data"] = t.Data
	r["session"] = t.AuthID.String()
	r["id"] = t.TokenID.String()

	return datatypes.NewStructFromMap(r), errors.New(err)
}

// CreateToken creates a new token with a username and a data payload.
func CreateToken(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := AuthToken{
		Name:    datatypes.GetString(args[0]),
		TokenID: uuid.New(),
	}

	if len(args) == 2 {
		t.Data = datatypes.GetString(args[1])
	}

	// Get the session ID of the current Ego program and add it to
	// the token. A token can only be validated on the same system
	// that created it.
	if session, ok := s.Get("_server_instance"); ok {
		t.AuthID, err = uuid.Parse(datatypes.GetString(session))
		if !errors.Nil(err) {
			return nil, errors.New(err)
		}
	}

	// Fetch the default interval, or use 15 minutes as the default.
	// Calculate a time value for when this token expires
	interval := settings.Get(defs.ServerTokenExpirationSetting)
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
	key := settings.Get(defs.ServerTokenKeySetting)
	if key == "" {
		key = uuid.New().String() + "-" + uuid.New().String()

		settings.Set(defs.ServerTokenKeySetting, key)
		_ = settings.Save()
	}

	return key
}

// Random implements the cipher.Random() function which generates a random token
// string value using the cryptographic random number generator.
func CipherRandom(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	n := 32
	if len(args) > 0 {
		n = datatypes.GetInt(args[0])
	}

	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return nil, errors.New(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

// EncodeBase64 encodes a string as a BASE64 string using standard encoding rules.
func EncodeBase64(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	text := datatypes.GetString(args[0])

	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

// DecodeBase64 encodes a string as a BASE64 string using standard encoding rules.
func DecodeBase64(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	text := datatypes.GetString(args[0])

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.New(err)
	}

	return string(b), nil
}
