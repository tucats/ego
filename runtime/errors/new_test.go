package errors

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestNewError_SingleArgument(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("Error message")

	result, err := newError(s, args)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := errors.Message("Error message")
	if !result.(*errors.Error).Is(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestNewError_TwoArguments(t *testing.T) {
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
}

func TestNewError_TooFewArguments(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList()

	_, err := newError(s, args)

	if err == nil || err.Error() != errors.ErrArgumentCount.In("New").Error() {
		t.Errorf("Expected error %v, got %v", errors.ErrArgumentCount.In("New"), err)
	}
}

func TestNewError_TooManyArguments(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("Error message", "Additional context", "Extra argument")

	_, err := newError(s, args)

	if err == nil || err.Error() != errors.ErrArgumentCount.In("New").Error() {
		t.Errorf("Expected error %v, got %v", errors.ErrArgumentCount.In("New"), err)
	}
}

func TestNewError_WithModuleAndLine(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	s.SetAlways(defs.ModuleVariable, "test_module")
	s.SetAlways(defs.LineVariable, 123)
	args := data.NewList("Error message", "additional context")
	verbose = true

	result, err := newError(s, args)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := errors.Message("Error message").In("test_module").At(123, 0)
	if !result.(*errors.Error).Is(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}

	fullText := result.(*errors.Error).Error()
	expectedText := "at test_module(line 123), Error message: additional context"
	if fullText != expectedText {
		t.Errorf("Expected %s, got %s", expectedText, fullText)
	}
}
