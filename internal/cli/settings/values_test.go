package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tucats/ego/internal/errors"
)

func TestSet(t *testing.T) {
	// Set up the initial state
	Configurations = map[string]*Configuration{
		"default": {
			Description: "default configuration",
			Items:       map[string]string{},
			Modified:    time.Now().Format(time.RFC1123Z),
			Dirty:       false,
		},
	}

	// Store the pointer to the current configuration
	// from the list of possible configurations.
	CurrentConfiguration = Configurations["default"]

	// Test case 1: Set a new key-value pair
	Set("newKey", "newValue")
	assert.Equal(t, "newValue", Get("newKey"))
	assert.True(t, Exists("newKey"))

	// Test case 2: Override an existing key-value pair
	Set("newKey", "overriddenValue")
	assert.Equal(t, "overriddenValue", Get("newKey"))

	// Test case 3: Check that the modified timestamp is updated
	c := Configurations["default"]
	initialModified := c.Modified

	time.Sleep(1 * time.Second) // Ensure the modified timestamp changes
	Set("anotherKey", "anotherValue")

	oldT, _ := time.Parse(time.RFC1123Z, initialModified)

	modTime := c.Modified

	newT, _ := time.Parse(time.RFC1123Z, modTime)

	assert.True(t, newT.After(oldT))

	// Test case 4: Check that the configuration is marked as dirty
	assert.True(t, c.Dirty)
}

// TestSetDefault verifies the ephemeral overlay contract SetDefault must
// hold: Get() sees the new value immediately, but the persisted
// Configuration itself is never touched and never marked dirty. This is
// what makes SetDefault (unlike Set) safe for profile.Set() to use for
// "ego.*" keys without leaking to disk the next time any CLI command's
// unconditional settings.Save() runs (BUG-78).
func TestSetDefault(t *testing.T) {
	Configurations = map[string]*Configuration{
		"default": {
			Description: "default configuration",
			Items:       map[string]string{},
			Modified:    time.Now().Format(time.RFC1123Z),
			Dirty:       false,
		},
	}
	CurrentConfiguration = Configurations["default"]

	ClearDefaults()

	SetDefault("ephemeralKey", "ephemeralValue")

	assert.Equal(t, "ephemeralValue", Get("ephemeralKey"))
	assert.True(t, Exists("ephemeralKey"))

	c := getCurrentConfiguration()
	_, inPersistedConfig := c.Items["ephemeralKey"]
	assert.False(t, inPersistedConfig)
	assert.False(t, c.Dirty)

	ClearDefaults()
}

// TestDeleteDefault mirrors TestSetDefault for the removal side: it must
// remove the ephemeral override (so Get() falls back to whatever, if
// anything, is in the persisted configuration) without ever marking the
// persisted configuration dirty -- including when no override was set in
// the first place, which must be a harmless no-op rather than an error
// (BUG-78).
func TestDeleteDefault(t *testing.T) {
	Configurations = map[string]*Configuration{
		"default": {
			Description: "default configuration",
			Items:       map[string]string{"persistedKey": "persistedValue"},
			Modified:    time.Now().Format(time.RFC1123Z),
			Dirty:       false,
		},
	}
	CurrentConfiguration = Configurations["default"]
	
	ClearDefaults()

	// Case 1: deleting an ephemeral override falls back to the persisted value.
	SetDefault("persistedKey", "overriddenValue")
	assert.Equal(t, "overriddenValue", Get("persistedKey"))

	DeleteDefault("persistedKey")
	assert.Equal(t, "persistedValue", Get("persistedKey"))

	c := getCurrentConfiguration()
	assert.False(t, c.Dirty)
	assert.Equal(t, "persistedValue", c.Items["persistedKey"])

	// Case 2: deleting a key with no ephemeral override is a harmless no-op.
	DeleteDefault("neverSet")
	assert.False(t, c.Dirty)

	ClearDefaults()
}

func TestGetBool(t *testing.T) {
	// Test case 1: Boolean value "true"
	SetDefault("test_key1", "true")

	if !GetBool("test_key1") {
		t.Errorf("Expected GetBool(\"test_key1\") to return true, but got false")
	}

	// Test case 2: Boolean value "True"
	SetDefault("test_key2", "True")

	if !GetBool("test_key2") {
		t.Errorf("Expected GetBool(\"test_key2\") to return true, but got false")
	}

	// Test case 3: Boolean value "yes"
	SetDefault("test_key3", "yes")

	if !GetBool("test_key3") {
		t.Errorf("Expected GetBool(\"test_key3\") to return true, but got false")
	}

	// Test case 4: Boolean value "YES"
	SetDefault("test_key4", "YES")

	if !GetBool("test_key4") {
		t.Errorf("Expected GetBool(\"test_key4\") to return true, but got false")
	}

	// Test case 5: Boolean value "1"
	SetDefault("test_key5", "1")

	if !GetBool("test_key5") {
		t.Errorf("Expected GetBool(\"test_key5\") to return true, but got false")
	}

	// Test case 6: Boolean value "false"
	SetDefault("test_key6", "false")

	if GetBool("test_key6") {
		t.Errorf("Expected GetBool(\"test_key6\") to return false, but got true")
	}

	// Test case 7: Boolean value "False"
	SetDefault("test_key7", "False")

	if GetBool("test_key7") {
		t.Errorf("Expected GetBool(\"test_key7\") to return false, but got true")
	}

	// Test case 8: Boolean value "no"
	SetDefault("test_key8", "no")

	if GetBool("test_key8") {
		t.Errorf("Expected GetBool(\"test_key8\") to return false, but got true")
	}

	// Test case 9: Boolean value "NO"
	SetDefault("test_key9", "NO")

	if GetBool("test_key9") {
		t.Errorf("Expected GetBool(\"test_key9\") to return false, but got true")
	}

	// Test case 10: Boolean value "0"
	SetDefault("test_key10", "0")

	if GetBool("test_key10") {
		t.Errorf("Expected GetBool(\"test_key10\") to return false, but got true")
	}

	// Test case 11: Boolean value "unknown"
	SetDefault("test_key11", "unknown")

	if GetBool("test_key11") {
		t.Errorf("Expected GetBool(\"test_key11\") to return false, but got true")
	}

	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetInt_ValidInteger(t *testing.T) {
	SetDefault("test_key", "123")

	result := GetInt("test_key")
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}

	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetInt_ValidNegativeInteger(t *testing.T) {
	SetDefault("test_key", "-456")

	result := GetInt("test_key")
	if result != -456 {
		t.Errorf("Expected -456, got %d", result)
	}

	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetInt_InvalidInteger(t *testing.T) {
	SetDefault("test_key", "abc")

	result := GetInt("test_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}

	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetInt_EmptyKey(t *testing.T) {
	SetDefault("test_key", "")

	result := GetInt("test_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetInt_NonExistentKey(t *testing.T) {
	SetDefault("test_key", "123")

	result := GetInt("non_existent_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestGetUsingList(t *testing.T) {
	// Test case 1: Key exists and matches the first value in the list
	SetDefault("test_key", "value1")

	result := GetUsingList("test_key", "value1", "value2", "value3")
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	// Test case 2: Key exists and matches the second value in the list
	SetDefault("test_key", "value2")

	result = GetUsingList("test_key", "value1", "value2", "value3")
	if result != 2 {
		t.Errorf("Expected 2, got %d", result)
	}

	// Test case 3: Key exists and matches the third value in the list
	SetDefault("test_key", "value3")

	result = GetUsingList("test_key", "value1", "value2", "value3")
	if result != 3 {
		t.Errorf("Expected 3, got %d", result)
	}

	// Test case 4: Key exists but does not match any value in the list
	SetDefault("test_key", "value4")

	result = GetUsingList("test_key", "value1", "value2", "value3")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}

	// Test case 5: Key does not exist in the configuration
	_ = Delete("test_key")

	result = GetUsingList("test_key", "value1", "value2", "value3")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestDelete(t *testing.T) {
	// Set up the initial state
	Configurations = map[string]*Configuration{
		"default": {
			Description: "default configuration",
			Items:       map[string]string{},
			Modified:    time.Now().Format(time.RFC1123Z),
			Dirty:       false,
		},
	}

	// Store the pointer to the current configuration
	// from the list of possible configurations.
	CurrentConfiguration = Configurations["default"]

	// Test case 1: Delete an existing key
	SetDefault("key1", "value1")

	err := Delete("key1")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if Exists("key1") {
		t.Error("Expected key to be deleted, but it still exists")
	}

	// Test case 2: Delete a non-existent key
	err = Delete("key2")
	if err == nil {
		t.Error("Expected an error, but got nil")
	}

	expectedErr := errors.ErrInvalidConfigName.Context("key2")
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error: %v, but got: %v", expectedErr, err)
	}

	// Test case 3: Delete a key after modifying it
	SetDefault("key3", "value3")
	Set("key3", "modifiedValue")

	err = Delete("key3")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if Exists("key3") {
		t.Error("Expected key to be deleted, but it still exists")
	}

	// Test case 4: Check that the modified time and dirty flag are updated
	// when using a key in a live configuration.
	Set("key4", "value4")

	originalTime := CurrentConfiguration.Modified

	time.Sleep(1 * time.Second) // Ensure the modified time is different

	err = Delete("key4")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	config := getCurrentConfiguration()
	if config.Modified == originalTime {
		t.Error("Expected modified time to be updated, but it was not")
	}

	if !config.Dirty {
		t.Error("Expected dirty flag to be set, but it was not")
	}

	// Test case 5: Check that the log message is correct
	SetDefault("key5", "value5")

	err = Delete("key5")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// All done, clear out the default value map so multiple tests run
	// sequentially don't find unexpected values.
	ClearDefaults()
}

func TestKeys_EmptyConfiguration(t *testing.T) {
	keys := Keys()

	assert.Empty(t, keys, "Keys() should return an empty slice when the configuration is empty")
}

func TestKeys_SingleItemConfiguration(t *testing.T) {
	CurrentConfiguration = &Configuration{
		Items: map[string]string{
			"key1": "value1",
		},
	}

	keys := Keys()
	assert.ElementsMatch(t, []string{"key1"}, keys, "Keys() should return a slice containing the single key")
}

func TestKeys_MultipleItemsConfiguration(t *testing.T) {
	CurrentConfiguration = &Configuration{Items: map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}}

	keys := Keys()
	assert.ElementsMatch(t, []string{"key1", "key2", "key3"}, keys, "Keys() should return a slice containing all keys")
}
