package symbols

// This file implements the slot bank: a fixed-size, integer-indexed array of
// local-variable values used by compile-time slot assignment (see docs/SLOTS.md
// and the locals field on SymbolTable in tables.go).
//
// The slot bank is deliberately much simpler than the name-based
// symbols/values machinery in values.go:
//
//   - Its size is fixed at allocation time (the compiler knows exactly how
//     many distinct locals a slot-eligible function declares), so there is no
//     bin growth and no bin math -- an index is a direct offset into one flat
//     []any.
//   - There is no map: the compiler resolved every name to an integer slot at
//     compile time, so nothing here ever translates a name to a slot.
//   - Because a slot-eligible function contains no closures/go/defer (by
//     construction -- that is the eligibility predicate), its bank belongs to
//     exactly one call activation on one goroutine and needs no locking. The
//     methods below intentionally do not take the table mutex.

// AllocateLocals attaches a fresh slot bank of len(names) entries to this table,
// each initialized to UndefinedValue{} (matching what Create() stores for a
// declared-but-unassigned name in the name-based path), and records names as the
// slot->name metadata table (see the localNames field). It is called by the
// AllocateLocal opcode at the entry of a slot-eligible function, against the
// boundary table that was just pushed for the call. An empty names slice is
// legal (an eligible function that declares no locals at all) and installs a
// non-nil, empty bank so LocalsBank() still recognizes this table as the bank
// owner.
func (s *SymbolTable) AllocateLocals(names []string) {
	if s == nil {
		return
	}

	bank := make([]any, len(names))
	for i := range bank {
		bank[i] = UndefinedValue{}
	}

	s.locals = bank
	s.localNames = names
}

// slotIndexByName resolves a name to a slot index in this table's own bank,
// returning the index and true, or (-1, false) if the name is not a slotted
// local here. It scans from the highest index down so that, when a name is
// shadowed across nested blocks (appearing at more than one index), the
// innermost (most-recently-declared) binding wins -- the intuitive choice for
// introspection. This is a cold path (introspection only); the hot slot opcodes
// never call it.
func (s *SymbolTable) slotIndexByName(name string) (int, bool) {
	if s == nil {
		return -1, false
	}

	for i := len(s.localNames) - 1; i >= 0; i-- {
		if s.localNames[i] == name {
			return i, true
		}
	}

	return -1, false
}

// slotValueByName returns the value of a slotted local by name from this table's
// own bank, and true, or (nil, false) if the name is not a slotted local here.
func (s *SymbolTable) slotValueByName(name string) (any, bool) {
	if idx, ok := s.slotIndexByName(name); ok {
		return s.locals[idx], true
	}

	return nil, false
}

// HasLocals reports whether this table owns a slot bank (i.e. AllocateLocals
// has been called on it). Note that an empty (zero-length) bank still counts:
// a nil check on the field is not equivalent, which is why this method exists.
func (s *SymbolTable) HasLocals() bool {
	return s != nil && s.locals != nil
}

// LocalsBank returns the nearest table in the parent chain (starting with this
// one) that owns a slot bank, or nil if none does. Under docs/SLOTS.md Section
// 5.3 Option A, a nested block scope inside a slot-eligible function has no
// bank of its own, so the LoadSlot/StoreSlot/etc. opcodes running inside it
// resolve their bank by walking up to the enclosing function's boundary table.
// Under Option B (the eventual target), the current table is itself the bank
// owner and this walk terminates immediately.
func (s *SymbolTable) LocalsBank() *SymbolTable {
	for t := s; t != nil; t = t.parent {
		if t.locals != nil {
			return t
		}
	}

	return nil
}

// GetRegister returns the value stored in the given slot of this table's own bank.
// The bool result is false if this table owns no bank or the index is out of
// range -- both of which indicate a compiler bug (a slot opcode was emitted
// with no matching AllocateLocal, or with a bad index), not a normal runtime
// condition. Callers that must tolerate a nested-block scope should resolve the
// bank owner with LocalsBank() first and call this on that table.
func (s *SymbolTable) GetRegister(index int) (any, bool) {
	if s == nil || index < 0 || index >= len(s.locals) {
		return nil, false
	}

	return s.locals[index], true
}

// SetRegister writes a value into the given slot of this table's own bank. It
// returns false (writing nothing) if this table owns no bank or the index is
// out of range -- again a compiler-bug signal, not a normal condition.
func (s *SymbolTable) SetRegister(index int, v any) bool {
	if s == nil || index < 0 || index >= len(s.locals) {
		return false
	}

	s.locals[index] = v
	s.modified = true

	return true
}

// AddressOfRegister returns the address of the given slot in this table's own bank,
// or nil if this table owns no bank or the index is out of range. Because the
// bank is a single fixed-size array that is never grown or reallocated after
// AllocateLocals, this pointer stays valid for the life of the call activation
// -- the same stability guarantee addressOfValue provides for the name-based
// path, but without the bin indirection that machinery needs to preserve it.
func (s *SymbolTable) AddressOfRegister(index int) *any {
	if s == nil || index < 0 || index >= len(s.locals) {
		return nil
	}

	return &s.locals[index]
}
