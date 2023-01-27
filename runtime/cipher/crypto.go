package cipher

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Hash implements the cipher.hash() function. For an arbitrary string
// value, it computes a crypotraphic hash of the value, and returns it
// as a 32-character string containing the hexadecimal hash value. Hashes
// are irreversible.
func Hash(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	return util.Hash(data.String(args[0])), nil
}

// Encrypt implements the cipher.Encrypt() function. This takes a string value and
// a string key, and encrypts the string using the key.
func Encrypt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, errors.ErrArgumentCount
	}

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
		return data.List(nil, err), errors.NewError(err)
	}

	result, err := util.Decrypt(string(b), data.String(args[1]))

	return data.List(result, err), err
}

// Random implements the cipher.Random() function which generates a random token
// string value using the cryptographic random number generator.
func Random(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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
