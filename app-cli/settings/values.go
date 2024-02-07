package settings

import (
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// explicitValues contains overridden default values.
var explicitValues = Configuration{Description: "overridden defaults", Items: map[string]string{}}

// ProfileDirty is set to true when a key value is written or deleted, which
// tells us to rewrite the profile. If false, then no update is required.
var ProfileDirty = false

// Configurations is a map keyed by the configuration name for each
// configuration in the config file.
var Configurations map[string]*Configuration

// Set stores a profile entry in the current configuration. It also updates
// the value in the transient default configuration as well.
func Set(key string, value string) {
	explicitValues.Items[key] = value
	c := getCurrentConfiguration()
	c.Items[key] = value
	c.Modified = time.Now().Format(time.RFC1123Z)
	ProfileDirty = true

	ui.Log(ui.AppLogger, "Setting profile key \"%s\" = \"%s\"", key, value)
}

// SetDefault puts a profile entry in the current Configuration structure. It is
// different than Set() in that it doesn't mark the value as dirty, so no need
// to update on account of this setting.
func SetDefault(key string, value string) {
	explicitValues.Items[key] = value

	ui.Log(ui.AppLogger, "Setting default key \"%s\" = \"%s\"", key, value)
}

// Get gets a profile entry in the current configuration structure.
// If the key does not exist, an empty string is returned.
func Get(key string) string {
	// First, search the default values that have been explicitly set.
	v, found := explicitValues.Items[key]
	if !found {
		// Not in the defaults area, so read from the persistent configuration.
		c := getCurrentConfiguration()
		v = c.Items[key]
	}

	return v
}

// GetBool returns the boolean value of a configuration item. If the item
// string is "Y", "YES", "1", or "true" then the value returns true.
func GetBool(key string) bool {
	s := strings.ToLower(Get(key))
	if s == "y" || s == "yes" || s == defs.True || s == "t" || s == "1" {
		return true
	}

	return false
}

// GetInt returns the integer value of a configuration item by name.
// The string is converted to an int and returned.
func GetInt(key string) int {
	s := strings.ToLower(Get(key))
	value, _ := strconv.Atoi(s)

	return value
}

// Get a key value, and compare it to a list of provided values. If it
// matches one of the items in the list, then the position in the list
// (one-based) is returned. If the value is not in the list at all,
// a result of 0 is returned.
func GetUsingList(key string, values ...string) int {
	v := strings.TrimSpace(strings.ToLower(Get(key)))

	for position, value := range values {
		if v == value {
			return position + 1
		}
	}

	return 0
}

// Delete removes a key from the configuration.
func Delete(key string) error {
	c := getCurrentConfiguration()

	if _, found := c.Items[key]; !found {
		return errors.ErrInvalidConfigName.Context(key)
	}

	delete(c.Items, key)
	delete(explicitValues.Items, key)

	c.Modified = time.Now().Format(time.RFC1123Z)

	ProfileDirty = true

	ui.Log(ui.AppLogger, "Deleting profile key \"%s\"", key)

	return nil
}

// Keys returns the list of keys in the profile as an array
// of strings.
func Keys() []string {
	result := []string{}

	c := getCurrentConfiguration()
	for key := range c.Items {
		result = append(result, key)
	}

	return result
}

// Exists test to see if a key value exists or not.
func Exists(key string) bool {
	_, exists := explicitValues.Items[key]
	if !exists {
		c := getCurrentConfiguration()
		_, exists = c.Items[key]
	}

	return exists
}
