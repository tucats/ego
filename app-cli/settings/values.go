package settings

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// explicitValues contains overridden default values.
var explicitValues = Configuration{
	Description: "overridden defaults",
	Items:       map[string]string{},
}

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
	c.Dirty = true

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
	wasFound := false

	if _, wasFound = explicitValues.Items[key]; wasFound {
		delete(explicitValues.Items, key)
	}

	// That takes care of the default value if it existed. Now
	// lets make sure it's delete from the configuration proper.
	c := getCurrentConfiguration()

	// If it wasn't in an default list, and isn't in the
	// configuration list, then it's not found and we complain.
	if !wasFound {
		if _, found := c.Items[key]; !found {
			return errors.ErrInvalidConfigName.Context(key)
		}
	}

	if _, isInConfig := c.Items[key]; isInConfig {
		// Its in the configuration, so update the datestamp, etc.
		c.Modified = time.Now().Format(time.RFC1123Z)
		c.Dirty = true

		delete(c.Items, key)
	}

	ui.Log(ui.AppLogger, "Deleting profile key \"%s\"", key)

	return nil
}

// Keys returns the list of keys in the profile as an array
// of strings.
func Keys() []string {
	keys := map[string]bool{}

	// If there are explicit (default override) keys, get that list
	// first.
	for key := range explicitValues.Items {
		keys[key] = true
	}

	// If there is a defined current configuration, get the keys from
	// that configuration as well.
	c := getCurrentConfiguration()
	if c != nil {
		for key := range c.Items {
			keys[key] = true
		}
	}

	// Convert the key list into an array of strings.
	result := []string{}

	for key := range keys {
		result = append(result, key)
	}

	// Sort the list of keys
	sort.Strings(result)

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
