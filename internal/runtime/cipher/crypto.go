package cipher

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
)

// hash implements the cipher.hash() function. For an arbitrary string
// value, it computes a cryptographic hash of the value, and returns it
// as a 64-character string containing the hexadecimal SHA-256 hash value.
// Hashes are irreversible.
func hash(s *symbols.SymbolTable, args data.List) (any, error) {
	return util.Hash(data.String(args.Get(0))), nil
}

// encrypt implements the cipher.encrypt() function. This takes a string value and
// a string key, and encrypts the string using the key. Returns the encrypted text
// as a hexadecimal string, and an error if the underlying encryption operation
// failed (for example, if the system's cryptographic random source is unavailable).
func encrypt(s *symbols.SymbolTable, args data.List) (any, error) {
	b, err := util.Encrypt(data.String(args.Get(0)), data.String(args.Get(1)))
	if err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	return data.NewList(hex.EncodeToString([]byte(b)), nil), nil
}

// decrypt implements the cipher.decrypt() function. It accepts an encrypted string
// and a key, and attempts to decode the string. If the string is not a valid encryption
// using the given key, an empty string is returned. It is an error if the string does
// not contain a valid hexadecimal character string.
func decrypt(s *symbols.SymbolTable, args data.List) (any, error) {
	b, err := hex.DecodeString(data.String(args.Get(0)))
	if err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	result, err := util.Decrypt(string(b), data.String(args.Get(1)))

	return data.NewList(result, err), err
}

// random implements the cipher.random() function which generates a random token
// string value using the cryptographic random number generator.
func random(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	n := 32
	if args.Len() > 0 {
		n, err = data.Int(args.Get(0))
		if err != nil {
			err = errors.New(err).In("cipher.Random")

			return data.NewList(nil, err), err
		}
	}

	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	return data.NewList(base64.URLEncoding.EncodeToString(b), nil), nil
}
