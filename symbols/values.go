package symbols

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// A few notes about how values are stored in a symbol table.
//
// 1. There is a simple map in the symbol table that maps the
//    symbol name to a slot number. The slot number is a value
//    that increments each time a new symbol is created in that
//    table.
//
// 2. Slot numbers are used to locate the value itself. VAlues are
//    stored in a []interface{}, which is always created using the
//    SymbolAllocationSize value. The address of this array is then
//    stored in the symbol table's Values array, which is an array
//    of pointers to the value lists.
//
// 3. When a value array is exhausted (i.e. a slot number exceeds
//    its capacity), a new value array is created and it's adderess
//	  is added to the Values array list.
//
// This somewhat cumbersome mechanism guarantees that, for the life
// of a symbol table, any address-of-value operations remain valid
// even if the symbol table must be expanded due to value list
// exhaustion. The new value list is appended to the Values array,
// and if they weren't pointers to the value list, then the array
// memory manager subsystem might move the arrays, breaking the
// address-of values.

const (
	noSlot   = -1
	notFound = "<not found>"
	elipses  = "..."
)

type UndefinedValue struct {
}

// initializeValues allocates the initial values list and creates the Values
// array with it's address. If the bin map is nil, create it as well.
func (s *SymbolTable) initializeValues() {
	if s.symbols == nil {
		s.symbols = map[string]*SymbolAttribute{}
	}

	if s.values == nil {
		bin := make([]interface{}, SymbolAllocationSize)
		s.values = make([]*[]interface{}, 1)
		s.values[0] = &bin
		s.size = 0
	}
}

// Given an index and a value, store the value in the Values list.
func (s *SymbolTable) setValue(index int, v interface{}) {
	if index == noSlot {
		return
	}

	// The index number is divided by the size of each value list to
	// determine which value list to use. The index number modulo the
	// maximum value list size gives the slot number within the selected
	// bin number.
	bin := index / SymbolAllocationSize
	for bin >= len(s.values) {
		newBin := make([]interface{}, SymbolAllocationSize)
		s.values = append(s.values, &newBin)

		ui.Log(ui.SymbolLogger, "symbols.new.bin", ui.A{
			"name": s.Name})
	}

	slot := index % SymbolAllocationSize
	(*s.values[bin])[slot] = v
	s.modified = true
}

// Given an index, retrieve a value from the Values list.
func (s *SymbolTable) getValue(index int) interface{} {
	if index == noSlot {
		return nil
	}

	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.values) {
		return nil
	}

	return (*s.values[bin])[slot]
}

// Given an index, return the address of the value in that
// slot.
func (s *SymbolTable) addressOfValue(index int) *interface{} {
	if index == noSlot {
		return nil
	}

	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.values) {
		return nil
	}

	return &(*s.values[bin])[slot]
}

// Given an index, return the address of the value in that
// slot.
func (s *SymbolTable) addressOfImmuableValue(index int) *interface{} {
	if index == noSlot {
		return nil
	}

	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.values) {
		return nil
	}

	oldValue := (*s.values[bin])[slot]
	if _, ok := oldValue.(data.Immutable); !ok {
		newValue := data.Constant(data.DeepCopy(oldValue))
		r := makeInterface(newValue)

		return &r
	}

	return &oldValue
}

func makeInterface(i data.Immutable) interface{} {
	return i
}
