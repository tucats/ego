package dsns

import (
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// Externally available function to lookup a DSN. Must provide the
// username and dsn name strings.
func Lookup(session int, user, name string) (*defs.DSN, error) {
	if DSNService == nil {
		return nil, errors.ErrNoSuchDSN.Context("DSN service not initialized")
	}

	dsn, err := DSNService.ReadDSN(session, user, name, true)
	if err != nil {
		return nil, err
	}

	return &dsn, nil
}
