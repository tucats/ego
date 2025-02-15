package symbols

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewSymbolTable(t *testing.T) {
	name := "test"
	alwaysShared := true

	symbols := NewSymbolTable(name)

	if symbols.Name != name {
		t.Errorf("NewSymbolTable() = %v, want %v", symbols.Name, name)
	}

	if symbols.parent != &RootSymbolTable {
		t.Errorf("NewSymbolTable() = %v, want %v", symbols.parent, &RootSymbolTable)
	}

	if symbols.id == uuid.Nil {
		t.Errorf("NewSymbolTable() = %v, want non-nil UUID", symbols.id)
	}

	if symbols.shared != alwaysShared {
		t.Errorf("NewSymbolTable() = %v, want %v", symbols.shared, alwaysShared)
	}
}
func TestNewChildSymbolTable(t *testing.T) {
	name := "test"
	parent := &SymbolTable{
		Name:    "parent",
		parent:  nil,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  false,
	}
	alwaysShared := true

	symbols := NewChildSymbolTable(name, parent)

	if symbols.Name != name {
		t.Errorf("NewChildSymbolTable() = %v, want %v", symbols.Name, name)
	}

	if symbols.parent != parent {
		t.Errorf("NewChildSymbolTable() = %v, want %v", symbols.parent, parent)
	}

	if symbols.id == uuid.Nil {
		t.Errorf("NewChildSymbolTable() = %v, want non-nil UUID", symbols.id)
	}

	if symbols.shared != alwaysShared {
		t.Errorf("NewChildSymbolTable() = %v, want %v", symbols.shared, alwaysShared)
	}
}

func TestSymbolTable_Shared(t *testing.T) {
	var s *SymbolTable

	// Test when the symbol table is nil
	if got := s.Shared(true); got != nil {
		t.Errorf("Shared(symbol table null) = %v, want nil", got)
	}

	// Create a new symbol table
	s = NewSymbolTable("shared test")

	// Test when alwaysShared is true and flag is false
	SerializeTableAccess = true

	if got := s.Shared(false); got != s {
		t.Errorf("Shared(alwaysShared is true) = %v, want %v", got, s)
	}

	// Test when alwaysShared is false and flag is true
	SerializeTableAccess = false

	if got := s.Shared(true); got != s {
		t.Errorf("Shared(alwaysShared is false) = %v, want %v", got, s)
	}

	// Test when alwaysShared is false and flag is false
	if got := s.Shared(false); got != s {
		t.Errorf("Shared() = %v, want %v", got, s)
	}

	// Test when setting shared flag to true
	s.shared = false
	if got := s.Shared(true); got != s {
		t.Errorf("Shared() = %v, want %v", got, s)

		if !s.shared {
			t.Errorf("shared flag should be true")
		}
	}

	// Test when setting shared flag to false
	s.shared = true
	if got := s.Shared(false); got != s {
		t.Errorf("Shared() = %v, want %v", got, s)

		if s.shared {
			t.Errorf("shared flag should be false")
		}
	}

	// Test when setting shared flag to true and crawling up the parent chain
	s.shared = false
	parent := NewSymbolTable("shared parent test")
	s.SetParent(parent)

	if got := s.Shared(true); got != s {
		t.Errorf("Shared() = %v, want %v", got, s)

		if !s.shared {
			t.Errorf("shared flag should be true")
		}

		if !parent.shared {
			t.Errorf("parent shared flag should be true")
		}
	}
}
func TestSymbolTable_Names(t *testing.T) {
	symbols := &SymbolTable{
		Name:   "test",
		parent: nil,
		symbols: map[string]*SymbolAttribute{
			"bar": {},
			"foo": {},
			"baz": {},
		},
		id:     uuid.New(),
		shared: false,
	}

	// Names should return the names in sorted order so the result is always
	// deterministic.
	expected := []string{"bar", "baz", "foo"}

	result := symbols.Names()

	if len(result) != len(expected) {
		t.Errorf("Names() returned %d elements, expected %d", len(result), len(expected))
	}

	for i, name := range expected {
		if result[i] != name {
			t.Errorf("Names() element %d is %s, expected %s", i, result[i], name)
		}
	}
}
func TestSymbolTable_Size(t *testing.T) {
	symbols := &SymbolTable{
		Name:   "test",
		parent: nil,
		symbols: map[string]*SymbolAttribute{
			"foo": {},
			"bar": {},
			"baz": {},
		},
		id:     uuid.New(),
		shared: false,
	}

	expectedSize := 3
	actualSize := symbols.Size()

	if actualSize != expectedSize {
		t.Errorf("Size() returned %d, expected %d", actualSize, expectedSize)
	}
}
