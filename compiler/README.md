# compiler

The `compiler` package is used to compile text in the _Ego_ language into
bytecode that can be executed using the `bytecode` package. This allows for
compiled scripts to be integrated into the application, and run repeatedly
without incurring the overhead of parsing and semantic analysis each time.

The _Ego_ language is loosely based on _Go_ but with some important
differences. Some important attributes of _Ego_ programs are:

* There are no pointer types, and no dynamic memory allocation.
* All objects are passed by value in function calls.
* Variables are untyped, but can be cast explicitly or will be type converted
automatically when possible.

The program stream executes at the topmost scope. You can define one or more
functions in that topmost scope, or execute commands directly. Each function
runs in its own scope; it can access variables from outer scopes but cannot
set them. Functions defined within another function only exist as long as
that function is running.

## Example
Here is a trivial example of compiling and running some _Ego_ code in your
Go program.

    // String containing arbitrary _Ego_ statements.
    src := "..."

    bc, err := compiler.CompileString(src)
    if !errors.Nil(err) {
        // Handle compile-time errors
    }

    syms := symbols.NewSymbolTable("test program")
    ctx := bytecode.NewContext(syms, bc)
    err := ctx.Run()
    if !errors.Nil(err) {
        // Handle run-time errors
    }

The general pattern is to pass a string containing the program text
 to the compiler. The compiler generates a bytecode object containing
the pseudocode for the program and any predefined symbols (constants or
functions) from the compilation.

The caller then creates a new symbol table (or can re-use an existing one
if symbols are meant to be persistent between compilation units). A new
runtime context is created (which contains the program counter, stack,
error handling stack, etc.) and uses the existing symbol table and 
bytecode. This allows bytecode to be persisted or re-used and can be
executed multiple times on multiple threads, each with it's own context.

Finally, the context is run, which executes the bytecode instructions.
If the instructions are meant to return a value, that value is left on
the stack for the context, and you can use `ctx.Pop()` to remove items
from the stack. The return values are opaque `interface{}` objects, and
you can use the util.Get*() functions to extract the integer, float64,
string, or bool object.

## Data types
_Ego_ support six data types, plus limited support for function pointers
as values.

| type | description |
|:-:|:-|
|int| 64-bit integer value |
|float64| 64-bit floating point value |
|string| Unicode string |
|bool| Boolean value of true or false |
| [...] | Array of values |
| {...} | Structure of fields |

An array is a list of zero or more values. The array values can be of any
type, including other arrays. The array elements are always indexed starting
at a value of zero. You can also reference a range (slice) of an array by using
the notation `a[b:e]` which returns an array containing elements from `b` to
`e` from array `a`. You cannot reference a subscript that has not been allocated.
You can use the `array` statement to initialize an array, and the `array()`
function to change the size of an existing array.

You can also define an array constant, and assign it to a value, where
the initial array is defined by the constant.

    names := [ "Tim", "Sue", "Bob", "Robin" ]

This results in `names` being assigned an array containing the four string
values given.

Structures contain zero or more labeled fields. Each field label must be
unique, and can reference a value of any type. You can create a struct
using a literal:

    employee := { name: "Susan", rate: 23.50, active:true }

This creates a structure with three members (`name`, `rate`, and `active`).
Once this is stored in a variable, you can use dot-notation to reference
a field directly,

    pay := 40 * employee.rate

If the member does not exist, an error is generated. You can however add
new fields to a structure simply by naming them in an assignment, such as

    employee.weekly = pay

If there isn't already a field named `weekly` in the structure, it is
created automatically. The field is then set to the value of `pay`.

Note that structures and arrays are always passed around by reference.
That is, if you create a structure `a`, assign it to `b`, and then change
a field in `b`, it will also be changed in `a`. To make an exact duplicate
of an existing structure, use the `new()` function which creates a new
instance of the existing type. For example,

     a := { age: 55, name: "Timmy"}
     b := a
     b.age = 4
     fmt.Println( a.age )   // Prints the value 4

     c := { age: 55, name: "Timmy"}
     d := new(c)
     d.age = 4
     fmt.Println( c.age )    // Prints the value 55


## Scope
The console input (when `ego` is run with no arguments or parameters) or
the source file named when `ego run` is used creates the main symbol table,
available to any statement entered by the user or in the source file.

The first time a symbol is created, you must use the := notation to create
a variable as well as set its value. All subsequent sets of that variable
should use the = notation to store in an existing symbol.

Whenever a `{ }` block is used, a new symbol table is created for use during
that block. Any symbols created within the block are deleted when the block
exits. The block can of course reference symbols in outer blocks using 
standard "=" notation. For example,

    x := 55
    {
        y := 66
        x = 42
    }

After this code runs, the value of `x` will be 42 (because it was changed
within the block) and the symbol `y` will not be defined (because it went
out of scope at the end of the basic block.)


## array
The `array` statement is used to allocate an array. An array can also be
created as an array constant and stored in a variable. The array statement
identifies the name of the array and the size, and optionally an initial
value for each member of the array.

    array x[5]
    array y[2] = 10

The first example creates an array of 5 elements, but the elements are
`<nil>` which means they do not have a usable value yet. The array elements
must have a value stored in them before they can be used in an expression.
The second example assigns an initial value to each element of the array,
so the second statement is really identical to `y := [10,10]`.

## const
The `const` statement can define constant values in the current scope. These
values are always readonly values and you cannot use a constant name as a
variable name. You can specify a single constant or a group of them; to specify
more than one in a single statement enclose the list in parenthesis:

    const answer = 42

    const (
        first = "a"
        last = "z"
    )

This defines three constant values. Note that the value is set using an
`=` character since a symbols is not actually being created.

## if
The `if` statement provides conditional execution. The statement must start
with a expression which can be cast as a boolean value. That value is
tested; if it is true then the following statement (or statement block)
is execued. By convention, even if the conditional code is a single
statement, it is enclosed in a statement block. For example,

    if age >= 50 {
        call aarp(name)
    }

This tests the variable age to determine if it is greater than or
equal to the integer value 50, and if so, it calls the function 
named `aarp` with the value of the `name` symbol.

You can optionally include an "else" clause to execute if the
condition is false, as in 

    if flag == "-d" {
        call debug()
    } else {
        call regular()
    }

If the value of `flag` does not equal the string "-d" then the 
code will call the function `regular()` instead of `debug()`.

## func
The `func` statement defines a function. This must have a name
which is a valid symbol, followed by an argument list. 

The argument list is a list of names which become local variables in the running
function, set to the value of the arguments from the caller. After each argument
name in the `func` statement, you can specify a type of `int`, `string`, `float64`, 
`bool`, `struct`, or `array`, in which case the value is
coerced to that type regardless of the value passed into the function.

After the  (possibly empty)  argument list you must specify the type of the
function's return value. This can be one of the base
types (`int`, `float64`, `string`, or `bool`). It can also be `[]` which
denotes a return of an array type, or `{}` which denotes the return
of a `struct` type. Finally, the type can be `any` which means any
type can be returned, or `void` which means no value is returned from
this function (it is intended to be invoked as a `call` statement).

The type declaration is then followed by
a statement or block defining the code to execute when the function
is used in an expression or in a `call` statement. For example,

    func double(x) float64 {
        return x * 2
    }

This accepts a single value, named `x` when the function is running.
The function returns that value multiplied by 2. The type of the result
is coerced to be a float64 value. Note that the braces are not required
in the above example since the function consists of a single `return`
statement, but by convention braces are always used to indicate the
body fo the function. The function just created can
then be used in an expression, such as:

    fun := 2
    moreFun := double(fun)

After this code executes, `moreFun` will contain the value 4.0 as a
float64 value.

## return
The `return` statement contains an expression that is identified as
the result of the function value. The generated code adds the value
to the runtime stack, and then exits the function. The caller can
then retrieve the value from the stack to use in an expression or
statement.
    
    return salary/12.0

This statement returns the value of the expression `salary/12.0` as
the result of the function.

If you use the `return` statement with no value, then the function
simply stops without leaving a value on the arithmetic stack. This is
the appropriate behavior for a function that is meant to be invoked
with a `call` statement.

## for
The `for` statement defines a looping construct. A single statement
or a statement block is executed based on the definition of the 
loop. There are two kinds of loops.

    x := [101, 232,363]
    for n:=0; n < len(x); n = n + 1 {
        fmt.Printf("element %d is %d\n", n, x[n])
    }

This example creates an array, and then uses a loop to read all
the values of the array. The `for` statement is followed by three
clauses, each separated by a ";" character. The first clause must
be a valid assignment that initializes the loop value. The second
clause is a condition which is tested at the start of each loop;
when the condition results in a false value, the loop stop 
executing. The third clause must be a statement that updates the
loop value.  This is followed by a block containing the statement(s)
to execute each time through the loop.

When using a loop to index over an array, you can use a short
hand version of this.

    x := [ 101, 232, 363 ]
    for n := range x {
        fmt.Println( "The value is ", n)
    }

In this example, the value of `n` will take on each element of
the array in turn as the body of the loop executes. You can
have the `range` option give you both the index number and
the value.

    x := [ 101, 232, 363 ]
    for i, n := range x {
        fmt.Println( "Element ", i, " is ", n )
    }

Here, the array index is stored in `i` and the value of
the array index is stored in `n`. This is symantically
identical to the following more explicit loop structure:

    for i := 1; i <= len(x); i = i + 1 {
        n := x[i]
        fmt.Println( "Element ", i, " is ", n )
    }

## break
The `break` statement exits from the currently running loop, as if the
loop had terminated normally.

    for i := 0; i < 10; i = i + 1 {
        if i == 5 {
            break
        }
        fmt.Println( i )
    }

This loop run run only five times, printing the values 0..4. On the next
iteration, because the index `i` is equal to 5, the loop is terminated.
Note that a `break` will only exit the current loop; if there are nested
loops the `break` only exits the loop in which it occurred and all outer
loops continue to run.

The `break` statement cannot be used outside of a `for` loop.

## continue
The `continue` statement exits from the current iteration of the loop, 
as if the loop had restarted with the next iteration.

    for i := 0; i < 10; i = i + 1 {
        if i == 5 {
            continue
        }
        fmt.Println( i )
    }

This loop run run only all ten times, but will only output the values
0..4 and 6..9. When the index `i` is equal to 5, the loop starts again
at the top of the loop with the next index value.

The `continue` statement cannot be used outside of a `for` loop.

## Error handling
You can use the `try` statement to run a block of code (in the same
scope as the enclosing statement) and catch any runtime errors that
occur during the execution of that block. The error causes the code
to execute the code in the `catch` block of the statement.
If there are no errors,  execution continues after the catch block.

    x := 0
    try {
        x = pay / hours
    } catch {
        fmt.Println( "Hours were zero!" )
    }
    fmt.Println( "The result is ", x )

If the value of `hours` is non-zero, the assignment statement will assign
the dividend to `x`. However, if hours is zero it will trigger a runtime
divide-by-zero error. When this happens, the remainder of the statements
(if any) in the `try` block are skipped, and the `catch` block is executed.
Within this block, there is a variable `_error_` that is set to the
value of the error that was signalled. This can be used in the `catch`
block if it needs handle more than one possible error, for example.

## package
Use the `package` statement to define a set of related functions in 
a package in the current source file. A give source file can only
contain one package statement and it must be the first statement.

    package factor

This defines that all the functions and constants in this module will
be defined in the `factor` package, and must be referenced with the 
`factor` prefix, as in

    y := factor.intfact(55)

This calls the function `intfact()` defined in the `factor` package.

## import
Use the `import` statement to include other files in the compilation
of this program. The `import` statement cannot appear within any other
block or function definition. Logically, the statement stops the
current compilation, compiles the named object (adding any function
and constant definitions to the named package) and then resuming the
in-progress compilation.

    import factor
    import "factor"
    import "factor.ego"

All three of these have the same effect. The first assumes a file named
"factor.ego" is found in the current directory. The second and third
examples assume the quoted string contains a file path. If the suffix
".ego" is not included it is assumed.

If the import name cannot be found in the current directory, then the
compiler uses the environment variables EGO_PATH to form a directory
path, and adds the "lib" directory to that path to locate the import.
So the above statement could resolve to `/Users/cole/ego/lib/factor.ego`
if the EGO_PATH was set to "~/ego".

Finally, the `import` statement can read an entire directory of source
files that all contribute to the same package. If the target of the
import is a directory in the $EGO_PATH/lib location, then all the
source files within that directory area read and processed as part
of one package. 

## @error
You can generate a runtime error by adding in a `@error` directive, 
which is followed by a string expression that is used to formulate
the error message text. 

    v = "unknonwn"
    @error "unrecognized value: " + v

This will result in a runtime error being generated with the error text
"unrecognized value: unknown". This error can be intercepted in a try/catch
block if desired.

## @global
You can store a value in the Root symbol table (the table that is the
ultimate parent of all other symbols). You cannot modify an existing
readonly value, but you can create new readonly values, or values that
can be changed by the user.

    @global base "http://localhost:8080"

This creates a variable named `base` that is in the root symbol table,
with the value of the given expression. If you do not specify an expression,
the variable is created as an empty-string.

## @template
You can store away a named Go template as inline code. The template
can reference any other templates defined. 

    @template hello "Greetings, {{.Name}}"

The resulting templates are available to the template() function,
 whose first parameter is the template name and the second optional
 parameter is a record containing all the named values that might
 be substituted into the template.  For example,

     fmt.Println( strings.template(hello, { Name: "Tom"}))

This results in the string "Greetings, Tom" being printed on the
stdout console. Note that `hello` becomes a global variable in the program, and
is a pointer to the template that was previously compiled. This
global value can only be used with template functions.

