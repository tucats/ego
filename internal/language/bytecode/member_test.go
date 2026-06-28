package bytecode

// member_test.go provides comprehensive unit tests for the member-access
// bytecode instruction handlers defined in member.go.
//
// # What "member access" means in Ego
//
// Ego uses a dot notation for member access just like Go: obj.Field or
// obj.Method().  The memberByteCode instruction implements the load side of
// that: given an object and a field/method name it pushes the value of that
// member onto the stack.
//
// # The stack convention
//
// memberByteCode can receive the member name in one of two ways:
//
//  1. From the instruction operand: memberByteCode(ctx, "FieldName")
//  2. From the stack top:           memberByteCode(ctx, nil)
//
// In case 2 the stack must look like:
//
//	[bottom] ... | object | name [top]
//
// so withStack(object, name) puts `name` on top where it is popped first.
//
// # Known issues documented here
//
// Tests whose names contain a MEMBERS-N suffix document a known bug described
// in docs/bytecode_issues.md under the matching namespace.  Tests ending in
// _CurrentlyBroken assert the present (incorrect) behavior; when the bug is
// fixed both the assertion and the name should be updated.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// ═══════════════════════════════════════════════════════════════════════════════
// memberByteCode — stack mechanics
//
// These tests verify how memberByteCode reads the name and the object from the
// stack (or from the operand), and how it handles underflow and stack markers.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_NameFromOperand verifies that when the instruction operand
// is non-nil it is used as the member name, avoiding a stack pop for the name.
// The object is still popped from the stack.
func Test_memberByteCode_NameFromOperand(t *testing.T) {
	// Build a struct with one field so we have something to look up.
	s := data.NewStructFromMap(map[string]any{"City": "London"})

	// Only the object is on the stack; the name comes from the operand.
	tc := newTestContext(t).withStack(s)
	err := memberByteCode(tc.ctx, "City") // operand provides the name
	tc.assertNoError(err)
	tc.assertTopStack("London")
	tc.assertStackEmpty()
}

// Test_memberByteCode_NameFromStack verifies that when the operand is nil the
// member name is popped from the top of the stack.
func Test_memberByteCode_NameFromStack(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"City": "London"})

	// withStack(s, "City") pushes s first (bottom), then "City" (top).
	// memberByteCode pops "City" first as the name, then s as the object.
	tc := newTestContext(t).withStack(s, "City")
	err := memberByteCode(tc.ctx, nil) // nil operand → use stack top as name
	tc.assertNoError(err)
	tc.assertTopStack("London")
	tc.assertStackEmpty()
}

// Test_memberByteCode_StackUnderflow_EmptyStack verifies that calling
// memberByteCode with an empty stack and no operand returns ErrStackUnderflow
// because the name pop fails immediately.
func Test_memberByteCode_StackUnderflow_EmptyStack(t *testing.T) {
	tc := newTestContext(t) // nothing on the stack
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_memberByteCode_StackUnderflow_OneItem verifies that having exactly one
// item on the stack (the object) and a nil operand causes the object pop to
// succeed but the subsequent object-extraction pop to underflow.
// Wait — with nil operand: pop name first (succeeds → gets the object!),
// then pop the object (stack is empty → underflow).
func Test_memberByteCode_StackUnderflow_NamePopLeavesObjectMissing(t *testing.T) {
	// Only one value on the stack.  memberByteCode pops it as the name
	// (which happens to be an integer), then tries to pop the object
	// and finds the stack empty.
	tc := newTestContext(t).withStack("Name") // only the name; no object below it
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_memberByteCode_StackUnderflow_OperandButEmptyStack verifies that even
// when the name comes from the operand (not the stack), an empty stack for the
// object still returns ErrStackUnderflow.
func Test_memberByteCode_StackUnderflow_OperandButEmptyStack(t *testing.T) {
	tc := newTestContext(t) // empty — nothing to pop as the object
	err := memberByteCode(tc.ctx, "FieldName")
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_memberByteCode_StackMarkerAsName verifies that a StackMarker on top of
// the stack (where the name should be) returns ErrFunctionReturnedVoid.
// StackMarkers are sentinels that mark the boundary between a function's return
// values and other stack contents; they should never be used as member names.
func Test_memberByteCode_StackMarkerAsName(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"X": 1})
	marker := NewStackMarker("results") // sentinel value, not a real name
	// Push object first (bottom), then marker (top — where name is expected).
	tc := newTestContext(t).withStack(s, marker)
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_memberByteCode_StackMarkerAsObject verifies that a StackMarker in the
// object position returns ErrFunctionReturnedVoid.
// Note: the error is returned without c.runtimeError() wrapping (MEMBERS-2).
func Test_memberByteCode_StackMarkerAsObject(t *testing.T) {
	marker := NewStackMarker("results")
	// Push the marker as the object, then a string as the name.
	tc := newTestContext(t).withStack(marker, "field")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// ═══════════════════════════════════════════════════════════════════════════════
// getMemberValue dispatch — *data.Type
//
// When the object on the stack is a *data.Type value (an Ego type descriptor),
// getMemberValue always returns the type's string name — it completely ignores
// the requested member name.  See MEMBERS-6 in docs/bytecode_issues.md.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_TypeObject_UnregisteredName_Error verifies that accessing
// a name that has no registered function on a *data.Type returns ErrUnknownMember.
// Previously broken (MEMBERS-6): the code ignored 'name' and always returned the
// type's string representation.  Now a proper member lookup is performed.
func Test_memberByteCode_TypeObject_UnregisteredName_Error(t *testing.T) {
	// data.StringType has no functions registered under "anything".
	tc := newTestContext(t).withStack(data.StringType, "anything")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrUnknownMember)
}

// Test_memberByteCode_TypeObject_RegisteredFunction_OK verifies that a function
// registered on a *data.Type IS accessible via member access.
// This is the positive case: the lookup now works correctly (MEMBERS-6 fix).
func Test_memberByteCode_TypeObject_RegisteredFunction_OK(t *testing.T) {
	// Build a custom type and register a function called "Format" on it.
	myType := data.StructureType(data.Field{Name: "X", Type: data.IntType})
	decl := &data.Declaration{Name: "Format"}
	myType.DefineFunction("Format", decl, nil)

	// Member access on the type itself should return the Function descriptor.
	tc := newTestContext(t).withStack(myType, "Format")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	// The result is the Function descriptor for "Format".
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("unexpected stack pop error: %v", popErr)
	}

	if _, ok := v.(data.Function); !ok {
		t.Errorf("expected data.Function on stack, got %T: %v", v, v)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// getMemberValue dispatch — *data.Map
//
// Map member access is gated by the context's extensions flag.  Without
// extensions, accessing a map member is an error.  With extensions enabled,
// the value at the given key is returned.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_Map_ExtensionsOff_Error verifies that accessing a map
// member without language extensions returns ErrInvalidTypeForOperation.
// Standard Ego does not allow map.field syntax; it is a language extension.
func Test_memberByteCode_Map_ExtensionsOff_Error(t *testing.T) {
	// Create a simple string→interface map.
	m := data.NewMapFromMap(map[string]any{"color": "blue"})
	tc := newTestContext(t).withStack(m, "color") // extensions off by default
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidTypeForOperation)
}

// Test_memberByteCode_Map_ExtensionsOn_Found verifies that with language
// extensions enabled, a map key is accessible via dot notation.
func Test_memberByteCode_Map_ExtensionsOn_Found(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"color": "blue"})
	tc := newTestContext(t).withStack(m, "color").withExtensions(true)
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("blue")
}

// Test_memberByteCode_Map_ExtensionsOn_MissingKey_PushesNil verifies that
// accessing a key that does not exist in the map pushes nil (not an error).
// The bool "found" return from Map.Get is discarded, so missing keys silently
// produce nil rather than an error.
func Test_memberByteCode_Map_ExtensionsOn_MissingKey_PushesNil(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"color": "blue"})
	tc := newTestContext(t).withStack(m, "missing").withExtensions(true)
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	// A missing key returns nil, not an error — the found bool is discarded.
	tc.assertTopStack(nil)
}

// ═══════════════════════════════════════════════════════════════════════════════
// getMemberValue dispatch — *data.Struct (via getStructMemberValue)
//
// Struct member access supports both direct field lookups and method lookups
// through the struct's associated type definition.  Package visibility rules
// apply when the struct belongs to a different package than the calling code.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_Struct_FieldExists verifies basic field access on an
// anonymous struct (no package, all fields accessible).
func Test_memberByteCode_Struct_FieldExists(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"Name": "Alice", "Age": 30})
	tc := newTestContext(t).withStack(s, "Name")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("Alice")
}

// Test_memberByteCode_Struct_FieldNotFound_Error verifies that requesting a
// field that does not exist on the struct returns ErrUnknownMember.
// Note: the error is returned without c.runtimeError() wrapping (MEMBERS-3).
func Test_memberByteCode_Struct_FieldNotFound_Error(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"Name": "Alice"})
	tc := newTestContext(t).withStack(s, "Missing")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrUnknownMember)
}

// Test_memberByteCode_Struct_MethodFromTypeDef verifies that a method defined
// on the struct's type (not a direct field) is still findable via member access.
// The returned value is a data.Function descriptor, not the function's return value.
func Test_memberByteCode_Struct_MethodFromTypeDef(t *testing.T) {
	// Create a named struct type with a method "Greet" attached.
	t2 := data.StructureType(data.Field{Name: "Name", Type: data.StringType})
	// DefineFunction stores a Function descriptor in the type's functions map.
	// Declaration must be non-nil so getStructMemberValue considers it "found".
	t2.DefineFunction("Greet", &data.Declaration{Name: "Greet"}, nil)

	s := data.NewStruct(t2)
	s.SetAlways("Name", "Alice")

	tc := newTestContext(t).withStack(s, "Greet")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	// The result should be the Function descriptor, not a computed value.
	// We just verify that something non-nil was pushed and it is a data.Function.
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("unexpected stack pop error: %v", popErr)
	}

	if _, ok := v.(data.Function); !ok {
		t.Errorf("expected data.Function on stack, got %T: %v", v, v)
	}
}

// Test_memberByteCode_Struct_ImmutableFieldUnwrapped verifies that when a
// struct field holds a data.Immutable (read-only constant) wrapper, the wrapper
// is removed before the value is pushed onto the stack.
func Test_memberByteCode_Struct_ImmutableFieldUnwrapped(t *testing.T) {
	// data.Constant wraps a value in an Immutable struct to mark it read-only.
	s := data.NewStructFromMap(map[string]any{"Pi": data.Constant(3.14)})
	tc := newTestContext(t).withStack(s, "Pi")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	// The Immutable wrapper is stripped — we get the plain float64, not Immutable{}.
	tc.assertTopStack(3.14)
}

// ─── Struct package visibility ────────────────────────────────────────────────
//
// In Ego (like Go), only exported identifiers (those starting with an uppercase
// letter) are accessible from outside the package in which they were defined.
// The struct's package is tracked in its type definition.

// Test_memberByteCode_Struct_ExportedField_OtherPackage verifies that a field
// with a capitalized name is accessible even when the caller is in a different
// package than the struct's defining package.
func Test_memberByteCode_Struct_ExportedField_OtherPackage(t *testing.T) {
	// Build a struct whose type belongs to "widgets" package.
	t2 := data.StructureType(data.Field{Name: "Name", Type: data.StringType})
	t2.SetPackage("widgets")
	s := data.NewStruct(t2)
	s.SetAlways("Name", "gear")

	// The calling context is in a different package.
	tc := newTestContext(t).withStack(s, "Name")
	tc.ctx.pkg = "main" // different from "widgets"

	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	// "Name" starts with an uppercase letter — export rules allow access.
	tc.assertTopStack("gear")
}

// Test_memberByteCode_Struct_UnexportedField_OtherPackage_Error verifies that
// a lowercase (unexported) field name from a struct defined in a different
// package returns ErrSymbolNotExported.
// Note: the error is returned without c.runtimeError() wrapping (MEMBERS-3).
func Test_memberByteCode_Struct_UnexportedField_OtherPackage_Error(t *testing.T) {
	t2 := data.StructureType(
		data.Field{Name: "name", Type: data.StringType}, // lowercase = unexported
	)
	t2.SetPackage("widgets")
	s := data.NewStruct(t2)
	s.SetAlways("name", "secret")

	tc := newTestContext(t).withStack(s, "name")
	tc.ctx.pkg = "main" // different package

	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrSymbolNotExported)
}

// Test_memberByteCode_Struct_UnexportedField_SamePackage_OK verifies that a
// lowercase field IS accessible when the calling code is in the same package
// that defined the struct.
func Test_memberByteCode_Struct_UnexportedField_SamePackage_OK(t *testing.T) {
	t2 := data.StructureType(
		data.Field{Name: "secret", Type: data.StringType},
	)
	t2.SetPackage("widgets")
	s := data.NewStruct(t2)
	s.SetAlways("secret", "hidden value")

	tc := newTestContext(t).withStack(s, "secret")
	tc.ctx.pkg = "widgets" // SAME package — unexported access is allowed

	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("hidden value")
}

// ═══════════════════════════════════════════════════════════════════════════════
// getMemberValue dispatch — *data.Package (via getPackageMemberValue)
//
// Packages store their exported symbols in two places:
//   1. The items map (directly accessible via pkg.Get(name))
//   2. An embedded symbol table (for capitalized names, checked first)
//
// getPackageMemberValue checks the embedded symbol table first when the name
// starts with an uppercase letter, then falls back to the items map.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_Package_CapitalizedMember_ItemsMap verifies that a
// capitalized package member stored in the package's items map is accessible.
func Test_memberByteCode_Package_CapitalizedMember_ItemsMap(t *testing.T) {
	pkg := data.NewPackage("math", "math")
	// Store a constant in the package's items map.
	pkg.Set("Pi", 3.14159)

	tc := newTestContext(t).withStack(pkg, "Pi")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3.14159)
}

// Test_memberByteCode_Package_CapitalizedMember_SymbolTable verifies that a
// capitalized package member stored in the package's embedded symbol table is
// accessible.  The symbol table is checked before the items map for capitalized
// names.
func Test_memberByteCode_Package_CapitalizedMember_SymbolTable(t *testing.T) {
	pkg := data.NewPackage("math", "math")

	// GetPackageSymbolTable lazily creates and attaches a symbol table to the
	// package if one does not already exist, then returns it.
	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("MaxInt", 9007199254740992) // stored in the symbol table

	tc := newTestContext(t).withStack(pkg, "MaxInt")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(9007199254740992)
}

// Test_memberByteCode_Package_LowercaseMember_ItemsMap verifies that a
// lowercase member stored in the package's items map is accessible.
// The symbol table is not checked for lowercase names — they go straight to
// the items map.
func Test_memberByteCode_Package_LowercaseMember_ItemsMap(t *testing.T) {
	pkg := data.NewPackage("internal", "internal")
	pkg.Set("version", "1.2.3") // lowercase — won't be in symbol table check
	tc := newTestContext(t).withStack(pkg, "version")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("1.2.3")
}

// Test_memberByteCode_Package_MemberNotFound_Error verifies that requesting a
// member that does not exist in either the symbol table or the items map returns
// ErrUnknownPackageMember.
func Test_memberByteCode_Package_MemberNotFound_Error(t *testing.T) {
	pkg := data.NewPackage("math", "math")
	// No items set — any name lookup will fail.
	tc := newTestContext(t).withStack(pkg, "Imaginary")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrUnknownPackageMember)
}

// Test_memberByteCode_Package_ConstantMemberUnwrapped verifies that a package
// member stored as a data.Constant (Immutable wrapper) is unwrapped before
// being pushed onto the stack.
func Test_memberByteCode_Package_ConstantMemberUnwrapped(t *testing.T) {
	pkg := data.NewPackage("math", "math")
	// data.Constant wraps the value in an Immutable{} struct.
	pkg.Set("E", data.Constant(2.71828))

	tc := newTestContext(t).withStack(pkg, "E")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	// The Immutable wrapper should be stripped — we get the plain float64.
	tc.assertTopStack(2.71828)
}

// ═══════════════════════════════════════════════════════════════════════════════
// getMemberValue dispatch — *any indirection
//
// When the object on the stack is a *any (a pointer to an interface value),
// getMemberValue dereferences it and then dispatches based on the underlying
// type.  This handles the case where a struct or type is stored behind an extra
// level of indirection.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_PtrAny_PointingToStruct verifies that a *any holding a
// *data.Struct is correctly dereferenced and the struct field is returned.
func Test_memberByteCode_PtrAny_PointingToStruct(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"Score": 100})
	// Create a *any by taking the address of an interface variable.
	var iface any = s
	ptr := &iface // *any pointing at the struct

	tc := newTestContext(t).withStack(ptr, "Score")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(100)
}

// Test_memberByteCode_PtrAny_PointingToType_UnregisteredName verifies that a
// *any holding a *data.Type returns ErrUnknownMember when the name has no
// registered function on the underlying base type.
// Previously (MEMBERS-6), this path returned the type's string "string".
// After the fix the recursive call into getMemberValue hits the *data.Type
// branch, checks t.Function("anything") (nil), and returns ErrUnknownMember.
func Test_memberByteCode_PtrAny_PointingToType_UnregisteredName(t *testing.T) {
	// TypeDefinition wraps StringType.  BaseType() returns StringType.
	// StringType has no function "anything" registered on it.
	namedType := data.TypeDefinition("MyString", data.StringType)

	var iface any = namedType
	ptr := &iface

	tc := newTestContext(t).withStack(ptr, "anything")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrUnknownMember)
}

// Test_memberByteCode_PtrAny_PointingToNilType verifies that a *any holding a
// nil *data.Type now returns ErrInvalidType instead of silently pushing nil.
// Previously broken (MEMBERS-7): BaseType() returned nil for a nil receiver,
// the inner switch had no else-branch, and getMemberValue fell through to
// return (nil, nil), causing memberByteCode to push nil with no error.
func Test_memberByteCode_PtrAny_PointingToNilType(t *testing.T) {
	var nilType *data.Type = nil // nil typed pointer — uncommon but valid in Go

	var iface any = nilType

	ptr := &iface // *any holding a nil *data.Type

	tc := newTestContext(t).withStack(ptr, "anything")
	err := memberByteCode(tc.ctx, nil)

	// MEMBERS-7 fix: an explicit error is now returned for a nil type receiver.
	tc.assertError(err, errors.ErrInvalidType)
}

// ═══════════════════════════════════════════════════════════════════════════════
// getNativePackageMemberValue — native Go types
//
// When the object type does not match any of the known Ego types (struct, map,
// package, *any), getMemberValue falls through to getNativePackageMemberValue,
// which uses reflection to look for a method and, failing that, checks if the
// type has a registered Ego function with that name.  For simple scalar types
// with no registered functions, it returns ErrInvalidTypeForOperation.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_NativeScalar_Error verifies that accessing a member on a
// plain Go int value (which is not an Ego object type) returns
// ErrInvalidTypeForOperation because ints have no methods or registered Ego
// functions, and are below the MaximumScalarType threshold.
func Test_memberByteCode_NativeScalar_Error(t *testing.T) {
	// A plain Go int hits the default case in getMemberValue → calls
	// getNativePackageMemberValue.  Since int has no methods and its kind
	// is below MaximumScalarType, ErrInvalidTypeForOperation is returned.
	tc := newTestContext(t).withStack(42, "method")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidTypeForOperation)
}

// Test_memberByteCode_NativeString_Error verifies the same behavior for a plain
// Go string value — accessing any member name returns ErrInvalidTypeForOperation.
func Test_memberByteCode_NativeString_Error(t *testing.T) {
	tc := newTestContext(t).withStack("hello world", "Length")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidTypeForOperation)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Edge cases and regression tests
// ═══════════════════════════════════════════════════════════════════════════════

// Test_memberByteCode_Struct_EmptyName verifies that an empty string as the
// member name triggers ErrUnknownMember from getStructMemberValue, since no
// struct field will ever have an empty name.
func Test_memberByteCode_Struct_EmptyName(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"Name": "Alice"})
	tc := newTestContext(t).withStack(s, "")
	err := memberByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrUnknownMember)
}

// Test_memberByteCode_Struct_IntegerName verifies that a non-string operand is
// converted to string via data.String() when used as the member name.
// data.String(42) returns "42", which is unlikely to match any struct field.
func Test_memberByteCode_Struct_IntegerOperand_AsName(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"Name": "Alice"})
	// Passing int 42 as operand: memberByteCode calls data.String(42) = "42"
	// which won't match any field, so ErrUnknownMember is expected.
	tc := newTestContext(t).withStack(s)
	err := memberByteCode(tc.ctx, 42) // int operand → data.String(42) = "42"
	tc.assertError(err, errors.ErrUnknownMember)
}

// Test_memberByteCode_Package_ImmutableInSymbolTable verifies that a
// data.Constant value stored in the package's embedded symbol table is unwrapped
// before being returned, matching the behavior documented for the items-map path.
func Test_memberByteCode_Package_ImmutableInSymbolTable(t *testing.T) {
	pkg := data.NewPackage("constants", "constants")
	syms := symbols.GetPackageSymbolTable(pkg)
	// Store a wrapped (read-only) constant in the symbol table.
	syms.SetAlways("Tau", data.Constant(6.28318))

	tc := newTestContext(t).withStack(pkg, "Tau")
	err := memberByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	// Immutable wrapper must be stripped by data.UnwrapConstant.
	tc.assertTopStack(6.28318)
}
