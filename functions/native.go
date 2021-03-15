package functions

import (
	"sync"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// This file contains shims to native functions called
// against defined types.

type NativeFunction func(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError)

type NativeFunctionDef struct {
	Kind int
	Name string
	F    NativeFunction
}

// For each combination of data type and function name, specify the native
// function handler for that type. For example, a sync.WaitGroup has Add(),
// Done(), and Wait() methods and are all shown here. This table is used by
// the Member opcode to check to see if the member index is into a natively
// implemented type...
var NativeFunctionMap = []NativeFunctionDef{
	{
		Kind: datatypes.WaitGroupType,
		Name: "Wait",
		F:    waitGroupWait,
	},
	{
		Kind: datatypes.WaitGroupType,
		Name: "Add",
		F:    waitGroupAdd,
	},
	{
		Kind: datatypes.WaitGroupType,
		Name: "Done",
		F:    waitGroupDone,
	},
}

// For a given datatype and name, see if there is a native function that
// supports this operation.  If so, return it's function pointer.
func FindNativeFunction(kind int, name string) NativeFunction {
	for _, f := range NativeFunctionMap {
		if f.Kind == kind && f.Name == name {
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

	return t
}

// For each of the native functions that can be called on  a native type,
// implement the shim wrapper. Each one has the responsibility of validating
// the argument count, locating the native "this" value, mapping it to the
// correct type, and then calling the function.

// Waitgroup functions.

// sync.WaitGroup Add() function.
func waitGroupAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError).In("Add()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		count := util.GetInt(args[0])
		wg.Add(count)

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}

// sync.WaitGroup Done() function.
func waitGroupDone(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ArgumentCountError).In("Done()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Done()

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}

// sync.WaitGroup Wait() function.
func waitGroupWait(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ArgumentCountError).In("Wait()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Wait()

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}
