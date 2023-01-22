package functions

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// This file contains support for shims to native functions called against
// Go-defined types.
//
// For each of the native functions that can be called on  a native type,
// implement the shim wrapper. Each one has the responsibility of validating
// the argument count, locating the native "this" value, mapping it to the
// correct type, and then calling the function.

// NativeFunction defines the signature of native (i.e. builtin) runtime
// functions.
type NativeFunction func(s *symbols.SymbolTable, args []interface{}) (interface{}, error)

type nativeFunctionDef struct {
	Kind *data.Type
	Name string
	F    NativeFunction
}

// nativeFunctionMap defines, for each combination of data type and function
// name, specify the native function handler for that type. For example, a
// sync.WaitGroup has Add(), Done(), and Wait() methods and are all shown
// here. This table is used by the Member opcode to check to see if the member
// index is into a natively implemented type...
var nativeFunctionMap = []nativeFunctionDef{
	{
		Kind: data.WaitGroupType,
		Name: "Wait",
		F:    waitGroupWait,
	},
	{
		Kind: data.WaitGroupType,
		Name: "Add",
		F:    waitGroupAdd,
	},
	{
		Kind: data.WaitGroupType,
		Name: "Done",
		F:    waitGroupDone,
	},
	{
		Kind: data.PointerType(data.WaitGroupType),
		Name: "Wait",
		F:    waitGroupWait,
	},
	{
		Kind: data.PointerType(data.WaitGroupType),
		Name: "Add",
		F:    waitGroupAdd,
	},
	{
		Kind: data.PointerType(data.WaitGroupType),
		Name: "Done",
		F:    waitGroupDone,
	},
	{
		Kind: data.MutexType,
		Name: "Lock",
		F:    mutexLock,
	},
	{
		Kind: data.MutexType,
		Name: "Unlock",
		F:    mutexUnlock,
	},
	{
		Kind: data.PointerType(data.MutexType),
		Name: "Lock",
		F:    mutexLock,
	},
	{
		Kind: data.PointerType(data.MutexType),
		Name: "Unlock",
		F:    mutexUnlock,
	},
}

// For a given datatype and name, see if there is a native function that
// supports this operation.  If so, return it's function pointer.
func FindNativeFunction(kind *data.Type, name string) NativeFunction {
	for _, f := range nativeFunctionMap {
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
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	if p, ok := t.(*interface{}); ok {
		return *p
	}

	return t
}

func GetName(f NativeFunction) string {
	// Native functions are methods on actual Go objects that we surface to Ego
	// code. Examples include the functions for waitgroup and mutex objects.
	functionName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	functionName = strings.Replace(functionName, "github.com/tucats/ego/", "", 1)

	return functionName
}
