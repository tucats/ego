package functions

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// This file contains support for shims to native functions called against
// Go-defined types.
//
// For each of the native functions that can be called on  a native type,
// implement the shim wrapper. Each one has the responsibility of validating
// the argument count, locating the native "this" value, mapping it to the
// correct type, and then calling the function.

type NativeFunction func(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError)

type NativeFunctionDef struct {
	Kind *datatypes.Type
	Name string
	F    NativeFunction
}

// NativeFunctionMap defines, for each combination of data type and function
// name, specify the native function handler for that type. For example, a
// sync.WaitGroup has Add(), Done(), and Wait() methods and are all shown
// here. This table is used by the Member opcode to check to see if the member
// index is into a natively implemented type...
var NativeFunctionMap = []NativeFunctionDef{
	{
		Kind: &datatypes.WaitGroupType,
		Name: "Wait",
		F:    waitGroupWait,
	},
	{
		Kind: &datatypes.WaitGroupType,
		Name: "Add",
		F:    waitGroupAdd,
	},
	{
		Kind: &datatypes.WaitGroupType,
		Name: "Done",
		F:    waitGroupDone,
	},
	{
		Kind: datatypes.Pointer(&datatypes.WaitGroupType),
		Name: "Wait",
		F:    waitGroupWait,
	},
	{
		Kind: datatypes.Pointer(&datatypes.WaitGroupType),
		Name: "Add",
		F:    waitGroupAdd,
	},
	{
		Kind: datatypes.Pointer(&datatypes.WaitGroupType),
		Name: "Done",
		F:    waitGroupDone,
	},
	{
		Kind: &datatypes.MutexType,
		Name: "Lock",
		F:    mutexLock,
	},
	{
		Kind: &datatypes.MutexType,
		Name: "Unlock",
		F:    mutexUnlock,
	},
	{
		Kind: datatypes.Pointer(&datatypes.MutexType),
		Name: "Lock",
		F:    mutexLock,
	},
	{
		Kind: datatypes.Pointer(&datatypes.MutexType),
		Name: "Unlock",
		F:    mutexUnlock,
	},
}

// For a given datatype and name, see if there is a native function that
// supports this operation.  If so, return it's function pointer.
func FindNativeFunction(kind *datatypes.Type, name string) NativeFunction {
	for _, f := range NativeFunctionMap {
		if f.Kind.IsType(kind) && f.Name == name {
			return f.F
		}
	}

	return nil
}

// getNativeThis returns a map for the "this" object in the current
// symbol table. It doesn't require it to be of any specific type, as
// that will be handled via mapping within the individual native
// function shims.
func getNativeThis(s *symbols.SymbolTable) interface{} {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	if p, ok := t.(*interface{}); ok {
		return *p
	}

	return t
}
