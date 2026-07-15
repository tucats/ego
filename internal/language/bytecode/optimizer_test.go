package bytecode

// optimizer_test.go — comprehensive unit tests for the peephole optimizer.
//
// # What is tested
//
// The tests are organized into sections:
//
//  1. operandEqual        — the type-switch equality helper (OPTIMIZER-2 regression guard)
//  2. tryConstantArithmetic — fast constant-fold path (OPTIMIZER-7)
//  3. Patch               — instruction splice with branch-target adjustment (OPTIMIZER-5)
//  4. Individual rules    — every optimization rule in optimizations.go, one or more
//                          tests each, verifying that the rule fires on matching bytecode
//                          and does NOT fire on non-matching bytecode
//  5. Concrete-operand    — regression tests for the bug where a concrete operand
//     mismatch            mis-match (e.g. "Store err" vs. pattern "Store _") caused
//                          the wrong rule to fire (OPTIMIZER-2 bug fix)
//  6. Branch safety       — optimizer must not collapse a pattern window that contains
//                          a branch target
//  7. Branch adjustment   — Patch must rewrite branch operands after shrinking bytecode
//  8. Backtracking        — after a match, the scanner re-examines earlier positions
//                          so cascading opportunities are not missed
//  9. Seal integration    — Seal() calls optimize() when the setting is 2 ("always")
//
// # Testing approach
//
// Each test builds a small ByteCode manually (using Emit), calls optimize(0) directly
// (it is unexported but accessible within the same package), and inspects the resulting
// instruction slice via b.instructions[:b.nextAddress].
//
// Tests that need the Branch-safety or Branch-adjustment features use real Branch
// operands (integer addresses), which are valid because Emit does not validate
// destinations at emit time.

import (
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// mustOptimize calls optimize(0) and fails the test if it returns an error.
func mustOptimize(t *testing.T, b *ByteCode) int {
	t.Helper()

	count, err := b.optimize(0)
	if err != nil {
		t.Fatalf("optimize returned unexpected error: %v", err)
	}

	return count
}

// instrAt returns the instruction at address addr, or fails the test if addr
// is out of range.
func instrAt(t *testing.T, b *ByteCode, addr int) instruction {
	t.Helper()

	if addr < 0 || addr >= b.nextAddress {
		t.Fatalf("address %d out of range [0, %d)", addr, b.nextAddress)
	}

	return b.instructions[addr]
}

// assertSize fails the test if the bytecode does not contain exactly n
// instructions.
func assertSize(t *testing.T, b *ByteCode, n int) {
	t.Helper()

	if b.nextAddress != n {
		t.Fatalf("expected %d instructions, got %d", n, b.nextAddress)
	}
}

// assertInstr fails the test if the instruction at addr does not match op
// and operand.
func assertInstr(t *testing.T, b *ByteCode, addr int, op Opcode, operand any) {
	t.Helper()

	i := instrAt(t, b, addr)
	if i.Operation != op {
		t.Errorf("addr %d: opcode = %v, want %v", addr, i.Operation, op)
	}

	if !operandEqual(i.Operand, operand) {
		t.Errorf("addr %d: operand = %#v, want %#v", addr, i.Operand, operand)
	}
}

// ── Section 1: operandEqual ──────────────────────────────────────────────────

// Test_operandEqual_Int verifies integer fast-path.
func Test_operandEqual_Int(t *testing.T) {
	if !operandEqual(42, 42) {
		t.Error("equal ints should return true")
	}

	if operandEqual(42, 43) {
		t.Error("unequal ints should return false")
	}

	if operandEqual(42, int64(42)) {
		t.Error("int vs int64 should return false (different types)")
	}
}

// Test_operandEqual_Int64 verifies int64 fast-path.
func Test_operandEqual_Int64(t *testing.T) {
	if !operandEqual(int64(100), int64(100)) {
		t.Error("equal int64s should return true")
	}

	if operandEqual(int64(100), int64(101)) {
		t.Error("unequal int64s should return false")
	}
}

// Test_operandEqual_Float64 verifies float64 fast-path.
func Test_operandEqual_Float64(t *testing.T) {
	if !operandEqual(3.14, 3.14) {
		t.Error("equal float64s should return true")
	}

	if operandEqual(3.14, 3.15) {
		t.Error("unequal float64s should return false")
	}
}

// Test_operandEqual_Bool verifies bool fast-path.
func Test_operandEqual_Bool(t *testing.T) {
	if !operandEqual(true, true) {
		t.Error("true == true should return true")
	}

	if operandEqual(true, false) {
		t.Error("true == false should return false")
	}
}

// Test_operandEqual_String verifies string fast-path.
func Test_operandEqual_String(t *testing.T) {
	if !operandEqual("hello", "hello") {
		t.Error("equal strings should return true")
	}

	if operandEqual("hello", "world") {
		t.Error("unequal strings should return false")
	}

	if operandEqual("_", "err") {
		t.Error("\"_\" vs \"err\" should return false — this is the regression guard")
	}
}

// Test_operandEqual_Nil verifies nil fast-path.
func Test_operandEqual_Nil(t *testing.T) {
	if !operandEqual(nil, nil) {
		t.Error("nil == nil should return true")
	}

	if operandEqual(nil, 0) {
		t.Error("nil vs 0 should return false")
	}
}

// Test_operandEqual_StackMarker verifies that two StackMarkers with the same
// label compare equal and two with different labels compare unequal.
// StackMarker is a struct with an unexported []any field, so it uses the
// reflect.DeepEqual fallback path.
func Test_operandEqual_StackMarker(t *testing.T) {
	a := NewStackMarker("let")
	b := NewStackMarker("let")
	c := NewStackMarker("other")

	if !operandEqual(a, b) {
		t.Error("same-label StackMarkers should be equal")
	}

	if operandEqual(a, c) {
		t.Error("different-label StackMarkers should not be equal")
	}
}

// ── Section 2: tryConstantArithmetic ────────────────────────────────────────

// Test_tryConstantArithmetic_Int verifies int add/sub/mul.
func Test_tryConstantArithmetic_Int(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, 3, 4)
	if !ok || v != 7 {
		t.Errorf("int add: got (%v, %v), want (7, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Sub, 10, 3)
	if !ok || v != 7 {
		t.Errorf("int sub: got (%v, %v), want (7, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Mul, 3, 4)
	if !ok || v != 12 {
		t.Errorf("int mul: got (%v, %v), want (12, true)", v, ok)
	}
}

// Test_tryConstantArithmetic_Int64 verifies int64 operations.
func Test_tryConstantArithmetic_Int64(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, int64(5), int64(6))
	if !ok || v != int64(11) {
		t.Errorf("int64 add: got (%v, %v), want (11, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Mul, int64(7), int64(8))
	if !ok || v != int64(56) {
		t.Errorf("int64 mul: got (%v, %v), want (56, true)", v, ok)
	}
}

// Test_tryConstantArithmetic_Float64 verifies float64 operations.
func Test_tryConstantArithmetic_Float64(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, 1.5, 2.5)
	if !ok || v != 4.0 {
		t.Errorf("float64 add: got (%v, %v), want (4.0, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Sub, 5.0, 2.0)
	if !ok || v != 3.0 {
		t.Errorf("float64 sub: got (%v, %v), want (3.0, true)", v, ok)
	}
}

// Test_tryConstantArithmetic_Complex128 verifies complex128 add/sub/mul fold,
// but Div is deliberately left unfolded (falls through to executeFragment,
// which runs the real divideByteCode and its zero-check).
func Test_tryConstantArithmetic_Complex128(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, complex128(1+2i), complex128(3-1i))
	if !ok || v != complex128(4+1i) {
		t.Errorf("complex128 add: got (%v, %v), want (4+1i, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Sub, complex128(4+1i), complex128(3-1i))
	if !ok || v != complex128(1+2i) {
		t.Errorf("complex128 sub: got (%v, %v), want (1+2i, true)", v, ok)
	}

	v, ok = tryConstantArithmetic(Mul, complex128(1+2i), complex128(3-1i))
	if !ok || v != complex128(5+5i) {
		t.Errorf("complex128 mul: got (%v, %v), want (5+5i, true)", v, ok)
	}

	_, ok = tryConstantArithmetic(Div, complex128(4+1i), complex128(3-1i))
	if ok {
		t.Error("complex128 div should return (nil, false) -- not folded here")
	}
}

// Test_tryConstantArithmetic_Complex64 verifies complex64 add/sub/mul fold.
func Test_tryConstantArithmetic_Complex64(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, complex64(1+2i), complex64(3-1i))
	if !ok || v != complex64(4+1i) {
		t.Errorf("complex64 add: got (%v, %v), want (4+1i, true)", v, ok)
	}
}

// Test_tryConstantArithmetic_StringConcat verifies string concatenation.
func Test_tryConstantArithmetic_StringConcat(t *testing.T) {
	v, ok := tryConstantArithmetic(Add, "foo", "bar")
	if !ok || v != "foobar" {
		t.Errorf("string concat: got (%v, %v), want (foobar, true)", v, ok)
	}
	// Sub and Mul on strings should return false (not handled).
	_, ok = tryConstantArithmetic(Sub, "foo", "bar")
	if ok {
		t.Error("string sub should return (nil, false)")
	}
}

// Test_tryConstantArithmetic_MixedTypes verifies that mismatched types
// return (nil, false) without panicking.
func Test_tryConstantArithmetic_MixedTypes(t *testing.T) {
	_, ok := tryConstantArithmetic(Add, 1, "two")
	if ok {
		t.Error("int + string should return (nil, false)")
	}

	_, ok = tryConstantArithmetic(Add, "one", 2)
	if ok {
		t.Error("string + int should return (nil, false)")
	}
}

// ── Section 3: Patch ─────────────────────────────────────────────────────────

// Test_Patch_ShrinkNoTargets verifies that Patch correctly removes instructions
// and adjusts no branch targets when none exist.
func Test_Patch_ShrinkNoTargets(t *testing.T) {
	b := New("patch test")
	b.Emit(Push, "a") // 0
	b.Emit(Push, "b") // 1
	b.Emit(Add)       // 2
	b.Emit(Push, "c") // 3
	b.instructions = b.instructions[:b.nextAddress]

	// Replace instructions 1-2 (Push "b" + Add) with a single Push "bc".
	b.Patch(1, 2, []instruction{{Operation: Push, Operand: "bc"}})

	assertSize(t, b, 3)
	assertInstr(t, b, 0, Push, "a")
	assertInstr(t, b, 1, Push, "bc")
	assertInstr(t, b, 2, Push, "c")
}

// Test_Patch_BranchAdjusted verifies that a Branch target after the patch
// region is decremented by the number of removed instructions.
func Test_Patch_BranchAdjusted(t *testing.T) {
	b := New("patch branch")
	b.Emit(Push, "a") // 0
	b.Emit(Push, "b") // 1
	b.Emit(Add)       // 2
	b.Emit(Branch, 5) // 3 — targets addr 5 (after the stream)
	b.Emit(Push, "d") // 4
	b.instructions = b.instructions[:b.nextAddress]

	// Remove instructions 1 and 2 (Push "b" + Add) — 2 deleted, 0 inserted.
	b.Patch(1, 2, nil)

	// Branch was at addr 3, now at addr 1. Its target was 5, which was > start(1),
	// so it decrements by offset=2: new target = 3.
	assertSize(t, b, 3)
	assertInstr(t, b, 1, Branch, 3)
}

// Test_Patch_BranchBeforePatch verifies that a Branch target BEFORE the patch
// region is left unchanged.
func Test_Patch_BranchBeforePatch(t *testing.T) {
	b := New("patch branch before")
	b.Emit(Branch, 0) // 0 — targets addr 0 (before patch region)
	b.Emit(Push, "x") // 1
	b.Emit(Push, "y") // 2
	b.Emit(Add)       // 3
	b.instructions = b.instructions[:b.nextAddress]

	// Remove instructions 1 and 2.
	b.Patch(1, 2, nil)

	// Branch targets addr 0, which is <= start(1), so it is not adjusted.
	assertInstr(t, b, 0, Branch, 0)
}

// Test_Patch_GrowingReplacement verifies that Patch is safe when the
// replacement is larger than the deleted region (OPTIMIZER-5 regression guard).
// Without the tail-copy fix, this would corrupt instructions after the patch.
func Test_Patch_GrowingReplacement(t *testing.T) {
	b := New("patch grow")
	b.Emit(Push, "a") // 0
	b.Emit(Push, "b") // 1  ← we will replace just this one with TWO instructions
	b.Emit(Push, "c") // 2
	b.instructions = b.instructions[:b.nextAddress]

	// Replace 1 instruction at addr 1 with 2 instructions.
	b.Patch(1, 1, []instruction{
		{Operation: Push, Operand: "X"},
		{Operation: Push, Operand: "Y"},
	})

	// Result should be: Push "a", Push "X", Push "Y", Push "c".
	assertSize(t, b, 4)
	assertInstr(t, b, 0, Push, "a")
	assertInstr(t, b, 1, Push, "X")
	assertInstr(t, b, 2, Push, "Y")
	assertInstr(t, b, 3, Push, "c")
}

// ── Section 4a: Assignment-elimination rules ──────────────────────────────────

// Test_Optimize_AssignmentOptimizedAway verifies that Push(Marker) followed by
// DropToMarker with the same marker is removed entirely.
func Test_Optimize_AssignmentOptimizedAway(t *testing.T) {
	b := New("assign away")
	b.Emit(Push, NewStackMarker("let"))
	b.Emit(DropToMarker, NewStackMarker("let"))
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count == 0 {
		t.Error("expected at least one optimization")
	}

	assertSize(t, b, 0)
}

// Test_Optimize_WriteConstantToNullVar verifies that Push(X) + Drop(1) → deleted.
func Test_Optimize_WriteConstantToNullVar(t *testing.T) {
	b := New("write const null")
	b.Emit(Push, 42)
	b.Emit(Drop, 1)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 0)
}

// ── Section 4b: Blank-identifier rules ──────────────────────────────────────

// Test_Optimize_StoreToNullVar verifies Store("_") → Drop.
func Test_Optimize_StoreToNullVar(t *testing.T) {
	b := New("store null")
	b.Emit(Store, "_")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Drop, nil)
}

// Test_Optimize_StoreToNullVar_NoFire_OtherName is the regression guard:
// Store("err") must NOT be replaced with Drop.
func Test_Optimize_StoreToNullVar_NoFire_OtherName(t *testing.T) {
	b := New("store not null")
	b.Emit(Store, "err")
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	// The rule should NOT fire.
	if count != 0 {
		t.Error("Store to null var rule fired on Store \"err\" — regression")
	}

	assertSize(t, b, 1)
	assertInstr(t, b, 0, Store, "err")
}

// Test_Optimize_CreateNullVar verifies SymbolOptCreate("_") → deleted.
func Test_Optimize_CreateNullVar(t *testing.T) {
	b := New("create null")
	b.Emit(SymbolOptCreate, "_")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 0)
}

// Test_Optimize_CreateNullVar_NoFire_OtherName is the regression guard:
// SymbolOptCreate("err") must NOT be removed.
func Test_Optimize_CreateNullVar_NoFire_OtherName(t *testing.T) {
	b := New("create not null")
	b.Emit(SymbolOptCreate, "err")
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count != 0 {
		t.Error("Create null var rule fired on SymbolOptCreate \"err\" — regression")
	}

	assertSize(t, b, 1)
	assertInstr(t, b, 0, SymbolOptCreate, "err")
}

// ── Section 4c: Increment fusion ─────────────────────────────────────────────

// Test_Optimize_ConstantIncrement verifies Load(x)+Push(n)+Add+Store(x) →
// Increment([x, n]).
func Test_Optimize_ConstantIncrement(t *testing.T) {
	b := New("const inc")
	b.Emit(Load, "i")
	b.Emit(Push, 1)
	b.Emit(Add)
	b.Emit(Store, "i")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != Increment {
		t.Fatalf("expected Increment, got %v", i.Operation)
	}

	arr, ok := i.Operand.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("Increment operand should be []any{name, step}, got %#v", i.Operand)
	}

	if arr[0] != "i" || arr[1] != 1 {
		t.Errorf("Increment operand = %v, want [i 1]", arr)
	}
}

// Test_Optimize_ConstantIncrement_DifferentNames verifies the rule does NOT
// fire when the Load and Store use different variable names.
func Test_Optimize_ConstantIncrement_DifferentNames(t *testing.T) {
	b := New("const inc mismatch")
	b.Emit(Load, "i")
	b.Emit(Push, 1)
	b.Emit(Add)
	b.Emit(Store, "j") // different name
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count != 0 {
		t.Error("constant increment rule fired despite Load/Store name mismatch")
	}

	assertSize(t, b, 4)
}

// ── Section 4d: Comparison with constant ─────────────────────────────────────

// Test_Optimize_LessThanConstant verifies Push(v)+LessThan() →
// LessThan([v]).
func Test_Optimize_LessThanConstant(t *testing.T) {
	b := New("lt const")
	b.Emit(Push, 10)
	b.Emit(LessThan) // nil operand
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != LessThan {
		t.Fatalf("expected LessThan, got %v", i.Operation)
	}

	arr, ok := i.Operand.([]any)
	if !ok || len(arr) != 1 || arr[0] != 10 {
		t.Errorf("LessThan operand = %#v, want []any{10}", i.Operand)
	}
}

// Test_Optimize_GreaterThanConstant verifies the GreaterThan variant.
func Test_Optimize_GreaterThanConstant(t *testing.T) {
	b := New("gt const")
	b.Emit(Push, 5)
	b.Emit(GreaterThan)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, GreaterThan, []any{5})
}

// Test_Optimize_EqualConstant verifies the Equal variant.
func Test_Optimize_EqualConstant(t *testing.T) {
	b := New("eq const")
	b.Emit(Push, 0)
	b.Emit(Equal)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Equal, []any{0})
}

// Test_Optimize_NotEqualConstant verifies the NotEqual variant.
func Test_Optimize_NotEqualConstant(t *testing.T) {
	b := New("ne const")
	b.Emit(Push, 0)
	b.Emit(NotEqual)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, NotEqual, []any{0})
}

// ── Section 4e: AtLine deduplication ─────────────────────────────────────────

// Test_Optimize_SequentialAtLine verifies that two consecutive AtLine
// instructions are collapsed to keep only the second (later) one.
func Test_Optimize_SequentialAtLine(t *testing.T) {
	b := New("atline seq")
	b.Emit(AtLine, 10)
	b.Emit(AtLine, 11)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, AtLine, 11)
}

// Test_Optimize_SequentialAtLine_Three verifies that three consecutive AtLine
// instructions collapse to one (the optimizer makes two passes on backtrack).
func Test_Optimize_SequentialAtLine_Three(t *testing.T) {
	b := New("atline three")
	b.Emit(AtLine, 1)
	b.Emit(AtLine, 2)
	b.Emit(AtLine, 3)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, AtLine, 3)
}

// ── Section 4f: Receiver loading ─────────────────────────────────────────────

// Test_Optimize_LoadSetThis verifies Load(x)+SetThis() → LoadThis(x).
func Test_Optimize_LoadSetThis(t *testing.T) {
	b := New("load this")
	b.Emit(Load, "obj")
	b.Emit(SetThis)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, LoadThis, "obj")
}

// ── Section 4g: Store fusion ──────────────────────────────────────────────────

// Test_Optimize_CollapsePushAndCreateAndStore verifies
// Push(v)+CreateAndStore(n) → CreateAndStore([n,v]).
func Test_Optimize_CollapsePushAndCreateAndStore(t *testing.T) {
	b := New("push createandstore")
	b.Emit(Push, "hello")
	b.Emit(CreateAndStore, "x")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != CreateAndStore {
		t.Fatalf("expected CreateAndStore, got %v", i.Operation)
	}

	arr, ok := i.Operand.([]any)
	if !ok || len(arr) != 2 || arr[0] != "x" || arr[1] != "hello" {
		t.Errorf("CreateAndStore operand = %#v, want [x hello]", i.Operand)
	}
}

// Test_Optimize_CollapsePushAndCreateAndStore_DoesNotEatStackMarker is a
// regression test for a real bug found while investigating the optional
// operator ("?expr : fallback") failing under optimizer level 2: the
// compiler emits "Push Marker<try>; <expr>; CreateAndStore <tempName>"
// (see Compiler.optional in expr_atom.go). When <expr> itself folds down to
// a single constant Push (e.g. "Push 100; Push 5; Div" -> "Push 20" via the
// constant-division-fold rule), "Collapse constant Push and CreateAndStore"
// correctly fires once on (Push 20, CreateAndStore "$N"). The scanner then
// backs up and, without the MustBeString/ExcludeStackMarker guards added
// alongside this test, could match AGAIN on (Push Marker<try>, CreateAndStore
// ["$N", 20]) — treating the marker as "the value" and the already-folded
// [name, value] pair as "the name", corrupting the temp variable's name and
// silently deleting the marker push a later DropToMarker/TryPop still
// expects on the runtime stack.
func Test_Optimize_CollapsePushAndCreateAndStore_DoesNotEatStackMarker(t *testing.T) {
	b := New("marker adjacent to foldable createandstore")
	b.Emit(Push, NewStackMarker("try"))
	b.Emit(Push, 100)
	b.Emit(Push, 5)
	b.Emit(Div)
	b.Emit(CreateAndStore, "$1")
	b.Emit(DropToMarker, NewStackMarker("try"))
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)

	// The division fold and the constant-push collapse should each fire
	// once, but the marker push must survive intact and CreateAndStore's
	// operand must remain a clean [name, value] pair - never absorbing the
	// marker as a third element or as the "value".
	if b.nextAddress != 3 {
		t.Fatalf("expected 3 instructions (Push marker, CreateAndStore, DropToMarker), got %d: %#v",
			b.nextAddress, b.instructions[:b.nextAddress])
	}

	marker := instrAt(t, b, 0)
	if marker.Operation != Push {
		t.Fatalf("expected instruction 0 to be Push, got %v", marker.Operation)
	}

	sm, ok := marker.Operand.(StackMarker)
	if !ok || sm.label != "try" {
		t.Fatalf("expected instruction 0 to push StackMarker(try), got %#v", marker.Operand)
	}

	create := instrAt(t, b, 1)
	if create.Operation != CreateAndStore {
		t.Fatalf("expected instruction 1 to be CreateAndStore, got %v", create.Operation)
	}

	arr, ok := create.Operand.([]any)
	if !ok || len(arr) != 2 || arr[0] != "$1" || arr[1] != 20 {
		t.Errorf("CreateAndStore operand = %#v, want [$1 20]", create.Operand)
	}

	drop := instrAt(t, b, 2)
	if drop.Operation != DropToMarker {
		t.Fatalf("expected instruction 2 to be DropToMarker, got %v", drop.Operation)
	}
}

// Test_Optimize_UnnecessaryLetMarker verifies the 4-instruction
// Push(let)+Push(const)+CreateAndStore(name)+DropToMarker(let) →
// Push(const)+CreateAndStore(name).
func Test_Optimize_UnnecessaryLetMarker(t *testing.T) {
	b := New("let marker")
	b.Emit(Push, NewStackMarker("let"))
	b.Emit(Push, 99)
	b.Emit(CreateAndStore, "y")
	b.Emit(DropToMarker, NewStackMarker("let"))
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	// After marker elimination: Push(99), CreateAndStore("y").
	// Then "Collapse constant Push and CreateAndStore" fires on those two,
	// producing a single CreateAndStore([y, 99]).
	// So the final result should be 1 instruction.
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != CreateAndStore {
		t.Fatalf("expected CreateAndStore, got %v", i.Operation)
	}
}

// ── Section 4h: Scope management ─────────────────────────────────────────────

// Test_Optimize_SequentialPopScope verifies two consecutive PopScope
// instructions are merged with their counts summed.
func Test_Optimize_SequentialPopScope(t *testing.T) {
	b := New("popscope seq")
	b.Emit(PopScope, 1)
	b.Emit(PopScope, 2)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != PopScope {
		t.Fatalf("expected PopScope, got %v", i.Operation)
	}

	if i.Operand != 3 {
		t.Errorf("merged PopScope operand = %v, want 3", i.Operand)
	}
}

// Test_Optimize_SequentialPopScope_NilOperands verifies that nil operands
// each default to 1, so two nil-operand PopScopes merge to PopScope(2).
func Test_Optimize_SequentialPopScope_NilOperands(t *testing.T) {
	b := New("popscope nil")
	b.Emit(PopScope)
	b.Emit(PopScope)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operand != 2 {
		t.Errorf("merged nil PopScope operand = %v, want 2", i.Operand)
	}
}

// Test_Optimize_CreateAndStore_Rule verifies SymbolCreate(n)+Store(n) →
// CreateAndStore(n).
func Test_Optimize_CreateAndStore_Rule(t *testing.T) {
	b := New("create store")
	b.Emit(SymbolCreate, "z")
	b.Emit(Store, "z")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, CreateAndStore, "z")
}

// Test_Optimize_CreateAndStore_Rule_DifferentNames verifies the rule does NOT
// fire when the names differ.
func Test_Optimize_CreateAndStore_Rule_DifferentNames(t *testing.T) {
	b := New("create store mismatch")
	b.Emit(SymbolCreate, "a")
	b.Emit(Store, "b")
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count != 0 {
		t.Error("Create-and-store rule fired despite name mismatch")
	}

	assertSize(t, b, 2)
}

// ── Section 4i: StoreIndex / StoreAlways ─────────────────────────────────────

// Test_Optimize_PushAndStoreIndex verifies Push(v)+StoreIndex() →
// StoreIndex(v).
func Test_Optimize_PushAndStoreIndex(t *testing.T) {
	b := New("push storeindex")
	b.Emit(Push, "val")
	b.Emit(StoreIndex)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, StoreIndex, "val")
}

// Test_Optimize_ConstantStoreAlways verifies Push(v)+StoreAlways(n) →
// StoreAlways([n, v]).
func Test_Optimize_ConstantStoreAlways(t *testing.T) {
	b := New("const storealways")
	b.Emit(Push, true)
	b.Emit(StoreAlways, "flag")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != StoreAlways {
		t.Fatalf("expected StoreAlways, got %v", i.Operation)
	}

	arr, ok := i.Operand.([]any)
	if !ok || len(arr) != 2 || arr[0] != "flag" || arr[1] != true {
		t.Errorf("StoreAlways operand = %#v, want [flag true]", i.Operand)
	}
}

// ── Section 4j: Constant folding ─────────────────────────────────────────────

// Test_Optimize_ConstantAdditionFold verifies Push(2)+Push(3)+Add →
// Push(5).
func Test_Optimize_ConstantAdditionFold(t *testing.T) {
	b := New("const add fold")
	b.Emit(Push, 2)
	b.Emit(Push, 3)
	b.Emit(Add)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Push, 5)
}

// Test_Optimize_ConstantSubtractionFold verifies Push(10)+Push(4)+Sub →
// Push(6).
func Test_Optimize_ConstantSubtractionFold(t *testing.T) {
	b := New("const sub fold")
	b.Emit(Push, 10)
	b.Emit(Push, 4)
	b.Emit(Sub)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Push, 6)
}

// Test_Optimize_ConstantMultiplicationFold verifies Push(6)+Push(7)+Mul →
// Push(42).
func Test_Optimize_ConstantMultiplicationFold(t *testing.T) {
	b := New("const mul fold")
	b.Emit(Push, 6)
	b.Emit(Push, 7)
	b.Emit(Mul)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Push, 42)
}

// Test_Optimize_ConstantFold_Float64 verifies folding works for float64.
func Test_Optimize_ConstantFold_Float64(t *testing.T) {
	b := New("const fold float")
	b.Emit(Push, 1.5)
	b.Emit(Push, 2.5)
	b.Emit(Add)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Push, 4.0)
}

// Test_Optimize_ConstantFold_StringConcat verifies string concatenation
// is folded (via the executeFragment fallback path, since tryConstantArithmetic
// handles strings directly).
func Test_Optimize_ConstantFold_StringConcat(t *testing.T) {
	b := New("const fold string")
	b.Emit(Push, "hello")
	b.Emit(Push, " world")
	b.Emit(Add)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	assertSize(t, b, 1)
	assertInstr(t, b, 0, Push, "hello world")
}

// Test_Optimize_ConstantFold_NoFold_LoadInStream verifies that a Load
// instruction between the two Pushes prevents constant folding
// (the pattern requires two consecutive Pushes).
func Test_Optimize_ConstantFold_NoFold_LoadInStream(t *testing.T) {
	b := New("no fold load")
	b.Emit(Push, 2)
	b.Emit(Load, "x")
	b.Emit(Add)
	b.instructions = b.instructions[:b.nextAddress]

	// The constant-fold rules start with Push; the second instruction is Load
	// (not Push), so the pattern fails the opcode check for the second slot.
	count := mustOptimize(t, b)
	if count != 0 {
		t.Error("constant fold should not fire when Load is in the stream")
	}

	assertSize(t, b, 3)
}

// ── Section 5: Concrete-operand mismatch regression ─────────────────────────
//
// These tests guard against the OPTIMIZER-2 regression where removing the old
// string fast-path caused concrete operand mismatches to leave `found = true`,
// making rules fire on instructions with different operand values.

// Test_Regression_StoreDoesNotFireOnWrongName verifies that "Store to null var"
// does NOT fire on Store instructions with names other than "_".
func Test_Regression_StoreDoesNotFireOnWrongName(t *testing.T) {
	names := []string{"gotError", "result", "err", "x", "myVar"}
	for _, name := range names {
		b := New("store regression " + name)

		b.Emit(Store, name)
		b.instructions = b.instructions[:b.nextAddress]

		count := mustOptimize(t, b)
		if count != 0 {
			t.Errorf("Store to null var fired on Store %q — regression", name)
		}

		if b.nextAddress != 1 || b.instructions[0].Operation != Store {
			t.Errorf("Store %q was modified by optimizer", name)
		}
	}
}

// Test_Regression_SymbolOptCreateDoesNotFireOnWrongName verifies that
// "Create null var" does NOT fire on SymbolOptCreate with names other than "_".
func Test_Regression_SymbolOptCreateDoesNotFireOnWrongName(t *testing.T) {
	names := []string{"err", "out", "result", "gotError"}
	for _, name := range names {
		b := New("soc regression " + name)
		b.Emit(SymbolOptCreate, name)
		b.instructions = b.instructions[:b.nextAddress]

		count := mustOptimize(t, b)
		if count != 0 {
			t.Errorf("Create null var fired on SymbolOptCreate %q — regression", name)
		}
	}
}

// Test_Regression_MultiValueAssignment simulates the pattern generated by
// "d, err := f()" which uses SymbolOptCreate+Store for each variable.  With
// the regression present, both Store instructions were replaced with Drop.
func Test_Regression_MultiValueAssignment(t *testing.T) {
	b := New("multi value assign")
	// Simulates the bytecode for:  d, err := f()
	b.Emit(AtLine, 5)
	b.Emit(SymbolOptCreate, "d")
	b.Emit(Store, "d")
	b.Emit(SymbolOptCreate, "err")
	b.Emit(Store, "err")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)

	// Both Store instructions must still store to their named variables.
	// Any Store → Drop replacement for "d" or "err" is a bug.
	for idx := 0; idx < b.nextAddress; idx++ {
		i := b.instructions[idx]
		if i.Operation == Drop {
			t.Errorf("addr %d: Store was incorrectly replaced with Drop", idx)
		}

		if i.Operation == Store {
			if i.Operand == "_" {
				t.Errorf("addr %d: Store was replaced with Store(\"_\")", idx)
			}
		}
	}
}

// Test_Regression_TryCatch_GotError simulates the catch-block assignment
// that failed in tests/cast/string.ego.  "gotError = true" must not be
// optimized away.
func Test_Regression_TryCatch_GotError(t *testing.T) {
	b := New("try catch gotError")
	// Simulates the bytecode in the catch block: gotError = true
	b.Emit(AtLine, 10)
	b.Emit(Push, true)
	b.Emit(Store, "gotError")
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)

	// Store "gotError" must remain — it must NOT become Drop.
	found := false

	for idx := 0; idx < b.nextAddress; idx++ {
		i := b.instructions[idx]
		if i.Operation == Store && i.Operand == "gotError" {
			found = true
		}
	}

	if !found {
		t.Error("Store \"gotError\" was removed or replaced — catch-block regression")
	}
}

// ── Section 6: Branch safety ─────────────────────────────────────────────────

// Test_Optimize_BranchSafety_NoFireWhenTargetInWindow verifies that the
// optimizer refuses to collapse instructions when a branch target points into
// the candidate pattern window.
//
// Layout:
//
//	0: Branch 1   ← targets address 1, which is inside the AtLine+AtLine window
//	1: AtLine 5
//	2: AtLine 6   ← normally: rule "Sequential AtLine" would remove addr 1
//	3: Push 0
//
// Because address 1 is a branch target, the AtLine+AtLine rule must NOT fire.
func Test_Optimize_BranchSafety_NoFireWhenTargetInWindow(t *testing.T) {
	b := New("branch safety")
	b.Emit(Branch, 1) // 0 — targets addr 1 (inside AtLine window)
	b.Emit(AtLine, 5) // 1 ← branch target
	b.Emit(AtLine, 6) // 2
	b.Emit(Push, 0)   // 3
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	// The AtLine rule at position 1 must be blocked.  Other rules might still
	// fire (e.g. Push 0 could participate if a matching rule existed), but the
	// AtLine pair must survive.
	_ = count

	// Verify that both AtLine instructions still exist.
	atlines := 0

	for i := 0; i < b.nextAddress; i++ {
		if b.instructions[i].Operation == AtLine {
			atlines++
		}
	}

	if atlines < 2 {
		t.Errorf("branch safety violated: only %d AtLine instructions remain (expected 2)", atlines)
	}
}

// Test_Optimize_BranchSafety_AllowsNonTargeted verifies the optimizer CAN
// collapse instructions at positions that are NOT branch targets.
func Test_Optimize_BranchSafety_AllowsNonTargeted(t *testing.T) {
	b := New("branch safety allow")
	b.Emit(Branch, 3)  // 0 — targets addr 3 (NOT inside the AtLine window at 1-2)
	b.Emit(AtLine, 10) // 1 ← NOT a branch target
	b.Emit(AtLine, 11) // 2
	b.Emit(Push, 0)    // 3 ← branch target, but not part of the AtLine window
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count == 0 {
		t.Error("AtLine+AtLine rule should fire when neither instruction is a branch target")
	}

	// Only one AtLine should remain after collapse.
	atlines := 0

	for i := 0; i < b.nextAddress; i++ {
		if b.instructions[i].Operation == AtLine {
			atlines++
		}
	}

	if atlines != 1 {
		t.Errorf("expected 1 AtLine after optimization, got %d", atlines)
	}
}

// ── Section 7: Branch target adjustment ──────────────────────────────────────

// Test_Optimize_BranchAdjustment_AfterPatch verifies that a Branch instruction
// whose target is AFTER the removed instructions has its operand decremented.
//
// Layout:
//
//	0: AtLine 5
//	1: AtLine 6    ← rule collapses 0-1 → removes one instruction
//	2: Push 0
//	3: Branch 4    ← was targeting addr 4, should become addr 3 after -1
//	4: Push 1
//
// After optimization: AtLine 6, Push 0, Branch 3, Push 1
// (addr 4 → new addr 3 after deleting one instruction).
func Test_Optimize_BranchAdjustment_AfterPatch(t *testing.T) {
	b := New("branch adj")
	b.Emit(AtLine, 5) // 0
	b.Emit(AtLine, 6) // 1
	b.Emit(Push, 0)   // 2
	b.Emit(Branch, 4) // 3 — targets addr 4
	b.Emit(Push, 1)   // 4
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)

	// After: AtLine(6) at 0, Push(0) at 1, Branch at 2, Push(1) at 3.
	// The branch at the original addr 3 (now addr 2) should point to addr 3.
	assertSize(t, b, 4)

	branchInstr := instrAt(t, b, 2)
	if branchInstr.Operation != Branch {
		t.Fatalf("expected Branch at addr 2, got %v", branchInstr.Operation)
	}

	if branchInstr.Operand != 3 {
		t.Errorf("Branch operand = %v, want 3 (adjusted for deleted instruction)", branchInstr.Operand)
	}
}

// ── Section 8: Backtracking ───────────────────────────────────────────────────

// Test_Optimize_Backtracking_CascadeOpportunity verifies that after one rule
// fires and the scanner backs up, a newly exposed opportunity is also captured.
//
// The sequence:
//
//	Push StackMarker("let")
//	Push 2
//	CreateAndStore "x"
//	DropToMarker StackMarker("let")
//
// The 4-instruction "Unnecessary stack marker" rule fires first, yielding:
//
//	Push 2
//	CreateAndStore "x"
//
// The scanner backs up and sees Push(2) + CreateAndStore("x"), which matches
// "Collapse constant Push and CreateAndStore", collapsing to:
//
//	CreateAndStore ["x", 2]
func Test_Optimize_Backtracking_CascadeOpportunity(t *testing.T) {
	b := New("cascade backtrack")
	b.Emit(Push, NewStackMarker("let"))
	b.Emit(Push, 2)
	b.Emit(CreateAndStore, "x")
	b.Emit(DropToMarker, NewStackMarker("let"))
	b.instructions = b.instructions[:b.nextAddress]

	count := mustOptimize(t, b)
	if count < 2 {
		t.Errorf("expected >= 2 optimizations (cascade), got %d", count)
	}

	assertSize(t, b, 1)

	i := instrAt(t, b, 0)
	if i.Operation != CreateAndStore {
		t.Fatalf("expected CreateAndStore after cascade, got %v", i.Operation)
	}

	arr, ok := i.Operand.([]any)
	if !ok || len(arr) != 2 || arr[0] != "x" || arr[1] != 2 {
		t.Errorf("cascaded CreateAndStore operand = %#v, want [x 2]", i.Operand)
	}
}

// Test_Optimize_Backtracking_AtLineCascade verifies three consecutive AtLine
// instructions are reduced to one in a single optimize() call (backtracking
// ensures the second reduction is not missed).
func Test_Optimize_Backtracking_AtLineCascade(t *testing.T) {
	b := New("atline cascade")
	// Pre-pad with a non-matching instruction so the backtrack starts earlier.
	b.Emit(Push, 0)
	b.Emit(AtLine, 1)
	b.Emit(AtLine, 2)
	b.Emit(AtLine, 3)
	b.instructions = b.instructions[:b.nextAddress]

	mustOptimize(t, b)
	// Push(0) should remain; the three AtLines should collapse to AtLine(3).
	assertSize(t, b, 2)
	assertInstr(t, b, 0, Push, 0)
	assertInstr(t, b, 1, AtLine, 3)
}

// ── Section 9: Seal integration ──────────────────────────────────────────────

// Test_Seal_AlwaysOptimize verifies that Seal() runs the optimizer when the
// settings value is 2 ("always optimize"), regardless of bytecode size.
func Test_Seal_AlwaysOptimize(t *testing.T) {
	// Save and restore the optimizer setting.
	orig := settings.Get(defs.OptimizerSetting)
	defer settings.Set(defs.OptimizerSetting, orig)

	settings.Set(defs.OptimizerSetting, "2")

	b := New("seal test")
	b.Emit(AtLine, 1)
	b.Emit(AtLine, 2) // two consecutive AtLines — should be collapsed

	b.Seal()

	atlines := 0

	for i := 0; i < b.nextAddress; i++ {
		if b.instructions[i].Operation == AtLine {
			atlines++
		}
	}

	if atlines != 1 {
		t.Errorf("Seal with mode=2 did not run optimizer: found %d AtLine instructions, want 1", atlines)
	}
}

// Test_Seal_DisabledOptimize verifies that Seal() with setting 0 leaves
// bytecode unchanged.
func Test_Seal_DisabledOptimize(t *testing.T) {
	orig := settings.Get(defs.OptimizerSetting)
	defer settings.Set(defs.OptimizerSetting, orig)

	settings.Set(defs.OptimizerSetting, "0")

	b := New("seal disabled")
	b.Emit(AtLine, 1)
	b.Emit(AtLine, 2)

	b.Seal()

	if b.nextAddress != 2 {
		t.Errorf("Seal with mode=0 should not optimize; got %d instructions", b.nextAddress)
	}
}

// Test_Seal_Idempotent verifies that calling Seal() twice does not further
// modify the bytecode (the optimized flag prevents a second pass).
func Test_Seal_Idempotent(t *testing.T) {
	orig := settings.Get(defs.OptimizerSetting)
	defer settings.Set(defs.OptimizerSetting, orig)

	settings.Set(defs.OptimizerSetting, "2")

	b := New("seal idempotent")
	b.Emit(AtLine, 1)
	b.Emit(AtLine, 2)

	b.Seal()
	sizeAfterFirst := b.nextAddress

	b.Seal() // should be a no-op (already sealed)

	if b.nextAddress != sizeAfterFirst {
		t.Errorf("second Seal() changed instruction count from %d to %d", sizeAfterFirst, b.nextAddress)
	}
}
