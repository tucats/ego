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
func getKey(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	key := data.String(args[0])

	return settings.Get(key), nil
}

// setKey implements the profile.set() function.
func setKey(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	key := data.String(args[0])
	isEgoSetting := strings.HasPrefix(key, "ego.")

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
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
		return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := data.String(args[1])
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
func deleteKey(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return setKey(symbols, []interface{}{args[0], ""})
}

// getKeys implements the profile.keys() function.
func getKeys(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	keys := settings.Keys()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		result[i] = key
	}

	return data.NewArrayFromArray(data.StringType, result), nil
}
