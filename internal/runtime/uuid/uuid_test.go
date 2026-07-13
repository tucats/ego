package uuid

import (
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestParseUUID_ValidInput(t *testing.T) {
	const valid = "ab34d542-a437-408a-b0ca-38ea5d78696f"

	s := symbols.NewSymbolTable("testing")

	result, err := parseUUID(s, data.NewList(valid))
	if err != nil {
		t.Fatalf("parseUUID(%q): unexpected wrapper-level error: %v", valid, err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("parseUUID(%q): result is not a data.List: %T", valid, result)
	}

	if list.Get(1) != nil {
		t.Errorf("parseUUID(%q): expected nil error, got %v", valid, list.Get(1))
	}

	if list.Get(0) == nil {
		t.Errorf("parseUUID(%q): expected a non-nil UUID struct", valid)
	}
}

// TestParseUUID_InvalidInput is the regression test for BUG-40: invalid input
// must produce a normal, catchable (nil, error) pair inside the returned
// data.List rather than aborting the program.
func TestParseUUID_InvalidInput(t *testing.T) {
	const invalid = "not-a-uuid"

	s := symbols.NewSymbolTable("testing")

	result, err := parseUUID(s, data.NewList(invalid))
	if err != nil {
		t.Fatalf("parseUUID(%q): wrapper-level error must be nil for a two-value result, got %v", invalid, err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("parseUUID(%q): result must be a data.List so the error is catchable, got %T", invalid, result)
	}

	if list.Len() != 2 {
		t.Fatalf("parseUUID(%q): expected 2-element list, got %d", invalid, list.Len())
	}

	if list.Get(0) != nil {
		t.Errorf("parseUUID(%q): expected nil id on failure, got %v", invalid, list.Get(0))
	}

	listErr, ok := list.Get(1).(error)
	if !ok || listErr == nil {
		t.Fatalf("parseUUID(%q): expected a non-nil error in the result list, got %v", invalid, list.Get(1))
	}
}

// TestUUIDTypeDef_ZeroValueIsUsable verifies that UUIDTypeDef's zero-value
// constructor (wired up in the package's init() function) produces a struct
// carrying a valid native uuid.UUID value -- matching Go's own zero value for
// uuid.UUID ([16]byte), which is the valid nil UUID -- rather than the empty,
// native-field-less struct that Type.InstanceOf falls back to for a StructKind
// type with no registered constructor.
func TestUUIDTypeDef_ZeroValueIsUsable(t *testing.T) {
	v := UUIDTypeDef.InstanceOf(UUIDTypeDef)

	s := symbols.NewSymbolTable("testing")
	s.SetAlways(defs.ThisVariable, v)

	result, err := toString(s, data.NewList())
	if err != nil {
		t.Fatalf("String() on zero-value UUID: unexpected error: %v", err)
	}

	const wantNil = "00000000-0000-0000-0000-000000000000"
	if result != wantNil {
		t.Errorf("String() on zero-value UUID = %v, want %v", result, wantNil)
	}
}
