package bytecode

// types_test.go contains unit tests for the functions in types.go:
//
//   - typeOfByteCode     — pops a value, pushes its *data.Type
//   - unwrapByteCode     — unwraps data.Interface; optionally asserts to a named type
//   - staticTypingByteCode — sets the context's type-strictness level
//   - requiredTypeByteCode — enforces a required type on the top-of-stack value
//   - addressOfByteCode  — pushes a *any pointer to a named symbol's storage
//   - deRefByteCode      — dereferences a *any pointer stored in a named symbol
//
// All tests use the shared testContext harness from testhelpers_test.go.
//
// # Known issues (see docs/BYTECODE_ISSUES.md for full details)
//
//   TYPES-1: deRefByteCode — nil guard inside the double-dereference branch
//            checks the wrong variable (content vs. c3), leaving *c3 vulnerable
//            to a panic when c3 is a nil *any.
//   TYPES-2: relaxedConformanceCheck — reflect.TypeOf(v).String() panics when
//            v is nil and the operand is a string type name.
//   TYPES-3: relaxedConformanceCheck — the int-operand switch is dead code for
//            all cases except IntKind because t = i.(int) is always a plain int.
//
// # BUG-03: type assertions always succeed in non-strict mode (FIXED)
//
//   The unwrapByteCode handler had separate code paths for strict vs. non-strict
//   type-strictness levels:
//
//     Strict:     check actualType.IsType(newType); push (nil, false) on mismatch.
//     Non-strict: call data.Coerce unconditionally — any value can be "asserted"
//                 to any type by silently converting it.
//
//   This made the comma-ok idiom useless in the default dynamic mode: every
//   type assertion returned ok=true, so there was no way to ask "does this
//   interface value actually hold a T?"  Cast functions (int(), string(), …)
//   already provide coercion; assertions must validate.
//
//   Fix: unwrapByteCode now applies the same type-match check in ALL modes.
//   A mismatch pushes (nil, false) regardless of the type-strictness level.
//   The "any" / "interface{}" target type is always a match (every value
//   satisfies the empty interface).  The fix also normalizes the "any" alias
//   to "interface{}" so the lookup loop correctly resolves the type.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─── Section 1: typeOfByteCode ────────────────────────────────────────────────

// Test_typeOfByteCode_Int verifies that the type of an int value is IntType.
func Test_typeOfByteCode_Int(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := typeOfByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(data.IntType)
}

// Test_typeOfByteCode_String verifies that the type of a string is StringType.
func Test_typeOfByteCode_String(t *testing.T) {
	tc := newTestContext(t).withStack("hello")

	err := typeOfByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(data.StringType)
}

// Test_typeOfByteCode_Bool verifies that the type of a bool is BoolType.
func Test_typeOfByteCode_Bool(t *testing.T) {
	tc := newTestContext(t).withStack(true)

	err := typeOfByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(data.BoolType)
}

// Test_typeOfByteCode_Float64 verifies that the type of a float64 is Float64Type.
func Test_typeOfByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withStack(float64(3.14))

	err := typeOfByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(data.Float64Type)
}

// Test_typeOfByteCode_Nil verifies that a nil value reports as NilType.
func Test_typeOfByteCode_Nil(t *testing.T) {
	tc := newTestContext(t).withStack(nil)

	err := typeOfByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(data.NilType)
}

// Test_typeOfByteCode_EmptyStack verifies that popping from an empty stack
// returns an error rather than panicking.
func Test_typeOfByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := typeOfByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("typeOfByteCode on empty stack: expected error, got nil")
	}
}

// Test_typeOfByteCode_IgnoresOperand verifies that the operand is ignored —
// whatever is passed does not affect the result.
func Test_typeOfByteCode_IgnoresOperand(t *testing.T) {
	tc := newTestContext(t).withStack(99)

	err := typeOfByteCode(tc.ctx, "should be ignored")

	tc.assertNoError(err)
	tc.assertTopStack(data.IntType)
}

// ─── Section 2: unwrapByteCode ────────────────────────────────────────────────

// Test_unwrapByteCode_NilOperand_PlainInt verifies the nil-operand "plain
// unwrap" path: a plain int value (not wrapped in data.Interface) is pushed
// back with its type on top.
func Test_unwrapByteCode_NilOperand_PlainInt(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := unwrapByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// Type is on top, value is below.
	tc.assertTopStack(42)
	tc.assertTopStack(data.IntType)
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_NilOperand_Interface verifies that a data.Interface
// wrapper is stripped: the inner value and its type are pushed onto the stack.
func Test_unwrapByteCode_NilOperand_Interface(t *testing.T) {
	wrapped := data.Interface{Value: 77, BaseType: data.IntType}
	tc := newTestContext(t).withStack(wrapped)

	err := unwrapByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(77)
	tc.assertTopStack(data.IntType)
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_TypeOperand behaves identically to the nil-operand case
// — the special "type" keyword operand triggers the same unwrap-only path.
func Test_unwrapByteCode_TypeOperand(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := unwrapByteCode(tc.ctx, "type")

	tc.assertNoError(err)
	tc.assertTopStack(42)
	tc.assertTopStack(data.IntType)
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_NamedType_Relaxed_Success verifies that asserting an int
// to "int" in non-strict mode succeeds and pushes (value, true).
func Test_unwrapByteCode_NamedType_Relaxed_Success(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	// bool (true) is on top; value is below.
	tc.assertTopStack(true)
	tc.assertTopStack(42)
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_NamedType_Strict_Success verifies that asserting an int
// to "int" in strict mode succeeds when the types match.
func Test_unwrapByteCode_NamedType_Strict_Success(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(true) // ok
	tc.assertTopStack(42)   // value
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_NamedType_Strict_Mismatch verifies that asserting a
// string value to "int" in strict mode fails cleanly: (nil, false) are pushed
// and no error is returned — the caller checks the bool.
func Test_unwrapByteCode_NamedType_Strict_Mismatch(t *testing.T) {
	tc := newTestContext(t).withStack("not-an-int").
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(false) // ok = false
	tc.assertTopStack(nil)   // value = nil
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_UnknownType verifies that naming a type that does not
// exist in the type registry or the symbol table returns ErrInvalidType.
func Test_unwrapByteCode_UnknownType(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := unwrapByteCode(tc.ctx, "nosuchtype")

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_unwrapByteCode_EmptyStack verifies that popping from an empty stack
// returns an error.
func Test_unwrapByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := unwrapByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("unwrapByteCode on empty stack: expected error, got nil")
	}
}

// ─── Section 2b: unwrapByteCode — BUG-03 regression tests ────────────────────
//
// These tests verify that type assertions correctly validate the actual stored
// type in ALL type-strictness modes after the BUG-03 fix.
//
// Background: before the fix, the non-strict code path called data.Coerce on
// every assertion, making every assertion succeed by silently converting the
// value.  The strict path already worked correctly.  The fix unifies both paths:
// assertions always check actualType.IsType(newType) regardless of the mode.

// Test_unwrapByteCode_BUG03_Relaxed_WrongType is the primary BUG-03 regression
// test.  In relaxed (non-strict) mode, asserting a string to "int" must push
// (nil, false) — the same result as strict mode — rather than coercing the
// string to an int and claiming success.
func Test_unwrapByteCode_BUG03_Relaxed_WrongType(t *testing.T) {
	tc := newTestContext(t).withStack("not-an-int").
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(false) // ok = false (WRONG type)
	tc.assertTopStack(nil)   // value = nil
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_BUG03_Dynamic_WrongType verifies the same behavior in
// dynamic mode (NoTypeEnforcement), which was the default runtime mode and the
// mode most users encounter.
func Test_unwrapByteCode_BUG03_Dynamic_WrongType(t *testing.T) {
	tc := newTestContext(t).withStack("not-an-int").
		withTypeStrictness(defs.NoTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(false) // ok = false
	tc.assertTopStack(nil)   // value = nil
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_BUG03_Relaxed_CorrectType verifies that the fix does not
// break the success path: a genuine int-to-int assertion still returns (42, true)
// in relaxed mode.
func Test_unwrapByteCode_BUG03_Relaxed_CorrectType(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(true) // ok = true
	tc.assertTopStack(42)   // value = 42
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_BUG03_Dynamic_CorrectType verifies the success path in
// dynamic mode.
func Test_unwrapByteCode_BUG03_Dynamic_CorrectType(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.NoTypeEnforcement)

	err := unwrapByteCode(tc.ctx, "int")

	tc.assertNoError(err)
	tc.assertTopStack(true) // ok = true
	tc.assertTopStack(42)   // value = 42
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_BUG03_IntToString verifies that asserting an int to
// "string" fails (nil, false) in all modes.  Before the fix, data.Coerce would
// convert 42 to "42" and report success.
func Test_unwrapByteCode_BUG03_IntToString(t *testing.T) {
	for _, mode := range []int{defs.StrictTypeEnforcement, defs.RelaxedTypeEnforcement, defs.NoTypeEnforcement} {
		tc := newTestContext(t).withStack(42).withTypeStrictness(mode)

		err := unwrapByteCode(tc.ctx, "string")

		tc.assertNoError(err)
		tc.assertTopStack(false)
		tc.assertTopStack(nil)
		tc.assertStackEmpty()
	}
}

// Test_unwrapByteCode_BUG03_FloatToInt verifies that float64 → int assertion
// fails.  In non-strict mode, data.Coerce would truncate 3.14 to 3 and report
// success; after the fix it returns (nil, false) because float64 ≠ int.
func Test_unwrapByteCode_BUG03_FloatToInt(t *testing.T) {
	for _, mode := range []int{defs.StrictTypeEnforcement, defs.RelaxedTypeEnforcement, defs.NoTypeEnforcement} {
		tc := newTestContext(t).withStack(float64(3.14)).withTypeStrictness(mode)

		err := unwrapByteCode(tc.ctx, "int")

		tc.assertNoError(err)
		tc.assertTopStack(false)
		tc.assertTopStack(nil)
		tc.assertStackEmpty()
	}
}

// Test_unwrapByteCode_BUG03_AnyAlias verifies that the "any" keyword is
// correctly recognized as an alias for "interface{}" and that asserting to "any"
// always succeeds (every value satisfies the empty interface).
//
// Before the BUG-03 fix, the type lookup loop compared td.Kind.Name() (which
// returns "interface{}") against the target string "any", so the lookup failed
// and unwrapByteCode returned ErrInvalidType.  The fix normalizes "any" to
// "interface{}" before the loop.
func Test_unwrapByteCode_BUG03_AnyAlias(t *testing.T) {
	// An int value asserted to "any" must always succeed.
	for _, mode := range []int{defs.StrictTypeEnforcement, defs.RelaxedTypeEnforcement, defs.NoTypeEnforcement} {
		tc := newTestContext(t).withStack(42).withTypeStrictness(mode)

		err := unwrapByteCode(tc.ctx, "any")

		// Must not return ErrInvalidType (the pre-fix behavior).
		tc.assertNoError(err)
		tc.assertTopStack(true) // ok = true — any satisfies the empty interface
		tc.assertTopStack(42)   // value unchanged
		tc.assertStackEmpty()
	}
}

// Test_unwrapByteCode_BUG03_InterfaceAlias verifies that "interface{}" (the
// canonical spelling of the any type) also always succeeds.
func Test_unwrapByteCode_BUG03_InterfaceAlias(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.NoTypeEnforcement)

	// "interface{}" as a single token would not be valid Ego syntax; the lexer
	// produces two tokens.  The unwrapByteCode lookup is driven by the compiled
	// type name, which can be a single string.  We use data.InterfaceTypeName
	// directly to simulate what a two-token assertion compiles to.
	err := unwrapByteCode(tc.ctx, data.InterfaceTypeName)

	tc.assertNoError(err)
	tc.assertTopStack(true)
	tc.assertTopStack(42)
	tc.assertStackEmpty()
}

// Test_unwrapByteCode_BUG03_AllModesConsistent verifies that all three
// type-strictness modes produce identical results for both the success case
// (int → int) and the failure case (string → int).  The BUG-03 fix makes
// non-strict modes behave the same as strict mode.
func Test_unwrapByteCode_BUG03_AllModesConsistent(t *testing.T) {
	modes := []struct {
		name string
		mode int
	}{
		{"strict", defs.StrictTypeEnforcement},
		{"relaxed", defs.RelaxedTypeEnforcement},
		{"dynamic", defs.NoTypeEnforcement},
	}

	for _, m := range modes {
		// Success case: int value asserted to int.
		tc := newTestContext(t).withStack(99).withTypeStrictness(m.mode)

		if err := unwrapByteCode(tc.ctx, "int"); err != nil {
			t.Errorf("mode %s, success case: unexpected error: %v", m.name, err)
		}

		okVal, _ := tc.ctx.Pop()
		if b, _ := okVal.(bool); !b {
			t.Errorf("mode %s, success case: ok = %v, want true", m.name, okVal)
		}

		// Failure case: string value asserted to int.
		tc2 := newTestContext(t).withStack("hello").withTypeStrictness(m.mode)

		if err := unwrapByteCode(tc2.ctx, "int"); err != nil {
			t.Errorf("mode %s, failure case: unexpected error: %v", m.name, err)
		}

		okVal2, _ := tc2.ctx.Pop()
		if b, _ := okVal2.(bool); b {
			t.Errorf("mode %s, failure case: ok = %v, want false", m.name, okVal2)
		}
	}
}

// ─── Section 3: staticTypingByteCode ─────────────────────────────────────────

// Test_staticTypingByteCode_SetStrict verifies that pushing 0 sets the context
// to StrictTypeEnforcement and updates the symbol table variable.
func Test_staticTypingByteCode_SetStrict(t *testing.T) {
	tc := newTestContext(t).withStack(defs.StrictTypeEnforcement)

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.typeStrictness != defs.StrictTypeEnforcement {
		t.Errorf("typeStrictness: got %d, want %d", tc.ctx.typeStrictness, defs.StrictTypeEnforcement)
	}

	tc.assertSymbolValue(defs.TypeCheckingVariable, defs.StrictTypeEnforcement)
}

// Test_staticTypingByteCode_SetRelaxed verifies that pushing 1 sets the
// context to RelaxedTypeEnforcement.
func Test_staticTypingByteCode_SetRelaxed(t *testing.T) {
	tc := newTestContext(t).withStack(defs.RelaxedTypeEnforcement)

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.typeStrictness != defs.RelaxedTypeEnforcement {
		t.Errorf("typeStrictness: got %d, want %d", tc.ctx.typeStrictness, defs.RelaxedTypeEnforcement)
	}

	tc.assertSymbolValue(defs.TypeCheckingVariable, defs.RelaxedTypeEnforcement)
}

// Test_staticTypingByteCode_SetNoEnforcement verifies that pushing 2 sets the
// context to NoTypeEnforcement.
func Test_staticTypingByteCode_SetNoEnforcement(t *testing.T) {
	tc := newTestContext(t).withStack(defs.NoTypeEnforcement)

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.typeStrictness != defs.NoTypeEnforcement {
		t.Errorf("typeStrictness: got %d, want %d", tc.ctx.typeStrictness, defs.NoTypeEnforcement)
	}
}

// Test_staticTypingByteCode_BelowRange verifies that a value below the valid
// range (< StrictTypeEnforcement = 0) returns ErrInvalidValue.
func Test_staticTypingByteCode_BelowRange(t *testing.T) {
	tc := newTestContext(t).withStack(-1)

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidValue)
}

// Test_staticTypingByteCode_AboveRange verifies that a value above the valid
// range (> NoTypeEnforcement = 2) returns ErrInvalidValue.
func Test_staticTypingByteCode_AboveRange(t *testing.T) {
	tc := newTestContext(t).withStack(3)

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidValue)
}

// Test_staticTypingByteCode_StackMarker verifies that a StackMarker on the
// stack returns ErrFunctionReturnedVoid instead of a conversion error.
func Test_staticTypingByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("void"))

	err := staticTypingByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_staticTypingByteCode_EmptyStack verifies that an empty stack returns an
// error rather than proceeding with an undefined value.
func Test_staticTypingByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := staticTypingByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("staticTypingByteCode on empty stack: expected error, got nil")
	}
}

// Test_staticTypingByteCode_NonIntValue verifies that a non-integer value
// (like a string) that cannot be coerced to int returns an error.
func Test_staticTypingByteCode_NonIntValue(t *testing.T) {
	tc := newTestContext(t).withStack(data.NewArray(data.IntType, 1)) // *data.Array can't coerce to int

	err := staticTypingByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("staticTypingByteCode with non-int value: expected error, got nil")
	}
}

// ─── Section 4: requiredTypeByteCode ─────────────────────────────────────────

// Test_requiredTypeByteCode_IntMatches_Relaxed verifies that an int value
// passes the RequiredType check when the operand is IntType in relaxed mode.
func Test_requiredTypeByteCode_IntMatches_Relaxed(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, data.IntType)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_requiredTypeByteCode_IntMatches_Strict verifies the same in strict mode.
func Test_requiredTypeByteCode_IntMatches_Strict(t *testing.T) {
	tc := newTestContext(t).withStack(42).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, data.IntType)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_requiredTypeByteCode_StringForInt_Strict verifies that passing a string
// when IntType is required in strict mode returns ErrArgumentType.
func Test_requiredTypeByteCode_StringForInt_Strict(t *testing.T) {
	tc := newTestContext(t).withStack("not-an-int").
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, data.IntType)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_requiredTypeByteCode_BoolMatches_Strict verifies that a bool passes
// when BoolType is required in strict mode.
func Test_requiredTypeByteCode_BoolMatches_Strict(t *testing.T) {
	tc := newTestContext(t).withStack(true).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, data.BoolType)

	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_requiredTypeByteCode_StackMarker returns ErrFunctionReturnedVoid when
// a StackMarker is on the stack.
func Test_requiredTypeByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("void"))

	err := requiredTypeByteCode(tc.ctx, data.IntType)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_requiredTypeByteCode_EmptyStack verifies that an empty stack returns an
// error.
func Test_requiredTypeByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := requiredTypeByteCode(tc.ctx, data.IntType)

	if err == nil {
		t.Error("requiredTypeByteCode on empty stack: expected error, got nil")
	}
}

// ─── Section 5: addressOfByteCode ────────────────────────────────────────────

// Test_addressOfByteCode_KnownSymbol verifies that addressing a known symbol
// pushes a non-nil *any onto the stack.
func Test_addressOfByteCode_KnownSymbol(t *testing.T) {
	tc := newTestContext(t).withSymbol("x", 42)

	err := addressOfByteCode(tc.ctx, "x")

	tc.assertNoError(err)

	// Pop the address and verify it is a non-nil *any.
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop after addressOfByteCode failed: %v", popErr)
	}

	addr, ok := v.(*any)
	if !ok {
		t.Fatalf("addressOfByteCode: expected *any on stack, got %T", v)
	}

	if addr == nil {
		t.Error("addressOfByteCode: returned nil address")
	}
}

// Test_addressOfByteCode_AddressPointsToValue verifies that the pushed *any
// actually points to the symbol's value.
func Test_addressOfByteCode_AddressPointsToValue(t *testing.T) {
	tc := newTestContext(t).withSymbol("y", 99)

	err := addressOfByteCode(tc.ctx, "y")

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()
	addr := v.(*any)

	// Dereference the address — the stored value should be 99.
	if *addr != 99 {
		t.Errorf("addressOfByteCode: *addr = %v, want 99", *addr)
	}
}

// Test_addressOfByteCode_UnknownSymbol verifies that naming an undeclared
// symbol returns ErrUnknownIdentifier.
func Test_addressOfByteCode_UnknownSymbol(t *testing.T) {
	tc := newTestContext(t)

	err := addressOfByteCode(tc.ctx, "undeclared")

	tc.assertError(err, errors.ErrUnknownIdentifier)
}

// ─── Section 6: deRefByteCode ─────────────────────────────────────────────────

// Test_deRefByteCode_ValidPointer verifies successful double-dereference: the
// symbol holds a *any (an Ego pointer to a target value), and deRefByteCode
// pushes the pointed-to value.
//
// Memory layout:
//
//	targetVal (any = 42)       ← the ultimate value
//	targetPtr (*any = &targetVal) ← the Ego pointer value stored in symbol "p"
//	symbol "p" stores targetPtr
//	GetAddress("p") returns *any pointing to the symbol slot
func Test_deRefByteCode_ValidPointer(t *testing.T) {
	tc := newTestContext(t)

	targetVal := any(42)
	targetPtr := &targetVal // a *any — the Ego pointer value

	tc.ctx.symbols.SetAlways("p", targetPtr)

	err := deRefByteCode(tc.ctx, "p")

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_deRefByteCode_ImmutablePointer verifies that when the pointed-to value
// is wrapped in a data.Immutable (a read-only constant), the inner value is
// unwrapped and pushed.
func Test_deRefByteCode_ImmutablePointer(t *testing.T) {
	tc := newTestContext(t)

	innerVal := any(data.Immutable{Value: "const-value"})
	tc.ctx.symbols.SetAlways("c", &innerVal)

	err := deRefByteCode(tc.ctx, "c")

	tc.assertNoError(err)
	tc.assertTopStack("const-value")
}

// Test_deRefByteCode_UnknownSymbol verifies ErrUnknownIdentifier for a name
// that is not in the current scope.
func Test_deRefByteCode_UnknownSymbol(t *testing.T) {
	tc := newTestContext(t)

	err := deRefByteCode(tc.ctx, "undeclared")

	tc.assertError(err, errors.ErrUnknownIdentifier)
}

// Test_deRefByteCode_NotAPointer verifies ErrNotAPointer when the symbol holds
// a plain value (not a *any).  A plain int is not an Ego pointer.
func Test_deRefByteCode_NotAPointer(t *testing.T) {
	tc := newTestContext(t).withSymbol("notptr", 42)

	err := deRefByteCode(tc.ctx, "notptr")

	tc.assertError(err, errors.ErrNotAPointer)
}

// Test_deRefByteCode_NilSymbolValue documents the behavior when a symbol holds
// an untyped nil — the runtime returns ErrNotAPointer because untyped nil
// cannot be type-asserted to *any (the assertion returns false), so the code
// reaches the ErrNotAPointer branch rather than the nil-pointer guard.
func Test_deRefByteCode_NilSymbolValue(t *testing.T) {
	tc := newTestContext(t)

	// Store untyped nil as the symbol value.  nil.(*any) type-asserts to
	// (nil, false), so the code reaches the ErrNotAPointer branch.
	tc.ctx.symbols.SetAlways("nilsym", nil)

	err := deRefByteCode(tc.ctx, "nilsym")

	tc.assertError(err, errors.ErrNotAPointer)
}

// ─── Section 7: TYPES-1/2/3 fix verification ─────────────────────────────────

// Test_deRefByteCode_TypedNilInnerPointer_TYPES1 verifies the TYPES-1 fix:
// when a symbol holds a typed nil *any pointer (an Ego pointer variable that
// was declared but never assigned), deRefByteCode now returns
// ErrNilPointerReference instead of panicking on *c3.
//
// Before the fix, the guard checked `content` (the outer pointer, always
// non-nil) instead of `c3` (the inner pointer), so a nil c3 reached the
// dereference `*c3` and caused a runtime panic.
func Test_deRefByteCode_TypedNilInnerPointer_TYPES1(t *testing.T) {
	var nilInnerPtr *any = nil

	tc := newTestContext(t)

	// Store a typed nil *any (simulating an unassigned Ego pointer variable).
	tc.ctx.symbols.SetAlways("p", nilInnerPtr)

	err := deRefByteCode(tc.ctx, "p")

	// After TYPES-1 fix: c3 == nil is detected and ErrNilPointerReference returned.
	tc.assertError(err, errors.ErrNilPointerReference)
}

// Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2 verifies the
// TYPES-2 fix: when the operand is a string type name and the value is nil,
// the function returns ErrArgumentType rather than panicking from
// reflect.TypeOf(nil).String().
func Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2(t *testing.T) {
	tc := newTestContext(t)

	_, err := relaxedConformanceCheck(tc.ctx, "int", nil)

	// After TYPES-2 fix: nil value with string operand returns an error.
	if err == nil {
		t.Error("TYPES-2 fix: nil value with string type operand: expected error, got nil")
	}
}

// Test_requiredTypeByteCode_Int16Operand_TYPES3 verifies the TYPES-3 fix:
// an int16 operand now correctly dispatches to the Int16Kind case so that an
// int16 stack value passes and a plain int value fails.
//
// Before the fix, the operand was extracted via i.(int), which fails for int16,
// causing the entire dispatch block to be skipped — the check returned no error
// even for mismatched types.
func Test_requiredTypeByteCode_Int16Operand_MatchingValue_TYPES3(t *testing.T) {
	tc := newTestContext(t).withStack(int16(5)).
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, int16(0))

	tc.assertNoError(err)
	tc.assertTopStack(int16(5))
}

// Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3 verifies that a
// plain int fails when an int16 is required — the TYPES-3 fix makes this case
// reachable whereas before it fell through silently.
func Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3(t *testing.T) {
	tc := newTestContext(t).withStack(42). // plain int
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := requiredTypeByteCode(tc.ctx, int16(0)) // requires int16

	tc.assertError(err, errors.ErrArgumentType)
}
