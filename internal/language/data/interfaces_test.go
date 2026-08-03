package data

import "testing"

// TestWrap_Idempotent is the BUG-92 regression test: wrapping an already-
// wrapped Interface value must return it unchanged, not double-wrap it.
// Without this, passing an any-typed value into a second any-typed
// parameter (e.g. a function forwarding its own "any" parameter into another
// function's "any" parameter) would nest Interface{Interface{v}}, and UnWrap
// only strips one layer, silently breaking any later type assertion against
// the value.
func TestWrap_Idempotent(t *testing.T) {
	once := Wrap(42)

	onceInterface, ok := once.(Interface)
	if !ok {
		t.Fatalf("Wrap(42) = %#v, want data.Interface", once)
	}

	if onceInterface.Value != 42 {
		t.Errorf("Wrap(42).Value = %v, want 42", onceInterface.Value)
	}

	twice := Wrap(once)

	twiceInterface, ok := twice.(Interface)
	if !ok {
		t.Fatalf("Wrap(Wrap(42)) = %#v, want data.Interface", twice)
	}

	if twiceInterface.Value != 42 {
		t.Errorf("Wrap(Wrap(42)).Value = %v, want 42 (unwrapped), got a nested Interface instead", twiceInterface.Value)
	}

	if twiceInterface.BaseType != onceInterface.BaseType {
		t.Errorf("Wrap(Wrap(42)).BaseType = %v, want %v (unchanged from the single wrap)", twiceInterface.BaseType, onceInterface.BaseType)
	}
}

// TestWrap_NilValue confirms Wrap(nil) still behaves as documented (BaseType
// left nil) and remains idempotent when re-wrapped.
func TestWrap_NilValue(t *testing.T) {
	wrapped := Wrap(nil)

	wi, ok := wrapped.(Interface)
	if !ok {
		t.Fatalf("Wrap(nil) = %#v, want data.Interface", wrapped)
	}

	if wi.BaseType != nil {
		t.Errorf("Wrap(nil).BaseType = %v, want nil", wi.BaseType)
	}

	rewrapped := Wrap(wrapped)

	rwi, ok := rewrapped.(Interface)
	if !ok {
		t.Fatalf("Wrap(Wrap(nil)) = %#v, want data.Interface", rewrapped)
	}

	if rwi.Value != nil || rwi.BaseType != nil {
		t.Errorf("Wrap(Wrap(nil)) = %#v, want a nil-valued Interface unchanged from the single wrap", rwi)
	}
}

// TestUnWrap_RoundTrip confirms UnWrap correctly reverses a single Wrap.
func TestUnWrap_RoundTrip(t *testing.T) {
	wrapped := Wrap("hello")

	value, baseType := UnWrap(wrapped)

	if value != "hello" {
		t.Errorf("UnWrap(Wrap(\"hello\")) value = %v, want \"hello\"", value)
	}

	if baseType == nil || baseType.Kind() != StringKind {
		t.Errorf("UnWrap(Wrap(\"hello\")) baseType = %v, want StringKind", baseType)
	}
}
