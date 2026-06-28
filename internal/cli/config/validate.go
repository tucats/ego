package config

import (
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// Determine if a key is allowed to be updated by the CLI. This rule
// applies to keys with the privileged key prefix ("ego.").
func ValidateKey(key string) error {
	if strings.HasPrefix(key, defs.PrivilegedKeyPrefix) {
		allowed, found := defs.ValidSettings[key]
		if !found {
			return errors.ErrInvalidConfigName.Context(key)
		}

		if !allowed {
			return errors.ErrNoPrivilegeForOperation.Context(key)
		}
	}

	return nil
}
