package bytecode

// this_test.go tests the bytecode instructions in this.go that bind a
// method's receiver ("this"):
//
//   getThisByteCode — GetThis opcode
//
// # BUG-64 (docs/ISSUES.md)
//
// getThisByteCode auto-dereferences an Ego pointer (*any) receiver so field
// writes propagate to the caller. Before the fix, that dereference also
// discarded Ego's own pointer-type marker, so a pointer receiver returned as
// its own declared "*T" type failed strict-mode type checking, and a
// pointer-receiver method called on a plain (non-&) value ("auto-address")
// was never boxed as a pointer at all. Both are fixed by re-boxing the
// dereferenced-or-original value into a fresh *any whenever the receiver is
// NOT a value receiver (byValue == false). See the design comment on
// getThisByteCode in this.go for the full explanation.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
)

// Test_getThisByteCode_PointerReceiverExplicitPointer verifies that when the
// caller passed an explicit Ego pointer (*any wrapping a *data.Struct) as
// the receiver, and the receiver is declared as a pointer ("func (b *T)"),
// the value bound to the receiver name is re-boxed as *any so it still
// carries Ego's pointer-type marker.
func Test_getThisByteCode_PointerReceiverExplicitPointer(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 1})
	var boxed any = s
	ptr := &boxed

	tc := newTestContext(t)
	tc.ctx.pushThis("b", ptr)

	err := getThisByteCode(tc.ctx, []any{"b", false})

	tc.assertNoError(err)

	v, found := tc.ctx.get("b")
	if !found {
		t.Fatal("expected 'b' to be bound after GetThis")
	}

	rewrapped, ok := v.(*any)
	if !ok {
		t.Fatalf("expected 'b' to be re-boxed as *any, got %T", v)
	}

	if _, ok := (*rewrapped).(*data.Struct); !ok {
		t.Errorf("expected the boxed value to still be *data.Struct, got %T", *rewrapped)
	}
}

// Test_getThisByteCode_PointerReceiverAutoAddress verifies the "auto-address"
// case: the caller passed a bare *data.Struct (a receiver called on a plain,
// non-pointer value), never wrapped in *any to begin with. A pointer
// receiver must still end up boxed, matching Go's implicit "take the
// address" rule for calling a pointer-receiver method on an addressable
// value.
func Test_getThisByteCode_PointerReceiverAutoAddress(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 1})

	tc := newTestContext(t)
	tc.ctx.pushThis("b", s) // no *any wrapper - the auto-address case

	err := getThisByteCode(tc.ctx, []any{"b", false})

	tc.assertNoError(err)

	v, found := tc.ctx.get("b")
	if !found {
		t.Fatal("expected 'b' to be bound after GetThis")
	}

	if _, ok := v.(*any); !ok {
		t.Fatalf("expected 'b' to be boxed as *any even without an existing pointer, got %T", v)
	}
}

// Test_getThisByteCode_ValueReceiverExplicitPointer verifies that a value
// receiver ("func (c T)") called via an explicit pointer variable still
// receives the bare dereferenced value, NOT a re-boxed pointer - the $new()
// copy step generateFunctionBytecode emits right after GetThis expects a
// plain value to copy.
func Test_getThisByteCode_ValueReceiverExplicitPointer(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 1})
	var boxed any = s
	ptr := &boxed

	tc := newTestContext(t)
	tc.ctx.pushThis("c", ptr)

	err := getThisByteCode(tc.ctx, []any{"c", true})

	tc.assertNoError(err)

	v, found := tc.ctx.get("c")
	if !found {
		t.Fatal("expected 'c' to be bound after GetThis")
	}

	if _, ok := v.(*any); ok {
		t.Fatalf("expected 'c' to be the bare dereferenced value for a value receiver, got boxed *any")
	}

	if _, ok := v.(*data.Struct); !ok {
		t.Errorf("expected 'c' to be *data.Struct, got %T", v)
	}
}

// Test_getThisByteCode_ValueReceiverAutoAddress verifies that a value
// receiver called on a plain (already-bare) value is left unboxed too.
func Test_getThisByteCode_ValueReceiverAutoAddress(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 1})

	tc := newTestContext(t)
	tc.ctx.pushThis("c", s)

	err := getThisByteCode(tc.ctx, []any{"c", true})

	tc.assertNoError(err)

	v, found := tc.ctx.get("c")
	if !found {
		t.Fatal("expected 'c' to be bound after GetThis")
	}

	if _, ok := v.(*any); ok {
		t.Fatalf("expected 'c' to remain unboxed for a value receiver, got boxed *any")
	}
}

// Test_getThisByteCode_PlainNameOperand verifies backward compatibility: a
// bare name operand (not the two-element []any{name, byValue} form) is still
// accepted, and behaves like byValue == false was passed (matching the
// original, pre-BUG-64 behavior for any hypothetical caller that never
// supplies byValue).
func Test_getThisByteCode_PlainNameOperand(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 1})

	tc := newTestContext(t)
	tc.ctx.pushThis("b", s)

	err := getThisByteCode(tc.ctx, "b")

	tc.assertNoError(err)

	v, found := tc.ctx.get("b")
	if !found {
		t.Fatal("expected 'b' to be bound after GetThis")
	}

	if _, ok := v.(*any); !ok {
		t.Fatalf("expected 'b' to be boxed as *any for the plain-operand (pointer-receiver) path, got %T", v)
	}
}

// Test_getThisByteCode_EmptyReceiverStack verifies that GetThis is a no-op
// (no error, no symbol created) when the receiver stack is empty.
func Test_getThisByteCode_EmptyReceiverStack(t *testing.T) {
	tc := newTestContext(t)

	err := getThisByteCode(tc.ctx, []any{"b", false})

	tc.assertNoError(err)

	if _, found := tc.ctx.get("b"); found {
		t.Error("expected 'b' to remain unbound when the receiver stack was empty")
	}
}
