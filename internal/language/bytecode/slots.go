package bytecode

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// This file implements the slot-based local-variable opcodes described in
// docs/SLOTS.md. Each is the integer-indexed counterpart of a name-based
// opcode in load.go / store.go / symbols.go, and each resolves its slot bank
// with c.symbols.LocalsBank() -- the nearest enclosing table that owns a bank
// (see internal/language/symbols/slots.go). The compiler only emits these when
// it has proven, at compile time, that an identifier resolves to a slot in the
// current function's own bank, so an absent bank or out-of-range index here is
// always a compiler bug, reported via ErrInternalCompiler rather than a
// user-facing error.

// slotBank resolves the slot index operand and the owning slot bank for a slot
// opcode, returning a descriptive internal error if either is unavailable. The
// opName is embedded in that error's context to make a compiler bug easy to
// pinpoint.
func (c *Context) slotBank(i any, opName string) (*symbols.SymbolTable, int, error) {
	index, err := data.Int(i)
	if err != nil {
		return nil, 0, c.runtimeError(err)
	}

	bank := c.symbols.LocalsBank()
	if bank == nil {
		return nil, 0, c.runtimeError(errors.ErrInternalCompiler).Context(opName + ": no locals bank in scope")
	}

	return bank, index, nil
}

// allocateLocalByteCode implements the AllocateLocal opcode. Its operand is the
// number of distinct locals the enclosing slot-eligible function declares. It
// attaches a fresh, fixed-size slot bank of that size (every entry
// UndefinedValue{}) to the current symbol table -- which is the boundary table
// just pushed for this call activation. Emitted once, at function entry.
func allocateLocalByteCode(c *Context, i any) error {
	n, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if n < 0 {
		return c.runtimeError(errors.ErrInternalCompiler).Context("AllocateLocal: negative slot count")
	}

	c.symbols.AllocateLocals(n)

	return nil
}

// loadSlotByteCode implements the LoadSlot opcode: the slot equivalent of
// loadByteCode. It pushes the value in the given slot (with any constant
// wrapper unwrapped, matching Load).
func loadSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "LoadSlot")
	if err != nil {
		return err
	}

	v, ok := bank.GetSlot(index)
	if !ok {
		return c.runtimeError(errors.ErrInternalCompiler).Context("LoadSlot: slot index out of range")
	}

	return c.push(data.UnwrapConstant(v))
}

// storeSlotByteCode implements the StoreSlot opcode: the slot equivalent of
// storeByteCode. It pops a value, applies the same type-compatibility check
// (checkTypeSlot) and struct value-copy semantics (copyStructForValueSemantics)
// as a named store, then writes it into the slot. Because slot readonly-ness is
// resolved at compile time (see docs/SLOTS.md Section 6), no runtime "_"-prefix
// check is performed here -- the compiler emits StoreSlot only for a writable
// slot.
func storeSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "StoreSlot")
	if err != nil {
		return err
	}

	value, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(value) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	value, err = c.checkTypeSlot(bank, index, value)
	if err != nil {
		return c.runtimeError(err)
	}

	// fix BUG-26 parity: binding a struct value to a slot must copy it, not
	// alias it -- exactly as storeByteCode does for a named destination.
	value = copyStructForValueSemantics(value)

	if !bank.SetSlot(index, value) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("StoreSlot: slot index out of range")
	}

	return nil
}

// createAndStoreSlotByteCode implements the CreateAndStoreSlot opcode: the slot
// equivalent of createAndStoreByteCode for a ":=" declaration whose target is a
// slot. Since the slot already exists (allocated by AllocateLocal), there is no
// separate "create" step -- this is a store that additionally honors the
// language-extension rule forbidding a bare type value unless extensions are
// enabled, matching CreateAndStore.
func createAndStoreSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "CreateAndStoreSlot")
	if err != nil {
		return err
	}

	value, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(value) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Storing a bare type value is a language-extension feature.
	if _, ok := value.(*data.Type); ok && !c.extensions {
		return c.runtimeError(errors.ErrInvalidType)
	}

	// fix BUG-26 parity: ":=" of a struct value copies, it does not alias.
	if !bank.SetSlot(index, copyStructForValueSemantics(value)) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("CreateAndStoreSlot: slot index out of range")
	}

	return nil
}

// symbolCreateIfSlotByteCode implements the SymbolOptCreateSlot opcode: the slot
// equivalent of symbolCreateIfByteCode. For a slot, the storage always already
// exists (AllocateLocal created it), so an idempotent "create if not present"
// is a no-op beyond validating the bank and index. It exists so the compiler
// can emit a slot analogue wherever it currently emits SymbolOptCreate without
// a special case; see docs/SLOTS.md Section 6, which flags that this opcode may
// prove entirely unnecessary once slot assignment subsumes Finding 11's
// idempotent-declaration analysis.
func symbolCreateIfSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "SymbolOptCreateSlot")
	if err != nil {
		return err
	}

	if _, ok := bank.GetSlot(index); !ok {
		return c.runtimeError(errors.ErrInternalCompiler).Context("SymbolOptCreateSlot: slot index out of range")
	}

	return nil
}

// symbolDeleteSlotByteCode implements the SymbolDeleteSlot opcode: the slot
// equivalent of symbolDeleteByteCode. It resets the slot to UndefinedValue{},
// the same state AllocateLocal established, so a later declaration or read sees
// the slot as unassigned again.
func symbolDeleteSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "SymbolDeleteSlot")
	if err != nil {
		return err
	}

	if !bank.SetSlot(index, symbols.UndefinedValue{}) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("SymbolDeleteSlot: slot index out of range")
	}

	return nil
}

// addressOfSlotByteCode implements the AddressOfSlot opcode: the slot
// equivalent of addressOfByteCode. It pushes a *any pointing directly at the
// slot's storage. The bank is a single fixed-size array that is never grown
// after AllocateLocal, so this address stays valid for the life of the call.
func addressOfSlotByteCode(c *Context, i any) error {
	bank, index, err := c.slotBank(i, "AddressOfSlot")
	if err != nil {
		return err
	}

	addr := bank.AddressOfSlot(index)
	if addr == nil {
		return c.runtimeError(errors.ErrInternalCompiler).Context("AddressOfSlot: slot index out of range")
	}

	return c.push(addr)
}
