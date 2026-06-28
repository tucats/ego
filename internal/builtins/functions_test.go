package builtins

// Tests for the function-dictionary helpers in builtins/functions.go.
//
// functions.go provides:
//   - FunctionDictionary: the registry of all builtin functions
//   - AddBuiltins: installs the builtins into a symbol table
//   - FindFunction / FindName: look up a function definition by pointer
//   - CallBuiltin: call a named builtin function
//   - AddFunction: register a new function at runtime

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// ---- FindFunction ----

// Test_FindFunction_KnownFunction verifies that FindFunction returns a non-nil
// entry when given a function pointer that is in the dictionary.
func Test_FindFunction_KnownFunction(t *testing.T) {
	// Length is registered in FunctionDictionary under "len".
	def := FindFunction(Length)
	if def == nil {
		t.Fatal("FindFunction(Length) returned nil, want non-nil definition")
	}
}

// Test_FindFunction_UnknownFunctionReturnsNil verifies that FindFunction
// returns nil for a function not in the dictionary.
func Test_FindFunction_UnknownFunctionReturnsNil(t *testing.T) {
	// This anonymous function is not registered.
	unknown := func(s *symbols.SymbolTable, args data.List) (any, error) {
		return nil, nil
	}

	def := FindFunction(unknown)
	if def != nil {
		t.Errorf("FindFunction(unknown) = %v, want nil", def)
	}
}

// ---- FindName ----

// Test_FindName_KnownFunction verifies that FindName returns the expected
// name for a registered function pointer.
func Test_FindName_KnownFunction(t *testing.T) {
	name := FindName(Length)
	if name != "len" {
		t.Errorf("FindName(Length) = %q, want \"len\"", name)
	}
}

// Test_FindName_AppendFunction verifies that FindName correctly identifies the
// Append function as "append".
func Test_FindName_AppendFunction(t *testing.T) {
	name := FindName(Append)
	if name != "append" {
		t.Errorf("FindName(Append) = %q, want \"append\"", name)
	}
}

// Test_FindName_UnknownFunctionReturnsEmpty verifies that FindName returns an
// empty string for an unregistered function.
func Test_FindName_UnknownFunctionReturnsEmpty(t *testing.T) {
	unknown := func(s *symbols.SymbolTable, args data.List) (any, error) {
		return nil, nil
	}

	name := FindName(unknown)
	if name != "" {
		t.Errorf("FindName(unknown) = %q, want empty string", name)
	}
}

// ---- CallBuiltin ----

// Test_CallBuiltin_LenOnString verifies that calling "len" via CallBuiltin
// returns the correct string length.
func Test_CallBuiltin_LenOnString(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := CallBuiltin(s, "len", "hello")
	if err != nil {
		t.Fatalf("CallBuiltin(\"len\", \"hello\") error: %v", err)
	}

	if got != 5 {
		t.Errorf("CallBuiltin(\"len\", \"hello\") = %v, want 5", got)
	}
}

// Test_CallBuiltin_InvalidNameReturnsError verifies that an unrecognized
// function name returns ErrInvalidFunctionName.
func Test_CallBuiltin_InvalidNameReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	_, err := CallBuiltin(s, "nonexistentFunction")
	if err == nil {
		t.Fatal("CallBuiltin(nonexistent) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidFunctionName) {
		t.Errorf("CallBuiltin(nonexistent) error = %v, want ErrInvalidFunctionName", err)
	}
}

// Test_CallBuiltin_WrongArgCountReturnsError verifies that passing the wrong
// number of arguments to a known function returns an error.
func Test_CallBuiltin_WrongArgCountReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// "len" expects exactly 1 argument; passing 0 should fail.
	_, err := CallBuiltin(s, "len")
	if err == nil {
		t.Fatal("CallBuiltin(\"len\") with 0 args expected error, got nil")
	}
}

// ---- AddBuiltins ----

// Test_AddBuiltins_RegistersLen verifies that AddBuiltins installs the "len"
// function into the provided symbol table.
func Test_AddBuiltins_RegistersLen(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Disable extensions so only the core set is registered.
	s.Root().SetAlways(defs.ExtensionsEnabledSetting, false)

	AddBuiltins(s)

	// After AddBuiltins, the symbol table should have a "len" entry.
	v, found := s.Get("len")
	if !found {
		t.Fatal("AddBuiltins did not register \"len\"")
	}

	if v == nil {
		t.Fatal("AddBuiltins registered nil value for \"len\"")
	}
}

// Test_AddBuiltins_ExtensionFunctionsSkippedWhenDisabled verifies that
// extension-only functions (like "sizeof") are not installed when extensions
// are disabled.
func Test_AddBuiltins_ExtensionFunctionsSkippedWhenDisabled(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	s.Root().SetAlways(defs.ExtensionsEnabledSetting, false)

	AddBuiltins(s)

	// "sizeof" is an extension function and should not be present.
	_, found := s.Get("sizeof")
	if found {
		t.Error("AddBuiltins registered extension function \"sizeof\" when extensions are disabled")
	}
}

// ---- AddFunction ----

// Test_AddFunction_NewFunctionRegistered verifies that AddFunction adds a new
// entry to FunctionDictionary and returns nil error.
func Test_AddFunction_NewFunctionRegistered(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	// Use a unique name to avoid collision with other tests that share the
	// package-level FunctionDictionary.
	uniqueName := "__test_builtin_unique_9f3a"

	fd := FunctionDefinition{
		Name:        uniqueName,
		MinArgCount: 0,
		MaxArgCount: 0,
		FunctionAddress: func(s *symbols.SymbolTable, args data.List) (any, error) {
			return "ok", nil
		},
	}

	err := AddFunction(s, fd)
	if err != nil {
		t.Fatalf("AddFunction new entry error: %v", err)
	}

	// Verify it was added.
	if _, ok := FunctionDictionary[uniqueName]; !ok {
		t.Errorf("AddFunction: %q not found in FunctionDictionary after add", uniqueName)
	}

	// Clean up so other tests are not affected.
	delete(FunctionDictionary, uniqueName)
}

// Test_AddFunction_DuplicateNameReturnsError verifies that attempting to
// register a function name that already exists returns ErrFunctionAlreadyExists.
func Test_AddFunction_DuplicateNameReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	// "len" is already in the dictionary.
	fd := FunctionDefinition{
		Name:        "len",
		MinArgCount: 1,
		MaxArgCount: 1,
		FunctionAddress: func(s *symbols.SymbolTable, args data.List) (any, error) {
			return nil, nil
		},
	}

	err := AddFunction(s, fd)
	if err == nil {
		t.Fatal("AddFunction duplicate expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrFunctionAlreadyExists) {
		t.Errorf("AddFunction duplicate error = %v, want ErrFunctionAlreadyExists", err)
	}
}
