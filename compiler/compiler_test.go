package compiler

import (
	"strings"
	"testing"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/symbols"
)

func TestAddStandard_NilSymbolTable(t *testing.T) {
	// Arrange
	var s *symbols.SymbolTable

	// Act
	result := AddStandard(s)

	// Assert
	if result {
		t.Error("Expected false, but got true when input symbol table is nil")
	}
}

func TestAddStandardFunctions(t *testing.T) {
	// Create a new symbol table
	s := symbols.NewSymbolTable("test")

	// Call the function to add standard functions
	added := AddStandard(s)

	// Check if the function added standard functions
	if !added {
		t.Errorf("Expected AddStandard to add standard functions to the symbol table, but got %v", added)
	}

	// Check if the symbol table contains the standard functions
	for name := range builtins.FunctionDictionary {
		// Skip if if the name has a "." in it.
		if dot := strings.Index(name, "."); dot < 0 {
			if _, found := s.Get(name); !found {
				t.Errorf("Expected symbol table to contain %s, but it was not found", name)
			}
		}
	}
}

func TestAddStandardFunctionsTwice(t *testing.T) {
	// Create a new symbol table
	s := symbols.NewSymbolTable("test")

	// Call the function to add standard functions
	added := AddStandard(s)

	// Check if the function added standard functions
	if !added {
		t.Errorf("Expected AddStandard to add standard functions to the symbol table, but got %v", added)
	}

	// Call the function to add standard functions
	added = AddStandard(s)

	// Check if the function added standard functions
	if added {
		t.Errorf("Expected AddStandard not to add standard functions to the symbol table, but got %v", added)
	}

	// Check if the symbol table contains the standard functions
	for name := range builtins.FunctionDictionary {
		// Skip if if the name has a "." in it.
		if dot := strings.Index(name, "."); dot < 0 {
			if _, found := s.Get(name); !found {
				t.Errorf("Expected symbol table to contain %s, but it was not found", name)
			}
		}
	}
}
