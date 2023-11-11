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
