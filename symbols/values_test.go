package symbols

import (
	"testing"

	"github.com/google/uuid"
)

func TestSymbolTable_SetValue(t *testing.T) {
	symbols := &SymbolTable{
		Name:    "test",
		parent:  nil,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  false,
		values:  []*[]any{},
	}

	// Set the allocation size to 4 so we know we use multiple bins
	SymbolAllocationSize = 4

	// Test setting a value at index 0
	symbols.setValue(0, "value1")

	if (*symbols.values[0])[0] != "value1" {
		t.Errorf("SetValue(0) failed, expected %v, got %v", "value1", (*symbols.values[0])[0])
	}

	// Test setting a value at index 1
	symbols.setValue(1, "value2")

	if (*symbols.values[0])[1] != "value2" {
		t.Errorf("SetValue(1) failed, expected %v, got %v", "value2", (*symbols.values[0])[1])
	}

	// Test setting a value at index 2
	symbols.setValue(2, "value3")

	if (*symbols.values[0])[2] != "value3" {
		t.Errorf("SetValue(2) failed, expected %v, got %v", "value3", (*symbols.values[0])[2])
	}

	// Test setting a value at index 3
	symbols.setValue(3, "value4")

	if (*symbols.values[0])[3] != "value4" {
		t.Errorf("SetValue(3) failed, expected %v, got %v", "value4", (*symbols.values[1])[0])
	}

	// Test setting a value at index 4. This test will split the value to a second bin.
	symbols.setValue(4, "value5")

	if (*symbols.values[1])[0] != "value5" {
		t.Errorf("SetValue(4) failed, expected %v, got %v", "value5", (*symbols.values[1])[1])
	}

	// Test setting a value at index 5
	symbols.setValue(5, "value6")

	if (*symbols.values[1])[1] != "value6" {
		t.Errorf("SetValue(5) failed, expected %v, got %v", "value6", (*symbols.values[1])[2])
	}
}
