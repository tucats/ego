package bytecode

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// This file implements the register-slot-based local-variable opcodes
// described in docs/SLOTS.md. Each is the integer-indexed counterpart of
// a name-based opcode in load.go / store.go / symbols.go, and each
// resolves its register bank with c.symbols.LocalsBank() -- the nearest
// enclosing table that owns a bank (see internal/language/symbols/slots.go).
// The compiler only emits these when it has proven, at compile time,
// that an identifier resolves to a register slot in the current function's own
// bank, so an absent bank or out-of-range index here is always a compiler
// issue, reported via ErrInternalCompiler rather than a user-facing error.

// registerBank resolves the register index operand and the owning register
// bank for a register-variant opcode, returning a descriptive internal error
// if either is unavailable. The opName is embedded in that error's context
// to make a compiler bug easy to pinpoint.
func (c *Context) registerBank(i any, opName string) (*symbols.SymbolTable, int, error) {
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
// []string of local names the enclosing register-eligible function declares, one per
// register index (the count is len(names)). It attaches a fresh, fixed-size register
// bank of that size (every entry UndefinedValue{}) to the current symbol table
// -- the boundary table just pushed for this call activation -- together with
// the names as register->name introspection metadata (docs/SLOTS.md, Q3). Emitted
// once, at function entry.
func allocateLocalByteCode(c *Context, i any) error {
	names, ok := i.([]string)
	if !ok {
		return c.runtimeError(errors.ErrInternalCompiler).Context("AllocateLocal: operand is not a name list")
	}

	c.symbols.AllocateLocals(names)

	return nil
}

// loadRegisterByteCode implements the LoadRegister opcode: the register equivalent of
// loadByteCode. It pushes the value in the given register (with any constant
// wrapper unwrapped, matching Load).
func loadRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "LoadRegister")
	if err != nil {
		return err
	}

	v, ok := bank.GetRegister(index)
	if !ok {
		return c.runtimeError(errors.ErrInternalCompiler).Context("LoadRegister: register index out of range")
	}

	return c.push(data.UnwrapConstant(v))
}

// storeRegisterByteCode implements the StoreRegister opcode: the register equivalent of
// storeByteCode. It pops a value, applies the same type-compatibility check
// (checkTypeRegister) and struct value-copy semantics (copyStructForValueSemantics)
// as a named store, then writes it into the register. Because register readonly-ness is
// resolved at compile time (see docs/SLOTS.md Section 6), no runtime "_"-prefix
// check is performed here -- the compiler emits StoreRegister only for a writable
// register.
func storeRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "StoreRegister")
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

	value, err = c.checkTypeRegister(bank, index, value)
	if err != nil {
		return c.runtimeError(err)
	}

	// fix BUG-26 parity: binding a struct value to a register must copy it, not
	// alias it -- exactly as storeByteCode does for a named destination.
	value = copyStructForValueSemantics(value)

	if !bank.SetRegister(index, value) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("StoreRegister: register index out of range")
	}

	return nil
}

// createAndStoreRegisterByteCode implements the CreateAndStoreRegister opcode: the register
// equivalent of createAndStoreByteCode for a ":=" declaration whose target is a
// register. Since the register already exists (allocated by AllocateLocal), there is no
// separate "create" step -- this is a store that additionally honors the
// language-extension rule forbidding a bare type value unless extensions are
// enabled, matching CreateAndStore.
func createAndStoreRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "CreateAndStoreRegister")
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
	if !bank.SetRegister(index, copyStructForValueSemantics(value)) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("CreateAndStoreRegister: register index out of range")
	}

	return nil
}

// symbolCreateIfRegisterByteCode implements the SymbolOptCreateRegister opcode: the register
// equivalent of symbolCreateIfByteCode. For a register slot, the storage always already
// exists (AllocateLocal created it), so an idempotent "create if not present"
// is a no-op beyond validating the bank and index. It exists so the compiler
// can emit a register analogue wherever it currently emits SymbolOptCreate without
// a special case; see docs/SLOTS.md Section 6, which flags that this opcode may
// prove entirely unnecessary once register assignment subsumes Finding 11's
// idempotent-declaration analysis.
func symbolCreateIfRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "SymbolOptCreateRegister")
	if err != nil {
		return err
	}

	if _, ok := bank.GetRegister(index); !ok {
		return c.runtimeError(errors.ErrInternalCompiler).Context("SymbolOptCreateRegister: register index out of range")
	}

	return nil
}

// symbolDeleteRegisterByteCode implements the SymbolDeleteRegister opcode: the register
// equivalent of symbolDeleteByteCode. It resets the register to UndefinedValue{},
// the same state AllocateLocal established, so a later declaration or read sees
// the register as unassigned again.
func symbolDeleteRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "SymbolDeleteRegister")
	if err != nil {
		return err
	}

	if !bank.SetRegister(index, symbols.UndefinedValue{}) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("SymbolDeleteRegister: register index out of range")
	}

	return nil
}

// addressOfRegisterByteCode implements the AddressOfRegister opcode: the register
// equivalent of addressOfByteCode. It pushes a *any pointing directly at the
// register's storage. The bank is a single fixed-size array that is never grown
// after AllocateLocal, so this address stays valid for the life of the call.
func addressOfRegisterByteCode(c *Context, i any) error {
	bank, index, err := c.registerBank(i, "AddressOfRegister")
	if err != nil {
		return err
	}

	addr := bank.AddressOfRegister(index)
	if addr == nil {
		return c.runtimeError(errors.ErrInternalCompiler).Context("AddressOfRegister: register index out of range")
	}

	return c.push(addr)
}
