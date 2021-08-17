# bytecode

The `bytecode` package supports a simple bytecode interpreter. This allows operations (especially those that might be
repeated) to be compiled into an expression of the semantics of the operation, without having to have the string 
parsed and lexically analyzed repeatedly.

Bytecode can be generated explicitly (as in the first example below) or by using the compiler package which accepts
text in a Go-like language called _Ego_ and generates bytecode. Once the bytecode is generated, a runtime `Context`
object is created which is used to manage the execution of a bytecode stream. This includes it's active symbol table,
program counter, stack, etc. A `Context` is separate from the bytecode because the same bytecode could be executed 
on multiple threads, each with it's own `Context`.

The bytecode also supports a symbol table. This can be used to store named values and retrieve them as part of the
execution of the bytecode. The symbol table also contains function pointers for each of the built-in function and
function packages. Calling functions is managed by the bytecode, but can be used to call a function provided
by the caller as native Go code.

## Example
Here is a trivial example of generating bytecode and executing it.

    
    // Create a ByteCode object and write some instructions into it.
    b := bytecode.New("sample program")
    b.Emit(bytecode.Load, "strings")
    b.Emit(bytecode.Member, "left")
    b.Emit{bytecode.Push, "fruitcake")
    b.Emit(bytecode.Push, 5)
    b.Emit(bytecode.Call, 2)
    b.Emit(bytecode.Stop)

    // Make a symbol table, so we can call the function library.
    s := symbols.NewSymbolTable("sample program")
    functions.AddBuiltins(s)

    // Make a runtime context for this bytecode, and then run it.
    // The context has the symbol table and bytecode attached to it.
    c := bytecode.NewContext(s, b)
    err := c.Run()

    // Retrieve the last value and extract a string 
    v, err := b.Pop()
    fmt.Printf("The result is %s\n", util.GetString(v))

This creates a new bytecode stream, and then adds instructions to it. These instructions would nominally
be added by a parser. The `Emit()` function emits an instruction with only one value, the opcode. The
`Emit()` method emits an instruction with two values, the opcode and an arbitrary operand value.

The stream puts arguments to a function on a stack, and then calls the function. The
result is left on the stack, and can be popped off after execution completes. The result (which is always
an abstract interface{}) is then converted to a string and printed.

## ByteCodes
This table enumerates the bytecode values in the `bytecode` package, and what they do.

| Opcode              | Description |
|:--------------------|:------------|
| Stop                | Stop execution of the current bytecode stream |
| AtLine &lt;int&gt;        | Record the current line number from the source file. This is used for forming error messages and debugging. |
| Push &lt;any&gt;          | Push a scalar (int, float64, string, or bool) value directly onto the stack. |
| Drop &lt;int&gt;          | Remove the specified number of items from the top of the stack and discard them. |
| Add                 | Remove the top two items from the stack and add s[0] to s[1] together and push the result back on the stack. |
| Sub                 | Remove the top two items from the stack and subtract s[0] from s[1] and push the result back on the stack |
| Div                 | Remove the top two items from the stack and divide s[0] by s[1] and push the result back on the stack |
| Mul                 | Remove the top two items from the stack and multiply s[0] by s[1] and push the result back on the stack. |
| And                 | Remove the top two items from the stack and Boolean AND them together, and push the result back on the stack. |
| Or                  | Remove the top two items from the stack and Boolean OR them together and push the result back on the stack. |
| Negate              | Remove the top item from the stack and push the negative (or Boolean NOT) of the value back on the stack. |
| Equal               | Remove the top two items and push a boolean expressions s[0] = s[1]. |
| NotEqual            | Remove the top two items and push a boolean expressions s[0] != s[1]. |
| GreaterThan         | Remove the top two items and push a boolean expressions s[0] &gt; s[1]. |
| LessThan            | Remove the top two items and push a boolean expressions s[0] &lt; s[1]. |
| GreaterThanOrEqual  | Remove the top two items and push a boolean expressions s[0] &gt;= s[1]. |
| LessThanOrEqual     | Remove the top two items and push a boolean expressions s[0] &lt;= s[1]. |
| Load  &lt;string&gt;      | Load the named value from the symbol table and push it on the stack. |
| Store &lt;string&gt;      | Remove the top item from the stack and store it in the symbol table using the given name. |
| Array &lt;int&gt;         | Remove the specified number of items from the stack and create an array with those values, and push it back on the stack. |
| MakeArray           |
| LoadIndex           | Remove the top item and use it as an index into the second item which must be an array, then push the array element back on the stack. |
| StoreIndex          | Remove the top item and use it as an index into the second item which must be an array, and store the third item into the array. |
| Struct &lt;int&gt;        | Remove the given number of _pairs_ of items. The first item must be a string, and becomes the field with the second item as its value. The resulting struct is pushed back on the stack. | 
| Member              | Remove the top item and use it as a field name into the second item which must be a struct, and store the third item into the struct. |
| Print               | Remove the top item from the stack and print it to the console. |
| Newline             | Print a newline character to the console. |
| Branch  &lt;addr&gt;      | Transfer control to the instruction at the given location in the bytecode array. |
| BranchTrue &lt;addr&gt;   | Remove the top item. If it is true, transfer control to the instruction at the given location in the bytecode array. |
| BranchFalse &lt;addr&gt;  | Remove the top item. If it is false, transfer control to the instruction at the given location in the bytecode array. |
| Call &lt;int&gt;          | Remove the given number of items from the stack to form a parameter list. The remove the pointer to the function. This can be a pointer to a native function or a pointer to a `bytecode` structure containing a function written in the _Ego_ language. |
| Return &lt;bool&gt;       | Return from a function. If the boolean value is true, then a return code is also popped from the stack and
passed to the caller's context. |
| SymbolCreate &lt;string&gt; | Create a new symbol in the most-local table of the given name |
| SymbolDelete &lt;string&gt; | Delete the symbol from the nearest scope in which it exists |
| Template &lt;string&gt; | Compile the template on top of the stack, and store in the persisted template store under the &lt;string&gt; name. |
