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

// hash implements the cipher.hash() function. For an arbitrary string
// value, it computes a crypotraphic hash of the value, and returns it
// as a 32-character string containing the hexadecimal hash value. Hashes
// are irreversible.
func hash(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	return util.Hash(data.String(args.Get(0))), nil
}

// encrypt implements the cipher.encrypt() function. This takes a string value and
// a string key, and encrypts the string using the key.
func encrypt(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() != 2 {
		return nil, errors.ErrArgumentCount
	}

	b, err := util.Encrypt(data.String(args.Get(0)), data.String(args.Get(1)))
	if err != nil {
		return b, err
	}

	return hex.EncodeToString([]byte(b)), nil
}

// decrypt implements the cipher.decrypt() function. It accepts an encrypted string
// and a key, and attempts to decode the string. If the string is not a valid encryption
// using the given key, an empty string is returned. It is an error if the string does
// not contain a valid hexadecimal character string.
func decrypt(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		return data.NewList(nil, err), errors.NewError(err)
	}

	result, err := util.Decrypt(string(b), data.String(args.Get(1)))

	return data.NewList(result, err), err
}

// random implements the cipher.random() function which generates a random token
// string value using the cryptographic random number generator.
func random(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	n := 32
	if args.Len() > 0 {
		n = data.Int(args.Get(0))
	}

	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return nil, errors.NewError(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
