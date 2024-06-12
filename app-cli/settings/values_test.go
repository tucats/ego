package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tucats/ego/errors"
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
}

func TestGetInt_ValidInteger(t *testing.T) {
	SetDefault("test_key", "123")
	result := GetInt("test_key")
	if result != 123 {
		t.Errorf("Expected 123, got %d", result)
	}
}

func TestGetInt_ValidNegativeInteger(t *testing.T) {
	SetDefault("test_key", "-456")
	result := GetInt("test_key")
	if result != -456 {
		t.Errorf("Expected -456, got %d", result)
	}
}

func TestGetInt_InvalidInteger(t *testing.T) {
	SetDefault("test_key", "abc")
	result := GetInt("test_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
}

func TestGetInt_EmptyKey(t *testing.T) {
	SetDefault("test_key", "")
	result := GetInt("test_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
}

func TestGetInt_NonExistentKey(t *testing.T) {
	SetDefault("test_key", "123")
	result := GetInt("non_existent_key")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
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
	Delete("test_key")
	result = GetUsingList("test_key", "value1", "value2", "value3")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
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
}
