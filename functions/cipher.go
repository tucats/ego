package functions

import (
	"crypto/rand"
	"encoding/base64"
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

// Hash implements the cipher.hash() function. For an arbitrary string
// value, it computes a crypotraphic hash of the value, and returns it
// as a 32-character string containing the hexadecimal hash value. Hashes
// are irreversible.
func Hash(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return util.Hash(data.String(args[0])), nil
}

// Encrypt implements the cipher.Encrypt() function. This takes a string value and
// a string key, and encrypts the string using the key.
func Encrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	b, err := util.Encrypt(data.String(args[0]), data.String(args[1]))
	if err != nil {
		return b, err
	}

	return hex.EncodeToString([]byte(b)), nil
}

// Decrypt implements the cipher.Decrypt() function. It accepts an encrypted string
// and a key, and attempts to decode the string. If the string is not a valid encryption
// using the given key, an empty string is returned. It is an error if the string does
// not contain a valid hexadecimal character string.
func Decrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	b, err := hex.DecodeString(data.String(args[0]))
	if err != nil {
		return nil, errors.NewError(err)
	}

	return util.Decrypt(string(b), data.String(args[1]))
}

// Validate determines if a token is valid and returns true/false.
func Validate(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	reportErr := false
	if len(args) > 1 {
		reportErr = data.Bool(args[1])
	}

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args[0]))
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
		err = errors.ErrInvalidTokenEncryption.In("validate()")
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
			err = errors.ErrExpiredToken.In("validate()")
		} else {
			return false, nil
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return true, err
}

// Extract extracts the data from a token and returns it as a struct.
func Extract(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	// Take the token value, and decode the hex string.
	b, err := hex.DecodeString(data.String(args[0]))
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
		return nil, errors.ErrInvalidTokenEncryption.In("extract()")
	}

	var t = authToken{}

	err = json.Unmarshal([]byte(j), &t)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		err = errors.ErrExpiredToken.In("validate()")
	}

	r := map[string]interface{}{}
	r["expires"] = t.Expires.String()
	r["name"] = t.Name
	r["data"] = t.Data
	r["session"] = t.AuthID.String()
	r["id"] = t.TokenID.String()

	if err != nil {
		err = errors.NewError(err)
	}

	return data.NewStructFromMap(r), err
}

// CreateToken creates a new token with a username and a data payload.
func CreateToken(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	t := authToken{
		Name:    data.String(args[0]),
		TokenID: uuid.New(),
	}

	if len(args) == 2 {
		t.Data = data.String(args[1])
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

// Random implements the cipher.Random() function which generates a random token
// string value using the cryptographic random number generator.
func CipherRandom(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	n := 32
	if len(args) > 0 {
		n = data.Int(args[0])
	}

	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return nil, errors.NewError(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

// EncodeBase64 encodes a string as a BASE64 string using standard encoding rules.
func EncodeBase64(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	text := data.String(args[0])

	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

// DecodeBase64 encodes a string as a BASE64 string using standard encoding rules.
func DecodeBase64(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	text := data.String(args[0])

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return string(b), nil
}
