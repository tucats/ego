package profile

import (
	"sort"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// getKey implements the profile.get() function.
func getKey(symbols *symbols.SymbolTable, args data.List) (any, error) {
	key := data.String(args.Get(0))

	// If this key is on the restricted list, it cannot be retrieved from this
	// function. This is to prevent exposing sensitive information.
	if defs.RestrictedSettings[key] {
		err := errors.ErrNoPrivilegeForOperation.In("Get").Context(key)

		return data.NewList("", err), err
	}

	return data.NewList(settings.Get(key), nil), nil
}

// setKey implements the profile.set() function. The returned error is
// carried as the function's single (any) return value with a nil native
// Go error, so it is a normal catchable/assignable value to Ego callers
// (matching how io.File.Close() behaves) rather than an uncatchable abort.
func setKey(symbols *symbols.SymbolTable, args data.List) (any, error) {
	key := data.String(args.Get(0))
	isEgoSetting := strings.HasPrefix(key, defs.PrivilegedKeyPrefix)

	// If this key is on the restricted list, it cannot be set from this
	// function, even ephemerally. This is to prevent exposing or overwriting
	// sensitive information (tokens, passwords, etc.)
	if defs.RestrictedSettings[key] {
		return errors.ErrNoPrivilegeForOperation.In("Set").Context(key), nil
	}

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return errors.ErrReservedProfileSetting.In("Set").Context(key), nil
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
		return errors.ErrReservedProfileSetting.In("Set").Context(key), nil
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	//
	// Ego settings ("ego.*") can only be updated in the in-memory copy, not
	// in the persisted data. Set()/Delete() mark the active Configuration
	// dirty regardless of key prefix, and the CLI's top-level command
	// dispatcher (internal/cli/app/run.go) unconditionally calls
	// settings.Save() after every successful command -- so using Set()/
	// Delete() here would flush the "ephemeral" change to disk anyway the
	// next time any command completes, defeating the in-memory-only intent
	// below. SetDefault()/DeleteDefault() touch only the transient
	// explicit-values overlay (which Get() still checks first) and never
	// mark anything dirty, so they can never leak to disk this way (BUG-78).
	value := data.String(args.Get(1))

	if isEgoSetting {
		if value == "" {
			settings.DeleteDefault(key)
		} else {
			settings.SetDefault(key, value)
		}

		return nil, nil
	}

	var err error

	if value == "" {
		err = settings.Delete(key)
	} else {
		settings.Set(key, value)
	}

	// Store the value back to the file system, preserving whichever error
	// (the delete/set itself, or the save) occurred first.
	if err == nil {
		err = settings.Save()
	}

	return err, nil
}

// deleteKey implements the profile.delete() function. This just calls
// the set operation with an empty value, which results in a delete operation.
// The consolidates the permission checking, etc. in the Set routine only.
func deleteKey(symbols *symbols.SymbolTable, args data.List) (any, error) {
	key := data.String(args.Get(0))

	// If this key is on the restricted list, it cannot be deleted from this
	// function. This is to prevent exposing sensitive information.
	if defs.RestrictedSettings[key] {
		return errors.ErrNoPrivilegeForOperation.In("Delete").Context(key), nil
	}

	return setKey(symbols, data.NewList(key, ""))
}

// getKeys implements the profile.keys() function. This returns a list of
// all of the keys found in the profile that are not on the restricted list.
func getKeys(symbols *symbols.SymbolTable, args data.List) (any, error) {
	keys := settings.Keys()
	result := []any{}

	sort.Strings(keys)

	for _, key := range keys {
		// We do not report back restricted keys.
		if defs.RestrictedSettings[key] {
			continue
		}

		result = append(result, key)
	}

	return data.NewArrayFromInterfaces(data.StringType, result...), nil
}

// getConfig implements the profile.Config() function. This returns a map of all
// the configuration keys and their values. Restricted keys are not returned.
func getConfig(symbols *symbols.SymbolTable, args data.List) (any, error) {
	keys := settings.Keys()
	result := data.NewMap(data.StringType, data.StringType)

	for _, key := range keys {
		// We do not report back restricted keys.
		if defs.RestrictedSettings[key] {
			continue
		}

		value := settings.Get(key)

		if _, err := result.Set(key, value); err != nil {
			return nil, err
		}
	}

	return result, nil
}
