// Package bytecode contains a byte code interpreter.
//
// A ByteCode object can be created by native Go code, or by using the
// compiler package. Bytecode consists of a stream of pseudo-instructions
// (bytecodes). The bytecode does not inherently have a symbol table, as
// all symbolic names are expressed as strings in the code to be bound to
// storate only during execution.
//
// The package also allows for creating a Context object that encapsulates the
// runtime status of the execution of a ByteCode object. There can be many
// different Context objects pointing to the same Bytecode, which allows for
// sharing of the pseudo-instruction stream (such as in threads).  Each Context
// contains the state of one execution of the code, which includes a symbol
// table, data stack, and other execution state information.
//
// Bytecode is executed until an error occurs, in which time the context.Run()
// operation returns with an error. If the code exits, it retursn the special
// errors.Exit return code. If the code runs to the start of a new line of
// source (based on information stored in the bytecode) and the debugger is
// enabled, the context returns errors.Debugger and the caller should invoke
// the debugger package before resuming exeution.
//
// The bytecode has a number of instructions that support managing the Ego
// language semantics.
//
//   - The symbol table is used to store all values. Symbol tables can be
//     marked as shared, which means they operation in a thread-safe fashion
//     and can be shared between go routines, server handers, etc. Symbols can
//     only be written after they are created. A symbol can be created with no
//     type, and takes on the type of the first value stored.
//
//   - Symbol tables can be chained together (that is, any symbol table can have
//     one parent, and any symbol table can be the parent to many other tables).
//     This is used to implement scope. New symbols are always created in the
//     current table (local scope). Storing into existing symbols search the
//     symbol table tree to find the symbol, only stopping when a table marked
//     as "top-of-scope" is found. When a symbol table scope completes (such as
//     at the end of the execution of a block in Ego), the symbol table is
//     deleted which resultes in discarding any variables created in that local
//     scope.
//
//   - Type safety is imposed at runtime. This means the degree of type checking
//     can be dynamically changed during the execution of the code. In strict
//     mode, storage into an object is only permitted for objects of the same
//     type. In relaxed mode, type conversions are permitted to ensure the value
//     is converted to the stored type if possible. Dyanmic means the type of the
//     storage is changed to match the type of the value stored.
//
//   - Error trapping (try/catch) are implemetned by a stack in the Context that is
//     added to when the try statement begins, and is discarded after the code for the
//     catch block is executed. The stack includes information about the class of errors
//     that the block will handle (this is really "all errors" or "math errors", the
//     latter of which support the ? optional operation).
//
//   - Bytecode has the ability to call other functions from the current context. These
//     can be other bytecode streams, in which case a new subordinate context is created
//     to support executing that function (this prevents the function from being able
//     to access symbol table values other than the root/global symbol table). They can
//     also be Type designations, in which case this becomes a proxy for calling an
//     internal built-in function called "$cast()". Finally, they can be "native"
//     functions, which are functions implemented with Ego as native Go code.  The
//     native function has access to the symbol table tree, and returns a value (or
//     tuple of values) and a runtime error, if any.
//
//   - There are bytecode instructions for creating, storing, and deleting symbole
//     values. There are instructions for creating any complex type supported by the
//     "data" package, such as Arrays, Maps, and Structs. There are instrutcions for
//     accessing members of those complex types by index value, key value, or field
//     name. There are instrutions for managing flow-of-control, including branching
//     within the bytecode stream or calling functions in the same or another stream.
//
// Here is a trivial example of generating bytecode and executing it.
//
//	// Create a ByteCode object and write some instructions into it.
//	   b := bytecode.New("sample program")
//	   b.Emit(bytecode.Load, "strings")
//	   b.Emit(bytecode.Member, "Left")
//	   b.Emit{bytecode.Push, "fruitcake")
//	   b.Emit(bytecode.Push, 5)
//	   b.Emit(bytecode.Call, 2)
//	   b.Emit(bytecode.Stop)
//
//	// Make a symbol table, so we can call the function library.
//	   s := symbols.NewSymbolTable("sample program")
//	   functions.AddBuiltins(s)
//
//	// Make a runtime context for this bytecode, and then run it.
//	// The context has the symbol table and bytecode attached to it.
//	   c := bytecode.NewContext(s, b)
//	   err := c.Run()
//
//	// Retrieve the last value and extract a string
//	   v, err := b.Pop()
//	   fmt.Printf("The result is %s\n", data.GetString(v))
//
// This creates a new bytecode stream, and then adds instructions to it. These
// instructions would nominally be added by the compiler when using _Ego_. The
// `Emit()` function emits an instruction with optional opcodes.
//
// The generated bytecode puts arguments to a function on a stack, and then
// calls the function. The result is left on the stack, and can be popped off
// after execution completes. The result (which is always an abstract
// interface{}) is then converted to a string and printed.
package bytecode
