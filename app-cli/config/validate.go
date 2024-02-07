package config

import (
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// Determine if a key is allowed to be updated by the CLI. This rule
// applies to keys with the privileged key prefix ("ego.").
func ValidateKey(key string) error {
	if strings.HasPrefix(key, defs.PrivilegedKeyPrefix) {
		allowed, found := defs.ValidSettings[key]
		if !found {
			return errors.ErrInvalidConfigName
		}

		if !allowed {
			return errors.ErrNoPrivilegeForOperation
		}
	}

	return nil
}
