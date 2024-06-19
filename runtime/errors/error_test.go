package errors

import (
	native "errors"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestErrorCustomError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	st.SetAlways(defs.ThisVariable, errors.Message("Custom error"))

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList())

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if result != "Custom error" {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorWithContext(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	theError := errors.Message("custom error").Context("testing")
	st.SetAlways(defs.ThisVariable, theError)

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList())

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if result != "custom error: testing" {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorWithFunctionName(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	theError := errors.Message("custom error").
		Context("testing").In("gotest")
	st.SetAlways(defs.ThisVariable, theError)

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList())

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if result != "in gotest, custom error: testing" {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorBuiltinError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	st.SetAlways(defs.ThisVariable, errors.ErrUnknownIdentifier)

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList())

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if result != errors.ErrUnknownIdentifier.Error() {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorGoError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	st.SetAlways(defs.ThisVariable, native.ErrUnsupported)

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList())

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if result != native.ErrUnsupported.Error() {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}
