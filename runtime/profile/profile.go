package profile

import (
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// getKey implements the profile.get() function.
func getKey(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	key := data.String(args.Get(0))

	// If this key is on the restricted list, it cannot be retrieved from this
	// function. This is to prevent exposing sensitive information.
	if defs.RestrictedSettings[key] {
		return nil, errors.ErrNoPrivilegeForOperation.In("Get").Context(key)
	}

	return settings.Get(key), nil
}

// setKey implements the profile.set() function.
func setKey(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	key := data.String(args.Get(0))
	isEgoSetting := strings.HasPrefix(key, defs.PrivilegedKeyPrefix)

	// If this key is on the restricted list, it cannot be retrieved from this
	// function. This is to prevent exposing sensitive information.
	if defs.RestrictedSettings[key] {
		return nil, errors.ErrNoPrivilegeForOperation.In("Set").Context(key)
	}

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return nil, errors.ErrReservedProfileSetting.In("Set").Context(key)
		}
	}

	// Additionally, we don't allow anyone to change runtime, compiler, or server settings from Ego code
	mode := "interactive"
	if modeValue, found := symbols.Get(defs.ModeVariable); found {
		mode = data.String(modeValue)
	}

	if mode != "test" &&
		(strings.HasPrefix(key, "ego.runtime") ||
			strings.HasPrefix(key, "ego.server") ||
			strings.HasPrefix(key, "ego.compiler")) {
		return nil, errors.ErrReservedProfileSetting.In("Set").Context(key)
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := data.String(args.Get(1))
	if value == "" {
		err = settings.Delete(key)
	} else {
		settings.Set(key, value)
	}

	// Ego settings can only be updated in the in-memory copy, not in the persisted data.
	if isEgoSetting {
		return err, nil
	}

	// Otherwise, store the value back to the file system.
	return err, settings.Save()
}

// deleteKey implements the profile.delete() function. This just calls
// the set operation with an empty value, which results in a delete operatinon.
// The consolidates the persmission checking, etc. in the Set routine only.
func deleteKey(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	key := data.String(args.Get(0))

	// If this key is on the restricted list, it cannot be deleted from this
	// function. This is to prevent exposing sensitive information.
	if defs.RestrictedSettings[key] {
		return nil, errors.ErrNoPrivilegeForOperation.In("Delete").Context(key)
	}

	return setKey(symbols, data.NewList(key, ""))
}

// getKeys implements the profile.keys() function.
func getKeys(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	keys := settings.Keys()
	result := []interface{}{}

	for _, key := range keys {
		// We do not report back restricted keys.
		if defs.RestrictedSettings[key] {
			continue
		}

		result = append(result, key)
	}

	return data.NewArrayFromInterfaces(data.StringType, result...), nil
}
