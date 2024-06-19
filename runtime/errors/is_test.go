package errors

import (
	native "errors"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestErrorIsCustomError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")
	theErr := errors.Message("custom error")

	st.SetAlways(defs.ThisVariable, theErr)

	// Call the Error function with no arguments
	result, err := isError(st, data.NewList(theErr))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if !data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorIsNotCustomError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")
	theErr := errors.Message("custom error")

	st.SetAlways(defs.ThisVariable, theErr)

	// Call the Error function with no arguments
	result, err := isError(st, data.NewList(errors.ErrAlignment))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorIsCustomErrorWithContext(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")
	theErr := errors.Message("custom error")

	st.SetAlways(defs.ThisVariable, theErr.Context("testing"))

	// Call the Error function with no arguments
	result, err := isError(st, data.NewList(theErr))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if !data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorIsCustomErrorWithFunction(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")
	theErr := errors.Message("custom error")

	st.SetAlways(defs.ThisVariable, theErr.In("testing"))

	// Call the Error function with no arguments
	result, err := isError(st, data.NewList(theErr))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if !data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorIsCustomErrorWithLocation(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")
	theErr := errors.Message("custom error")

	st.SetAlways(defs.ThisVariable, theErr.At(100, 200).In("testing"))

	// Call the Error function with no arguments
	result, err := isError(st, data.NewList(theErr))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if !data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}

func TestErrorIsGoError(t *testing.T) {
	// Create a new symbol table
	st := symbols.NewSymbolTable("test")

	st.SetAlways(defs.ThisVariable, native.ErrUnsupported)

	// Call the Error function with no arguments
	result, err := Error(st, data.NewList(native.ErrUnsupported))

	// Check if the function returned the expected error message
	if err != nil {
		t.Errorf("Error function returned an error: %v", err)
	}

	// Check if the function returned the expected error message
	if data.Bool(result) {
		t.Errorf("Error function returned an unexpected error message: %v", result)
	}
}
