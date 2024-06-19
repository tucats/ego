package errors

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestUnwrapError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("Error message", "Additional context")

	result, err := newError(s, args)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := errors.Message("Error message").Context("Additional context")
	if !result.(*errors.Error).Is(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}

	// Now unwrap the error and make sure we get the context back.
	s.SetAlways(defs.ThisVariable, result)
	result, err = unwrap(s, data.NewList())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedText := "Additional context"
	if result.(string) != expectedText {
		t.Errorf("Expected %v, got %v", expectedText, result)
	}

}
