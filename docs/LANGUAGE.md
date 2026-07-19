# The _Ego_ Language

Version 1.8

This document describes the language _Ego_, which is a scripting
language and tool set patterned off of the _Go_ programming language.

## Table of Contents

1. [Introduction](#intro)

1. [Data Types](#datatypes)
    1. [Base Types](#basetypes)
    2. [Arrays](#arrays)
    3. [Structures](#structures)
    4. [Maps](#maps)
    5. [Pointers](#pointers)
    6. [User Types](#usertypes)

1. [Symbols and Expressions](#symbolsExpressions)
    1. [Symbols and Scope](#symbolsScope)
    1. [Constants](#const)
    1. [Operators](#operators)
    1. [Type Conversion](#typeConversion)
    1. [Builtin Functions](#builtInFunctions)
    1. [Type Assertions and Unwrapping](#typeAssertions)

1. [Conditional and Iterative Execution](#flow-control)
    1. [If/Else Conditional](#if)
    2. [Switch](#switch)
    3. [For &lt;condition&gt;](#for-conditional)
    4. [For &lt;index&gt;](#for-index)
    5. [For &lt;range&gt;](#for-range)
    6. [Break and Continue](#break-continue)

1. [User Functions](#user-functions)
    1. [The `func` Statement](#function-statement)
    2. [The `return` Statement](#return-statement)
    3. [The `defer` Statement](#defer-statement)
    4. [Function Variables](#function-variables)
    5. [Function Receivers](#function-receivers)

1. [Error Handling](#errors)
    1. [Try and Catch](#try-catch)
    2. [Signalling Errors](#signaling)
    3. [`throw`](#throw)

1. [Threads](#threads)
    1. [Go Routines](#goroutine)
    2. [Channels](#channels)

1. [Packages](#packages)
   1. [The `import` statement](#import)
   1. [`base64` package](#base64)
   1. [`cipher` package](#cipher)
   1. [`cmplx` package](#cmplx)
   1. [`errors` package](#errors)
   1. [`exec` package](#exec)
   1. [`filepath` package](#filepath)
   1. [`fmt` package](#fmt)
   1. [`http` package](#http)
   1. [`i18n` package](#i18n)
   1. [`io` package](#io)
   1. [`json` package](#json)
   1. [`math` package](#math)
   1. [`os` package](#os)
   1. [`rest` package](#rest)
   1. [`sort` package](#sort)
   1. [`sql` package](#sql)
   1. [`strconv` package](#strconv)
   1. [`strings` package](#strings)
   1. [`sync` package](#sync)
   1. [`reflect` package](#reflect)
   1. [`runtime` package](#runtime)
   1. [`tables` package](#tables)
   1. [`util` package](#util)
   1. [`uuid` package](#uuid)

1. [User Packages](#packages)
   1. [The `package` statement](#package)

1. [Directives](#directives)
   1. [@capture](#at-capture)
   1. [@compile](#at-compile)
   1. [@error](#at-error)
   1. [@global](#at-global)
   1. [@localization](#at-localization)
   1. [@optimizer](#at-optimizer)
   1. [@template](#at-template)
   1. [@type](#at-type)

1. [Testing](#testing)
   1. [The `test` command](#at-test)
   2. [The `@test` directive](#at-test)
   3. [The `@assert` directive](#at-assert)
   4. [The `@fail` directive](#at-fail)
   5. [The `@pass` directive](#at-pass)

&nbsp;
&nbsp;

## Introduction to _Ego_ Language <a name="intro"></a>

The _Ego_ language name is a portmanteau for _Emulated Go_. If you
already know Go, you will feel at home immediately: the syntax, data
types, and control-flow statements are intentionally close to Go.
The sections below call out the most important differences so you
know where to watch out.

### Key differences from Go <a name="go-differences"></a>

| Go | Ego |
| :--- | :--- |
| Statically typed by default | **Dynamically typed by default** — variables take the type of the value assigned to them. Static typing can be enabled via a setting. |
| `panic` / `recover` for runtime errors | **`try` / `catch`** — a simpler, optional error-handling model. This is an Ego language extension (not standard Go). |
| Full standard library | **Subset of packages** — only a curated set of packages is available (see the Packages section). |
| `int` may be 32 or 64 bits depending on platform | In all current Ego ports **`int` is always 64-bit** (`int64`). |
| Array literals use `{}` | Ego also allows **`[]`** for anonymous (untyped) array literals. |
| Errors are returned values | Ego follows the same pattern, using `errors.New()` to create error values and `panic()` (an extension keyword) to signal runtime errors. |
| `string(65)` produces `"A"` (Unicode rune) | In Ego, **`string(int)` produces the decimal string** `"65"`. To convert a rune to a string, use `string([]int{65})` or `fmt.Sprintf("%c", 65)`. |
| Missing map key returns the zero value of the value type | In Ego, **a missing map key always returns `nil`**, regardless of the declared value type. Use the two-value form `v, ok := m[key]` to distinguish missing from present. |
| `var p *T` declares a nil pointer of type `*T` | In Ego, **`var p *T` produces the zero value of `T`**, not a nil pointer. Use `p := &x` to obtain a real pointer, or return `nil` explicitly from a function with pointer return type. |
| Go does not allow nested named function declarations inside another function | Ego **allows nested named functions**, but they do **not** capture the enclosing function's scope. A nested named function can only access its own parameters, its own locals, and package/global names — not the enclosing function's parameters or locals. Use a function literal (closure) when you need the enclosing scope. |
| An untyped constant adapts to the type it's combined with, but the compiler statically (at compile time) rejects one that would overflow or truncate (e.g. `var w int32 = 5; w = 3.7` is a compile error) | Ego's `strict` mode gives a constant the same "adapts to context, but losslessly" rule, consistently at all four coercion boundaries (assignment, expressions, function arguments, and return values — see [Type Conversions](#typeConversion)) — but since Ego doesn't evaluate constant expressions at compile time, the rejection happens as a catchable **runtime** error instead of a compile error. |

In addition, _Ego_ offers a conditional expression shorthand (`?expr : default`) for supplying
a fallback value when an expression would otherwise produce an error — this has no Go equivalent.

### Running Ego programs

The _Ego_ language is run using the `ego` command-line interface. This
provides the ability to run a program from an external text file, to
interactively enter _Ego_ programming statements like a REPL, or to run _Ego_
programs as web services. This functionality is documented elsewhere;
this guide focuses on writing _Ego_ programs regardless of the runtime
environment.

The _Ego_ language is Copyright 2020-2026 by Tom Cole, and is freely
available for any use, public or private, including commercial software.
The only requirement is that any software that incorporates or makes use
of _Ego_ or the packages written by Tom Cole to support _Ego_ must
include a statement attributing authorship to _Ego_ and its runtime
environment to Tom Cole.

&nbsp;
&nbsp;

## Data Types<a name="datatypes"></a>

The _Ego_ language supports a number of base types which express a
single value of some type (string, integer, boolean, etc.). These base
types can be members of complex types consisting of arrays (ordered
lists), maps (dynamic types key/value pairs) and structs (field-name/value
pairs). Additionally, the user can create types based on the base or complex
types, such as a type describing a structure that records information
about an employee; this type can be used to create instances of the structure, etc.

### Base Types<a name="basetypes"></a>

A value can be a base type; when it is a base type is contains only
one value at a time, and that value has a specific type. These are
listed here.

&nbsp;

| Type | Example | Range | Description |
| :---------- | :-------- | :--------------------- | :------------ |
| `nil` | nil | nil | The `nil` value indicates no value or type specified |
| `bool` | true | true, false | A Boolean value that is either true or false |
| `byte` | 5 | 0 to 255 | An 8-bit unsigned integer (alias for `uint8`) |
| `uint8` | 200 | 0 to 255 | An unsigned 8-bit integer (same as `byte`) |
| `int8` | -3 | -128 to 127 | A signed 8-bit integer |
| `int16` | -1025 | -32768 to 32767 | A signed 16-bit integer |
| `uint16` | 17000 | 0 to 65535 | An unsigned 16-bit integer |
| `int32` | 1024 | -2147483648 to 2147483647 | A signed 32-bit integer |
| `uint32` | 1553 | 0 to 4294967295 | An unsigned 32-bit integer |
| `int` | -1024 | <varies> | A signed integer |
| `uint` | 12345678 | <varies> | An unsigned integer |
| `int64` | 1573 | -2^63 to 2^63-1 | A 64-bit signed integer value |
| `uint64` | 5505955 | 0 to 18446744073709551615 | An unsigned 64-bit integer value |
| `float32` | -3.14 | -1.79e+38 to 1.79e+38 | A 32-bit floating point value |
| `float64` | -153.35 | -1.79e+308 to 1.79e+308 | A 64-bit floating point value |
| `complex64` | 3+4i | float32 real/imaginary components | A complex number with `float32` real and imaginary parts |
| `complex128` | 3+4i | float64 real/imaginary components | A complex number with `float64` real and imaginary parts |
| `string` | "Andrew" | any | A string value, consisting of a varying number of Unicode characters |
| `chan` | chan | any | A channel, used to communicate values between threads. Ego channels do not take an element type — always write `chan` alone, not `chan string` |

_Note that the numeric range values shown are approximate._ The types `int` and `uint`
are generalized values for "the most efficient integer type for this architecture." In
implementations of _Ego_ for macOS, Windows, and amd64-based Linux systems, the type of
`int` is the same range as the type `int64`, and similarly for the unsigned version.
However, there may be systems where an `int` is a 32-bit value because that is the most
efficient form of integer for that architecture. You can use the _Ego_ function `sizeof()`
to determine the size of a value in bytes to determine the actual range of an `int`
value on your implementation.

&nbsp;

A value expressed in an _Ego_ program has an implied type. The
language processor will attempt to determine the type of the value.
For Boolean values, the value can only be `true` or `false`. For
numeric types, the language differentiates between integer and floating
point values. The value `1573` will be interpreted as an int value because
it has no exponent or factional part, but `-153.35` will be
interpreted as a float64 value because it has a decimal point and a
fractional value. A string value enclosed in double quotes (") cannot
span multiple lines of text. A string value enclosed in back-quotes
(`) are allowed to span multiple lines of text if needed.

In the Go language, `int` is a shortcut for either an `int32` or `int64`,
depending on whichever is the fastest native integer data type. In all
current ports of _Ego_, the `int` type is an `int64` (and a `uint` is
therefore the same size as a `uint64`).

A `chan` value has no constant expression; it is a type that can be
used to create a variable used to communicate between threads. See
the section below on threads for more information.

#### Radix (base) integer literals<a name="radix-literals"></a>

An integer literal can be written in binary, octal, or hexadecimal instead of decimal,
matching Go's notation:

| Prefix | Base | Example | Decimal value |
| :----- | :--- | :------ | :------------ |
| `0b` or `0B` | 2 (binary) | `0b101` | 5 |
| `0o` or `0O`, or a bare leading `0` | 8 (octal) | `0o644`, `0644` | 420 |
| `0x` or `0X` | 16 (hexadecimal) | `0x41` | 65 |

The explicit `0o` prefix and the bare-leading-zero form are interchangeable -- `0o644`
and `0644` are the same value. The bare form only applies when every digit after the
leading zero is a valid octal digit (`0`-`7`); a lone `0` is simply the integer zero, and
a leading zero followed by a decimal point or exponent (`0.5`, `0e5`) is a `float64`
literal, not octal. Radix literals are always integers -- there is no radix notation for
floating-point values (other than the exponent form already shown in the table above).

#### Imaginary literals<a name="imaginary-literals"></a>

An integer or floating-point literal immediately followed by `i`, with no space, is an
imaginary literal -- the imaginary half of a complex number, matching Go's notation:

```go
x := 3i      // complex128(0+3i)
y := 2.5i    // complex128(0+2.5i)
z := 1 + 2i  // complex128(1+2i), via ordinary addition
```

An imaginary literal always compiles to `complex128`, regardless of magnitude -- the
same way a plain floating-point literal always defaults to `float64` rather than
`float32`. The `i` must be immediately adjacent to the number with no space; `3 i` (with
a space) is two separate tokens -- the integer `3` followed by an identifier named `i` --
not an imaginary literal, matching Go's grammar exactly.

Unlike Go, which requires an explicit conversion (e.g. `complex128(5)`) to use a real
number where a complex value is expected, _Ego_ coerces real numbers (any integer or
floating-point type) to `complex64`/`complex128` implicitly at assignment and call
boundaries, with the imaginary part set to zero -- consistent with how _Ego_ already
coerces other scalar types more liberally than Go does. The reverse direction is never
implicit: extracting the real or imaginary component of a complex value always requires
an explicit call to `real()` or `imag()` (see [Builtin Functions](#builtInFunctions)),
matching Go's own strictness in that direction. Dividing by a complex zero value (`c /
(0+0i)`) raises a runtime error rather than producing Go's native IEEE Inf/NaN result,
consistent with how dividing a `float64` by `0` already behaves in _Ego_.

See the [`cmplx` package](#cmplx) for complex-number math functions (`Abs`, `Sqrt`,
`Polar`, and so on), and [`strconv.ParseComplex`/`FormatComplex`](#strconv) for parsing
and formatting complex values as strings.

### Arrays<a name="arrays"></a>

An array is an ordered list of values. That is, it is a set where each
value has a numerical position referred to as its index. The first
value in an array has an index value of 0; the second value in the
array has an index of 1, and so on. An array has a fixed size; once it
is created, you cannot add to the array directly.

Array constants can be expressed using square brackets, which contain a
list of values separated by commas. The values may be any valid value
(base, complex, or user types). The values do not have to be of the
same type. For example,

```go
    [ 101, 335, 153, 19, -55, 0 ]
    [ 123, "Fred", true, 55.738]
```

The first example is an array of integers. The value at position 0 is
`101`. The value at position 1 is `335`, and so on. The second
example is a heterogenous array, where each value is of varying types.
For example, the value at position 0 is the integer `123` and the
value at position 1 is the string `"Fred"`.

These kinds of arrays are _anonymous_ arrays, in that they have no
specific type for the values. In Go parlance, these are implemented
as arrays of type `any`. You can also specify a type for the array
using a typed array constant. For example,

```go
a := []int{101, 102, 103}
```

In this example, an array is created that can only contain `int` values.
If you specify a value in the array initialization list that is not an
`int`, Ego's behavior depends on what type strictness it is running under.

| Mode | Behavior |
| ---- | -------- |
| dynamic | The array becomes an array of `[]interface`, which an array of "any type" |
| relaxed | The value is converted automatically to the `int` type |
| strict | An error is reported if the value is not `int` |

When in `relaxed` mode, if the value cannot be converted, then a runtime
error is generated (for example, trying to store "donut" in a `[]int` array
is an error because "donut" cannot be converted to an `int` value, but storing
"153" in the `[]int` array will work because "153" can be unambiguously
converted to an integer.)

Assuming Ego is running in `strict` mode, the following outcomes would
be expected:

```go
a[1] = 1325    // Succeeds
a[1] = 1325.0  // Failed, must be of type int
```

### Structures<a name="structures"></a>

A structure (called `struct` in the _Ego_ language) is a set of
named fields with values for each field. The field name is an
_Ego_ identifier, and the value is any supported value type.
Each field name must be unique. The values can be read or written
in the `struct` based on the key name. Once a `struct`
is created, it cannot have new fields added to it. A `struct`
constant is indicated by braces, as in:

```go
{  Name: "Tom", Age: 53 }
```

This `struct` has two fields, `Name` and `Age`. Note that the
field names are case-sensitive. The `struct` member `Name` is
a string value, and the `struct` member `Age` is an int value.

This type of `struct` is known as an _anonymous_ struct in that it
does not have a specific type, and in fact the fields are all
declared as type `any` so they can hold any arbitrary values
unless strict type checking is enabled.

You cannot add new fields to a `struct` if you create it using
a `struct` constant with fields. That is, you cannot

```go
a := { Name: "Bob" }
a.Age = 43
```

The second line will generate an error because Age is not a member
of the structure. There is one special case of an _anonymous_
`struct` that can have fields added (or removed) dynamically. This is
an empty _anonymous_ `struct`,

```go
a := {}
a.Name = "Fred"
a.Gender = "M"
```

The empty anonymous `struct` can have fields added to it just by
naming them, and they are created as needed. In the above example,
the anonymous `struct` named `a` is created using a constant with
no fields declared. This is followed by naming field values that
do not already exist. These fields are added to the `struct`
dynamically. You can also delete a field using the `delete()`
function:

```go
a := {}
a.Name = "Fred"
a.Age = 55
delete(a, "Age")
```

The delete operation names the field as a string expression "Age",
and the associated field is deleted from the `struct` value. This
operation can _only_ be done on _anonymous_ structs that were
created using the `{}` empty struct literal. Additionally, the
field name given in `delete()` must exist, or an error is
generated.

### Maps<a name="maps"></a>

A `map` in the _Ego_ language functions the same as it does in Go. A
`map` is declared as having a key type and a value type, and internally,
a hashmap is constructed based on that information. You can set a value
in the map and you can read a value from the map.

You can create an initialized map by setting a value to an empty map constant.
An initialized map has internal data structure initialized so it is ready
to have values added to it. For example,

```go
staff := map[int]string{}
```

Alternatively you can declare the map be expressing it's type and then
using the `make()` function to initialize it:

```go
var staff map[int]string

staff = make(map[int]string)
```

Either form creates a map (stored in `staff`) that has an integer
value as the key, and stores a string value for each unique key. A
map can contain only one key of a given value; setting the key
value a second time just replaces the value of the map for that key.

You can also initialize the map values using a map constant, which is
a list of key/value pairs:

```go
staff := map[int]string{101:"Jeff", 102:"Susan"}

staff[103] = "Buddy"
staff[104] = "Donna"
```

This adds members to the map. Note that the key _must_ be an integer
value, and the value _must_ be a string value because that's how the
map was declared. Unlike a variable, a map always has a static definition
once it is created and cannot contain values of a different type.
Attempting to store a boolean in the map results in a runtime error.

You can read a value from the may by using `[]` notation to express
the key value. The key value must match the map declaration. for example,

```go
id := 102
name := staff[id]
```

This uses an integer variable to retrieve a value from the map. In this
case, the value of `name` will be set to "Susan". If there is nothing
in the map with the given key, the value of the expression is `nil`.

You can test for the existence of an item when you attempt to read it
from the map. In this notation, the second value returned in the assignment
is a boolean that indicates if the item was found or not.

```go
emp, found := staff[105]
```

In this example, `found` will be true if there was a value in the map for
the key value `105`, and the value of the map item (a string in this case)
will be stored in `emp`. If there was no value in the map for the given key,
`found` will be set to false, and `emp` will be nil. (Note that this is
slightly different than traditional Go, where the result would be the zero
value for the type, i.e. an empty string in this case).

You can delete a member from a map using the delete() function, such as:

```go
delete(staff, 103)
```

This is delete an entry from the map `staff` with a key value of `103`. If
the member does not exist, no error is thrown.

### Pointers<a name="pointers"></a>

The _Ego_ language adopts the Go standards for pointers. Pointers exist
solely to identify the address of another object. This address can be
passed across function boundaries to allow the function to modify the
value of a parameter.

No pointer arithmetic is permitted; a pointer can only be set as the
address of another variable.

```go

var x *int                      // (1)

y := 42
x = &y                         // (2)

fmt.Println(*x)                 // (3)
```

In this example,

1. A variable `x` is created as a pointer to an integer
value. Note that in Ego (unlike Go), `var x *int` does not produce a
nil pointer — it produces the zero value for `int` (i.e., `0`). To obtain
a real nil pointer, return `nil` explicitly from a function or assign `nil`
directly: `x = nil`.

2. The value of `x` is now set to a non-nil value; it becomes the
address of the variable `y`. From this point forward (until the
value of `x` is changed again) you can reference the value of `y`
using either the symbol `y` or by dereferencing the pointer to `y`
stored in `x`.

3. This shows dereferencing the pointer value to access the underlying
value of `42` as the output of the print operation. If you had printed
the value `x` rather than `*x`, it would print the string `&42` to show
that the value points to `42`.

The above examples illustrate basic functions of a pointer, but the
most common case is as a return value from a function or as a function
parameter.

```go
func hasPositive( x int ) *int {
    if x >= 0 {
        return &x
    }
    return nil
}

v := hasPositive(55)
if v == nil {
    fmt.Println("Not positive; no value returned")
}
```

In this somewhat contrived example, the function `hasPositive` does not
return an integer, it returns a pointer to an integer. The logic of the
function is such that if a positive value was given, it is returned,
else a nil value is returned as the pointer value indicating _no value_
returned from the function.

As a final example, you can use pointers to allow a function to modify
a value.

```go
func setter( destination *int, source int) {
    *destination = source
}

x := 55
setter(&x, 42)
fmt.Println(x)
```

In this example, the function `setter` is given the address of an integer
and a value to store in that integer. Because the value is passed by
pointer, the value of `destination` is the address of the value `55`. The
`setter` function overwrites that with the parameter passed (in this case,
the value `42`). The result is that the value of x has now been changed
by the function, and the value printed will be "42".

This is the only way a function can change a parameter value. By default,
a value (such as `source` in the example above) gets a copy made and that
copy is what is passed to the function. If the `setter` function had
modified the value of `source`, then the value would be different in the
copy local to the function, but the global value (`42`, in this case) would
not have changed.

### User Types<a name="usertypes"></a>

The _Ego_ language includes the ability to create user-defined types.
These are limited to `struct` definitions. They allow the program to
define a short-hand for a specific type, and then reference that type
when creating a new variable of that type. The `type` statement is used
to define the type. Here is an example:

```go
type Employee struct {
    Name    string
    Age     int
}
```

This creates a new type called `Employee` which is a struct with
two members, `Name` and `Age`. A variable created with this type
will always be a struct, and will always contain these two members.
You can then create a variable of this type using

```go
e := Employee{}
```

The `{}` indicates this is a type, and a new structure (of type
`Employee`) is created and stored in the variable `e`. You can
initialize fields in the struct when you create it if you wish,

```go
a := Employee{ Name: "Robin" }
```

In this example, a new Employee is created and the `Name` field is
initialized to the string "Robin". The value `a` also contains a
field `Age` because that was declared in the type, but at this
point it contains the zero-value for its type (in this case, an
integer zero). You can only initialize fields in a type that were
declared in the original type.

&nbsp;
&nbsp;

## Variables and Expressions<a name="symbolsExpressions"></a>

This section covers variables (named storage for values) and
expressions (sequences of variables, values, and operators that
result in a computed value).

### Symbols and Scope<a name="symbolsScope"></a>

A variable is a named storage location, identified by a _symbol_.
The _Ego_ language is, by default, a case-sensitive language, such
 that the variables `Age` and `age` are two different values. A
 symbol names can consist of letters, numbers, or the underscore
 ("&lowbar;") character. The first character must be either an underscore
 or a alphabetic character. Here are some examples of valid and
 invalid names:

&nbsp;

| Name | Description |
| :-------- | :---------- |
| a123 | Valid name |
| user_name | Valid name |
| A123 | Valid name, different than `a123` |
| _egg | Valid name, but is a read-only variable |
| 15States | Invalid name, does not start with an alphabetic character |
| $name | Invalid name, dollar-sign is not a valid symbol character |

&nbsp;

There is a reserved name that is just an underscore, "_". This name
means _value we will ignore._ So anytime you need to reference a variable
to conform to the syntax of the language, but you do not want or need
the value for your particular program, you can specify "_" which is a
short-hand for "discard this value".

A symbol name that starts with an underscore character is a read-only
variable. That is, its value can be set once when it is created, but
can not be changed once it has its initial value. For example, when an
 _Ego_ program runs, there is always a read-only variable called
`_version` that can be read to determine the version of the `ego`
command line tool, but the value cannot be set by a user program.

The term _scope_ refers to the mechanism by which a symbol name is
resolved to a variable. When an _Ego_ program runs, each individual
function, and in fact each basic block (code enclosed within `{...}`
braces) has its own scope. A variable that is created at one scope is
_visible_ to any scopes that are contained within that scope. For
example, a variable created at the first line of a function is visible
to all the code within the function. A variable created within a
basic-block is only visible within the code inside that basic block.
When a scope ends, any variables created in that scope are deleted
and they no longer have a value.

This will become clearer below when we talk about functions and basic
blocks. In general, just know that you can reference (read or write)
a value to any symbol in your same scope or an outer scope, but you
can only create variables at the current scope, and once that scope
is completed, the created variables no longer exist. A symbol can be
assigned a value using an _assignment_ statement. This consists of the
variable name that is to receive the value, and either `=` or `:=`,
followed by the value to store in that variable. The difference between
the two assignment operators is that `:=` will create a new value in
the current scope, while `=` will locate a value that already exists,
and write a new value to it.

> **Go developer note:** In Go, `:=` both creates _and_ infers the type,
> which is then fixed for the lifetime of the variable. In Ego's default
> dynamic mode, `:=` creates the variable but its type can change to match
> whatever value you assign to it later with `=`. Enable static typing
> (via the `ego.compiler.types` setting) to get Go-like type discipline.

```go
name := "Bob"
name = "Mary"
```

In this example, the first statement uses the `:=` operator, which
causes the symbol `name` to be created, and then the string value "Bob"
is stored in the variable. If the variable already exists, this is an
error and you should use the `=` instead to update an existing value.
The `=` operator looks for the named value in the current scope, and
if it finds it, the value "Mary" is assigned to it. If the variable
does not exist at this scope, but does in an outer scope level, then
the variable at the outer scope is updated.

#### Parallel Assignment <a name="parallel-assignment"></a>

_Ego_ also supports Go-style _parallel assignment_, where more than one
variable is assigned in a single statement using a comma-separated list of
targets on the left and a matching comma-separated list of values on the
right. Both `=` and `:=` are supported:

```go
var a, b, c int

a, b, c = 10, 20, 30
x, y, z := 1, 2, 3
```

All of the values on the right are evaluated first, in the order written,
before any of the assignments happen. This makes parallel assignment a
convenient way to swap values without a temporary variable:

```go
a, b := 1, 2
a, b = b, a       // a is now 2, b is now 1
```

The number of values on the right must exactly match the number of targets
on the left; a mismatched count is a compile-time error. As with any
assignment, the blank identifier `_` can be used for a target whose value
you don't need:

```go
a, _, c := 1, 2, 3   // the middle value, 2, is discarded
```

Parallel assignment also covers two multi-value forms that already existed
in _Ego_: capturing every value returned by a function that returns more
than one value, and the two-value form of a channel receive. In both cases
the right side is a single expression (not a comma-separated list) that
itself produces multiple values:

```go
func minMax(values []int) (int, int) {
    // ...
    return low, high
}

lo, hi := minMax(data)     // multi-value function return
v, ok := <-ch              // two-value channel receive
```

Note that _Ego_ allows a shortcut for a specific assignment statement
that adds or subtracts the constant `1` from a value. For example,

```go
i = i + 1
i++
```

These statements have the identical function. They require that the
variable `i` already exist, and a value of 1 is added to `i`. The
same thing can be done using the `--` operator to subtract one from
a value. This is commonly used in `for` loops, discussed later.

If _Ego_ language extensions are enabled, the `++` and `--` operators
can be used in an expression value. For example, consider this code:

```go
i := 10
j := 5 + i++
```

This has the effect of setting `j` to the value 15, and setting `i`
to the value 11. That is, the increment operation `++` is performed
on the variable `i` after its value is read in the addition
operation that sets the value of `j`.

```go
t := i
i = i + 1
j := 5 + t
```

The above code has the same effect as the second statement in the
auto-increment expression value above. Using the auto-increment
(or decrement) operation prevents the need to store the pre-increment
value in a temporary variable, and reduces code clutter.

_Ego_ also supports implied operators in the assignment, using the
assignment operators `+=`, `-=`, `*=`, and `/=`. Each of these
performs an assignment that includes the given operation (addition,
subtraction, multiplication, or division) of the value following
the `=` character. For example,

```go
i := 10
i += 5
```

The use of the `+=` operator requires that the value to the left of
the assignment already exists. In this example, the value of i has
the constant value 5 added to it. This is the same as `i = i + 5`
but is easier to read.

You can also create a variable using the `var` statement, which is
followed by a comma-separated list of names and finally a type value.
The variables are all created and set to the given type, with an
appropriate "zero value" for that type.

```go
var first, last string
var e1 Employee
```

The second example declares a variable `e1` of type `Employee`. To create
an initialized instance, use `:=` with the type name and braces:

```go
e1 := Employee{}
```

The `{}` after the type name creates an instance of that type. You can
supply field initializations inside the braces:

```go
e2 := Employee{Name: "Bob", Age: 55}
```

The type of `e2` is `Employee` and it contains initialized values for
the permitted fields. Fields not explicitly named are set to zero
values for their types.

#### `var` with an initial value

A `var` declaration can also supply an initial value instead of getting the
type's zero value. When you do this, you can either state the type
explicitly or let _Ego_ infer it from the value you supply.

```go
var age int = 25       // explicit type
var pi = 3.14          // type inferred from the value (float64)
```

Both statements create a single variable and set it to the given value. The
second form, where the type name is omitted, works exactly like the short
variable declaration `pi := 3.14` — the new variable's type becomes whatever
type the initializer expression evaluates to. This is convenient when the
type is obvious from the value, or when writing the type out would just
repeat what the value already makes clear. Because there is no explicit type
in this form, a composite literal used as the initializer must name its own
type, the same way it would need to on the right side of `:=`:

```go
var e2 = Employee{Name: "Bob", Age: 55}   // Employee names its own type
```

If more than one name shares a single initializer, for example
`var a, b = 42`, every name is set to a copy of that one value (both `a` and
`b` become `42`) rather than requiring one value per name.

Alternatively, a `var` declaration can supply a comma-separated list of
initializers, one value per name, exactly like [parallel
assignment](#parallel-assignment):

```go
var a, b, c = 10, "hello", true   // type inferred separately for each name
var x, y, z int = 1, 2, 3         // explicit shared type, one value each
```

Here each name gets its own value — and, for the type-inferred form, its own
independent type — rather than every name sharing a single duplicated value.
As with parallel assignment, the number of initializer values must exactly
match the number of names; a mismatched count is a compile-time error.

#### Grouped `var` declarations

Multiple `var` declarations can be grouped together inside parentheses,
avoiding the need to repeat the `var` keyword on every line:

```go
var (
    greeting = "hi"          // type inferred (string)
    count    int             // zero value (0)
    rate     float64 = 1.5   // explicit type with initial value
)
```

Each line inside the parentheses is a complete, independent declaration and
may use any of the forms described above — a bare type, a type with an
initializer, or an inferred type with no type token at all. This mirrors how
`const (...)` groups multiple constant declarations together (see
[Constants](#const) below).

### Constants <a name="const"></a>

The `const` statement can define constant values in the current scope.
These values are always readonly values and you cannot use a constant
name as a variable name. You can specify a single constant or a group
of them; to specify more than one in a single statement enclose the
list in parenthesis:

```go
const answer = 42

const (
    first = "a"
    last = "z"
)
```

This defines three constant values. Note that the value is set using an
`=` character since a symbol is not actually being created.

#### iota

Inside a parenthesized `const (...)` group, the pre-declared identifier `iota`
represents a counter that starts at `0` for the first constant in the group
and increases by one for each constant after that:

```go
const (
    Sunday = iota // 0
    Monday        // 1
    Tuesday       // 2
    Wednesday     // 3
    Thursday      // 4
    Friday        // 5
    Saturday      // 6
)
```

Notice that only the first entry, `Sunday`, spells out `= iota`. Any later
entry in the same group that has no `=` of its own simply repeats the
expression from the nearest entry above it that did have one -- so `Monday`
behaves as if it had been written `Monday = iota`, and picks up the next
counter value (`1`). This is why the sequence above counts up automatically
even though `iota` is only mentioned once.

`iota` can also appear as part of a larger expression, and that expression is
what gets repeated for each following entry. A common use of this is building
a set of bit-shifted values, such as byte-size constants:

```go
const (
    _  = iota             // 0 is skipped by assigning it to the blank identifier
    KB = 1 << (10 * iota) // 1 << 10  == 1024
    MB                    // 1 << 20 == 1,048,576
    GB                    // 1 << 30 == 1,073,741,824
)
```

Here every entry after `KB` repeats the whole expression `1 << (10 * iota)`,
not just the `iota` part, with `iota` itself continuing to count up (`1`, `2`,
`3`, ...). The first entry's value is discarded by assigning it to the blank
identifier `_`, a common idiom when the zero value isn't useful on its own.

A few other things to know about `iota`:

- It is only meaningful inside a `const` declaration. Using `iota` anywhere
  else is treated like referencing any other undeclared name, and is a
  compile-time error.
- It is not a reserved word, so it's still fine to use `iota` as an ordinary
  variable or parameter name outside of a `const` declaration.
- Each separate `const` declaration -- parenthesized group or single
  statement -- gets its own counter starting back at `0`. A plain,
  non-parenthesized `const answer = iota` is legal too; it is treated as a
  one-entry group, so `answer` is simply `0`.
- Omitting the `= expr` on an entry is only allowed once at least one earlier
  entry in the same group provided an expression to repeat. The very first
  entry in a group must always include `= expr`.

### Operators<a name="operators"></a>

Operators is the term for language elements that allow you to perform
mathematical or other operations using constant values and
variable values, to produce a new computed value. Some operators can
operate on a wide range of different data types, and some operators
have more limited functionality.

There are _dereference_ operators that are used to access members of
a struct, key values in a map, or index into an array.

&nbsp;

| Operator | Example | Description |
| :------- | :------- | :---------------------------------------------------- |
| . | emp.age | Find the member named `age` in the struct named `emp` |
| [] | items[5] | Find the value at index 5 of the array named `items` |
| {} | emp{} | Create an instance of a struct of the type `emp` |

The `[]` operator can also be used to access a map, by supplying the key value
in the brackets. This key value must be of the same type as the map's declared
key type. So if the map is declared as `map[string]int` then the key must be
of type `string`.

&nbsp;

There are _monadic_ operators which precede an expression and operate
on the single value given.

&nbsp;

| Operator | Example | Description |
| -------- | ------- | :---------- |
| - | -temp | Calculate the negative of the value in `temp` |
| ! | !active | Calculate the boolean NOT of the value in `active` |

&nbsp;

There are _diadic_ operators that work on two values, one of which
precedes the operator and one of which follows the operator.

&nbsp;

| Operator | Example | Description |
| -------- | ------- | :---------- |
| + | a+b | Calculate the sum of numeric values, or concatenate strings; when applied to boolean values, computes the logical AND |
| - | a-b | Calculate the difference of the integer or floating-point values |
| * | a*b | Calculate the product of two numeric values; when applied to two boolean values, computes the logical OR |
| / | a/b | Calculate the division of the numeric values |
| % | a%b | Calculate the remainder of the division operation |
| ^ | 2^n | Calculate `2` to the power `n` |
| & | a&b | Bitwise AND of two integer values |
| \| | a\|b | Bitwise OR of two integer values |
| << | a<<n | Shift `a` left by `n` bits (`n` must be `>= 0`) |
| >> | a>>n | Shift `a` right by `n` bits (`n` must be `>= 0`) |
| && | a&&b | Logical AND of two boolean values (short-circuit: if `a` is false, `b` is not evaluated) |
| \|\| | a\|\|b | Logical OR of two boolean values (short-circuit: if `a` is true, `b` is not evaluated) |

&nbsp;

Note that applying `+` to two boolean values returns their logical AND, and applying
`*` to two boolean values returns their logical OR. This mirrors the mathematical
analogy where addition is union and multiplication is intersection, but it is
counterintuitive. Prefer `&&` and `||` for boolean logic in Ego programs.

For division, integer values will result in the integer value of
the division, so `10/3` will result in `3` as the expression value.
A floating point value retains the fractional value of the conversion,
so `10.0/3.0` results in `3.333333333` as the result.

The modulo operator is only valid on integer types, and the divisor
cannot be zero.

The bit-shift operators `<<` and `>>` operate on integer values and shift
the left operand by the number of bits given in the right operand. The shift
amount must be non-negative; as in Go, shifting by a negative amount is a
runtime error (`negative shift amount`) rather than a shift in the opposite
direction. A left shift accepts an amount up to 64 (shifting a bit entirely
out of a 64-bit value yields `0`), and a right shift accepts an amount up to
63; larger amounts are a runtime error. The result of a shift is a 64-bit
integer.

Expressions can be combined together, and follow normal mathematical
order of precedence (multiplication and division are done before
subtraction and addition). So the expression `a+b*c` will first
multiply `b` and `c`, and then add the product to the value of `a`.
You can use parenthesis to control the order of evaluation, so
`(a+b)*c` will calculate the sum of `a+b` and then multiply that
result by the value in `c`.
&nbsp;
&nbsp;

There are _relational_ operators that work on two values, one of which
precedes the operator and one of which follows the operator. The
result of the operator is always a boolean (`true` or `false`) value
describing the relationship between the two values.

&nbsp;

| Operator | Example | Description |
| :-------: | :---------: | :---------- |
| == | a == b | True if `a` is equal to `b` |
| != | a != b | True if `a` is not equal to `b` |
| &gt; | a &gt; b | True if `a` is greater than `b` |
| &gt;= | a &gt;= b | True if `a` is greater than or equal to `b` |
| &lt; | a &lt; b | True if `a` is less than `b` |
| &lt;= | a &lt;= b | True if `a` is less than or equal to `b` |

&nbsp;
&nbsp;

There are additional expression types that are only enabled when compiler
extensions are permitted. These are _optional_ and _conditional_
expressions.

An _optional_ expression is an expression, that if when computed
causes an error or exception, is replaced with a default value.
Consider the following code:

```go
x := 0
y := 15/x
```

This will cause an error (division by zero) when the expression is
evaluated. However, you can specify a default value for when an error
occurs in an expression using an optional:

```go
x := 0
y := ?(15/x):-1
```

The `?` character indicates the next term or sub-expression is optional.
If in calculating it, an error occurs, the value after the colon is used
instead. In the above example, the value of `y` will be -1 since the
division error occurred and the optional value was used. If the value of
`x` was non-zero, then the expression value `15/x` would have been the
result stored in `y`.

A similar feature exists for using one of two values in a _conditional_
expression that first evaluates an expression, and based on whether
the result of the expression converts to `true` or `false` determines
which value is ultimately used. For example,

```go
wage := 22.50
hours := 40

// Calculate pay as an hourly amount or a fixed salary
pay := if hours >= 0 {wage * hours} else {wage}
```

In this example, because hours is greater than zero, the pay is calculated
as an hourly wage. If the value of hours is a negative number, then the
wage is assumed to be a salary value and the pay is just set to the wage. This
_conditional_ expression value can be part of a larger expression, such as

```go
x := "got a " + if wasCorrect { "true" } else { "false" } + " answer"
```

Depending on the value of the expression `wasCorrect`, the variable `x` will
contain either "got a true answer" or "got a false answer".

### Type Conversions<a name="typeConversion"></a>

When an operation is done on two values (either a variable or a
constant value) of the same type, no additional conversion is performed
or required. If the operation is performed on two values of different types,
_Ego_ will convert the values to the same type if possible before
performing the operation. For example, the expression `10.0/3` divides
an integer value into a floating point value; the _Ego_ language converts
the integer to a floating point value before performing the division. In
general, _Ego_ will convert the values to the value that will result in
minimal or no loss of precision.

These conversions happen automatically, though you can use type casting
functions like `int()` or `string()` discussed later to force a specific
type of conversion operation. For example, if a boolean value is used in
an expression that requires a float64 value, then the boolean is converted
such that `false` is converted to `0.0` and `true` is converted to
 `1.0`. Similarly, if a numeric or boolean value is needed as a string,
the string value is the formatted version of the original value. So a
value of `123.5` as a float64 becomes the string `"123.5"`.

#### How much conversion happens depends on the type-checking mode

How much of this automatic conversion happens — and how much you must manage
explicitly yourself — depends on two things: the active **type-checking mode**
(`dynamic`, `relaxed`, or `strict`, set via the `ego.compiler.types` setting,
the `--types` command-line flag, or temporarily overridden with the
[`@type`](#at-type) directive), and **which of four boundaries** the
conversion happens at:

1. Assigning a value to a variable (`x = value`)
2. Combining two values in an expression (`a + b`, `a == b`, and so on)
3. Passing a value as a function argument
4. Returning a value from a function

These four boundaries do not all follow identical rules for a _non-constant_
value. But all four **do** agree on one thing in `strict` mode: a
**compile-time constant literal** (like `5` or `3.7` written directly in your
source) is given some of the same leeway Go itself gives an untyped constant
— it may adapt to a narrower type — but, matching Go's own static rejection
of a lossy untyped-constant conversion, **only when doing so loses no
information**. This is consistent across all four boundaries as of the
BUG-68 fix; see [Key differences from Go](#go-differences) for the one-line
summary.

##### Assigning to a variable

| Mode | A non-constant value of a different type | A constant literal of a different type |
| :--- | :--- | :--- |
| `dynamic` (default) | The variable's type changes to match the new value. | Same — the variable's type changes to match. |
| `relaxed` | Converted automatically to the variable's existing type. An error is raised only if the value genuinely cannot be converted at all (e.g. the string `"abc"` into an `int`). | Same as a non-constant value: converted automatically, even if the conversion loses information — `w = 3.7` on an `int` variable silently becomes `3`. |
| `strict` | **Rejected.** Cast explicitly first: `w = int(f)`. | Converted automatically, but **only if no information is lost**. `var w int = 5; w = 3` succeeds; `w = 3.7` is rejected, because truncating the fractional part would lose information — even though `3.7` is itself a constant. |

See [`@type`](#at-type) for the complete set of strict-mode assignment rules,
including array constants and struct member creation.

##### Combining values in an expression

When both operands already have the same type, no conversion is needed. When
they differ:

| Mode | Two non-constant values of different types | One constant, one non-constant |
| :--- | :--- | :--- |
| `dynamic` / `relaxed` | Both operands are converted to whichever type loses the least precision — e.g. `anInt32 + anInt64` promotes both to `int64`. | The constant adapts to match the non-constant operand's type, even if that loses information — e.g. `anInt32 + 2` stays `int32` (a bare `2` would otherwise be treated as the "wider" plain `int` kind), and `anInt32 + 2.7` silently drops the fractional part. |
| `strict` | **Rejected** with a type-mismatch error. Combine values of the same type, or cast one side explicitly first. | The constant adapts to the non-constant operand's type — **but only losslessly**. `anInt32 + 2` stays `int32`; `anInt32 + 2.7` is rejected, the same way assigning `2.7` to an `int32` variable is rejected. |

##### Passing a function argument, and returning a value from a function

These two boundaries follow the same rules as each other, and — as of the
BUG-68 fix — the same lossless-constant rule as assignment and expressions:

| Mode | A non-constant argument/return value of a different type than the parameter/declared return type | A constant literal argument/return value of a different type |
| :--- | :--- | :--- |
| `dynamic` / `relaxed` | Converted automatically to the declared type. | Converted automatically to the declared type, even if that loses information — `f(2.7)` (`f` taking `int32`) silently becomes `f(2)`. |
| `strict` | **Rejected** with an argument-type or type-mismatch error. Cast explicitly first. | Adapts to the declared type, **but only losslessly** — `func f(x int32) int32 {...}; f(4)` and `return x * 2` (where `x` is `int32`) both work, the same leniency Go gives an untyped constant; `f(2.7)` and `return x * 2.7` are both rejected, the same way the equivalent assignment would be. |

##### The `ego.runtime.precision.error` setting

This is a separate mechanism from the lossless-constant rule above (which
applies unconditionally in `strict` mode, at all four boundaries, regardless
of this setting). This setting (default `false`) instead controls whether an
_ordinary_ conversion — a non-constant value, an explicit cast such as
`int32(x)`, or a constant being converted in `dynamic`/`relaxed` mode — raises
a runtime error instead of silently truncating or wrapping. With the default
`false`:

- An integer conversion that overflows the target width silently wraps
  (`int32(99999999999)` produces whatever value 64-bit-to-32-bit truncation
  yields, with no error).
- A float-to-integer conversion that has a fractional part is truncated
  toward zero with no error, for every integer width.

Setting `ego.runtime.precision.error=true` makes both of the above raise a
catchable runtime error instead, consistently for every integer width
(`int8` through `int64`, signed and unsigned, and plain `int`/`uint`).
Explicit casts (`int32(x)`, `byte(x)`, etc.) use this exact same per-width
coercion machinery, so the same rules apply to them too.

### Builtin Functions<a name="builtInFunctions"></a>

The _Ego_ language includes a library of built-in functions which can
also be used as elements of an expression, including having the function
value be assigned to a variable. A function consists of a name, followed
by a list of zero or more values in parenthesis, separated by commas. If
there are no values, you still must specify the parenthesis. The function
may accept a fixed or variable number of arguments, and typically returns
a single value.

&nbsp;

| Function | Example | Description |
| :-------- | :-------------------- | :---------- |
| append() | append(list, 5, 6, 7) | Append the items together into an array. |
| close() | close(sender) | Close a channel. See the information on [Threads](#threads) for more info. |
| complex() | complex(3.0, 4.0) | Combine two real numbers into a complex value, `3+4i`. Produces `complex64` only when both arguments are literally `float32`; `complex128` otherwise. |
| delete() | delete(emp, "Name") | Remove the named field from a map, or a delete a dynamic struct member |
| imag() | imag(3+4i) | Return the imaginary part of a complex value, `4`, as `float32` or `float64` matching the argument's width |
| index() | index(items, 55) | Return the array index of `items` that contains the value `55` |
| len() | len(items) | If the argument is a string, return its length in characters. If it is an array, return the number of items in the array |
| make() | make([]int, 5) | Create an array of int values with `5` elements in the array |
| real() | real(3+4i) | Return the real part of a complex value, `3`, as `float32` or `float64` matching the argument's width |

&nbsp;
&nbsp;

### Casting

This refers to functions used to explicitly change the type of a value, or
convert it to a comparable value where possible. This can be done for base
type values (int, bool, string) as well as for arrays.

For base types, the following are available:

&nbsp;

| Function | Example | Description |
| :--------- | :-------------------- | :---------- |
| bool() | bool(55) | Convert the value to a boolean, where zero values are false and non-zero values are true |
| byte() | byte(65) | Convert the value to an 8-bit integer |
| int16() | int16(25434) | Convert the value to an 16-bit integer |
| int32() | int32(4096) | Convert the value to an 32-bit integer |
| int() | int(78.3) | Convert the value to an integer, in this case `78` |
| int64() | int64(2^20) | Convert the value to a 64-bit integer, in this case `1125899906842624` |
| uint8() | uint8(240) | Convert the value to an unsigned 8-bit integer |
| uint16() | uint16(40000) | Convert the value to an unsigned 16-bit integer |
| uint32() | uint32(2503) | Convert the value to an unsigned 16-bit integer |
| uint() | uint(240) | Convert the value to an unsigned default-sized integer |
| uint64() | uint64(12071957) | Convert the value to an unsigned 64-bit integer |
| float32() | float32(33) | Convert the value to a 32-bit floating value, in this case `33.0` |
| float64() | float64(33) | Convert the value to a 64-bit floating value, in this case `33.0` |
| complex64() | complex64(3) | Convert a real number to a complex value with a 32-bit real/imaginary pair, in this case `3+0i` |
| complex128() | complex128(3) | Convert a real number to a complex value with a 64-bit real/imaginary pair, in this case `3+0i` |
| string() | string(true) | Convert the argument to a string value, in this case `true` |

&nbsp;

A special note about `string()`; it has a feature where if the value passed in is an array of
integer values, each one is treated as a Unicode rune value and the resulting string is
the return value. Any other type is just converted to its default formatted value.

Note that `string(int)` in Ego converts to the **decimal string representation** of the integer,
not the Unicode character for that code point. This differs from Go, where `string(65)` produces
`"A"`. In Ego, `string(65)` produces `"65"`. Use `string([]int{65})` to convert a single code
point to its Unicode character.

You can also perform conversions on arrays, to a limited degree. This is done with
the function:

&nbsp;

| Function | Example | Description |
| :------- | :------ | :---------- |
| []bool() | []bool([1, 5, 0]) | Convert the array to a []bool array. |
| []int() | []int([1, 5.5, 0]) | Convert the array to a []int array. If the parameter is a string, then the string is converted to an array of int values representing each rune in the string. |
| []any() | []any([true, 5, "string"]) | Convert the array to a []any array where there are no static types for the array. |
| []float64() | []float64([1, 5, 0]) | Convert the array to a []float64 array |
| []float32() | []float32([1, 5, 0]) | Convert the array to a []float32 array |
| []string() | []string([1, 5, 0]) | Convert the array to a []string array. |

&nbsp;

In all cases, the result is a typed array of the given cast type. Each
element of the array is converted to the target type and stored in the
array. So []bool() on an array of integers results in an array of bool
values, where zeros become false and any other value becomes true. The
special type name `any` means _no specified type_ and is used
for arrays with heterogenous values.

Note the special case of []int("string"). If the parameter is not an
array, but instead is a string value, the resulting []int array contains
each rune from the original string.

#### make

The `make` pseudo-function is used to allocate an `array`, a `map`, or a
`channel` with the capacity to hold multiple messages. The first argument
must be a data type specification, and the second argument is the size
of the item (array elements or channel messages). For a `map` type, the
size is optional.

```go
a := make([]int, 5)
a := make(map[string]int)
b := make(chan, 10)
```

The first example creates an `array` of 5 elements, each of which is of type `int`,
and initialized to the _zero value_ for the given type. This could have been
done by using `a := [0,0,0,0,0]` as a statement, but by using the make() function
you can specify the number of elements dynamically at runtime.

The second example creates a `map` with a type that specifies a `string` key and an
`int` value. The size can be omitted; it is present for compatibility with Go, which
can use it as a hint for storage allocation. Currently, _Ego_ ignores the size
value.

The third example creates a `channel` capable of holding up to 10 messages. Note that,
unlike Go, Ego channels do not take an element type — `make(chan, 10)`, not
`make(chan string, 10)`. Creating a `channel` like this is required if the `channel`
is shared among many threads. If a `channel` variable is declared by default, it
holds a single message.
This means that before a thread can send a value, another thread must read the
value; if there are multiple threads waiting to send they are effectively going to
run one-at-a-time. By creating a `channel` that can hold multiple messages, up to 10
(in the above example) threads could send a message to the `channel` before the
first message was read.

&nbsp;
&nbsp;

### Type Assertions and Unwrapping<a name="typeAssertions"></a>

When a value is stored in a variable of type `any` (Go's `interface{}`), _Ego_
remembers the concrete type of the value along with the value itself. A
**type assertion** — sometimes described as "unwrapping" the interface — lets
you ask, at runtime, whether that stored value actually holds a value of a
specific type, and if so, retrieve it as that type:

```go
x.(T)
```

This is written as the value, followed by `.`, followed by the target type
in parentheses. A type assertion is **not** a type conversion — no coercion
is performed. It either confirms that the concrete value stored in `x` is
of type `T`, or it fails; it does not attempt to convert one type into
another the way `int("42")` would. Use the [casting functions](#builtInFunctions)
when you want conversion; use a type assertion when you want to check and
recover a value's actual type.

#### The two-value ("comma-ok") form

```go
value, ok := x.(T)
```

This is the safe form. `ok` is `true` if `x` holds a value of type `T`, in
which case `value` holds that value. If `x` holds something else, `ok` is
`false` and `value` is `nil` — no error is raised. This is the idiomatic way
to test a value's type without risking a runtime error:

```go
var x any = 42

n, ok := x.(int)
if ok {
    fmt.Println("x is an int:", n)
} else {
    fmt.Println("x is not an int")
}
```

#### The single-value form

```go
value := x.(T)
```

If you are certain `x` holds a `T` — or want a runtime error if it does not
— use the single-value form. If the assertion fails, it raises a catchable
runtime error, exactly as if a division by zero or similar runtime error
had occurred, and can be caught with [`try`/`catch`](#try-catch):

```go
var x any = 42
n := x.(int)          // succeeds; n is 42
fmt.Println(n)

var y any = "not a number"

try {
    m := y.(int)       // fails; raises a runtime error
    fmt.Println(m)
} catch(e) {
    fmt.Println("assertion failed:", e)
}
```

> **Note:** write the single-value form as its own standalone assignment
> statement (`v := x.(T)`), as shown above. Using it directly inline as
> part of a larger expression — for example calling the result immediately
> (`x.(func())()`) or combining it with an operator (`x.(int) + 1`) — is not
> yet fully supported and can produce incorrect results. Assign the
> asserted value to a variable first, and use that variable afterward.

#### Asserting to a builtin type

Any base type name is a valid assertion target:

```go
var v any = "hello"

s, ok := v.(string)
fmt.Println(s, ok)     // hello true

n, ok := v.(int)
fmt.Println(n, ok)     // <nil> false  (v does not hold an int)
```

Asserting to `any` (or the equivalent `interface{}`) always succeeds, since
every value satisfies the empty interface:

```go
var v any = 42

x, ok := v.(any)
fmt.Println(x, ok)     // 42 true
```

#### Asserting to a user-defined type

A type assertion works the same way for a struct type you have defined,
and for a pointer to one:

```go
type Point struct {
    X int
    Y int
}

var v any = &Point{X: 1, Y: 2}

p, ok := v.(*Point)
if ok {
    fmt.Println(p.X, p.Y)   // 1 2
}
```

A slice, map, or array type can also be the target of an assertion:

```go
var v any = []int{1, 2, 3}

a, ok := v.([]int)
fmt.Println(ok, len(a))    // true 3
```

#### Asserting to a function type

A function type can also be the assertion target. This is most useful when
a function value has been stored in an `any`-typed variable, map, or struct
field, and needs to be retrieved and called:

```go
var v any = func() int {
    return 42
}

f, ok := v.(func() int)
if ok {
    fmt.Println(f())        // 42
}
```

A function type with multiple return values works the same way:

```go
var v any = func() (int, string) {
    return 5, "five"
}

f := v.(func() (int, string))
n, s := f()
fmt.Println(n, s)           // 5 five
```

A function type declared with parameters works as an assertion target too,
in either Go's usual named form (`func(a, b int) int`) or its type-only form
with no parameter names at all (`func(int, int) int`):

```go
var v any = func(a, b int) int {
    return a + b
}

f, ok := v.(func(int, int) int)
if ok {
    fmt.Println(f(3, 4))    // 7
}
```

A function type can also be used as an ordinary variable's type, the same
way any other type can:

```go
var g func(int, int) int
g = func(a, b int) int { return a * b }
fmt.Println(g(3, 4))        // 12
```

#### Type-switch: asserting against multiple types at once

The [`switch`](#switch) statement has a special form,
`switch v := x.(type)`, that lets you branch on `x`'s concrete type
directly, without writing a chain of individual assertions:

```go
var x any = 42

switch v := x.(type) {
case int:
    fmt.Println("int:", v)
case string:
    fmt.Println("string:", v)
default:
    fmt.Println("some other type")
}
```

Inside each `case`, `v` has already been narrowed to that case's type — no
further assertion is needed. This form is syntactic sugar over a sequence
of individual type assertions, and follows the same matching rules
described above.

&nbsp;
&nbsp;

## Conditional and Iterative Execution <a name="flow-control"></a>

We have discussed how variables are created, and how expressions are
used to calculate values based on variables, constant values, and
functions. However, most interesting programs require some decision
making to control the flow of execution based on the values of
variables. This section will describe how to make _either/or_ decisions
in the code, and how to execute a block of code repeatedly until a
condition is met.

### If-Else <a name="if"></a>

The general nature of a conditional `if` statement is

```go
     if <condition> {
         <statements>
     } else { 
         <statements>
     }
```

The `else` clause is optional, as described below. Even when there
is only a single statement in the block, a basic block is used for
readability.

Consider the following example code:

 ```go
salary := hours * wage                  // (1)
if salary < 100.0 {                     // (2)
   fmt.Println("Not paid enough!")      // (3)
}                                       // (4)

total = total + salary                  // (5)
```

This introduces a number of new elements to a program, so let's go
over them line-by-line. The numbers is parenthesis are not part of
the program, but are used to identify each line of the code.

1. This first line calculates a new value by multiplying the `hours`
   times the `wage`, and store it in a new value called `salary`.
   This uses an assignment statement; the `:=` indicates the variable
   does not already exist and will be created by this operation. We
   assume the values of `hours` and `wage` were calculated already in
   the program.

2. This statement performs a conditional evaluation. After the `if`
   statement, there is a relational expression that can be converted
   to a boolean value. In this case, if `salary` has a value less than
   `100.0` then the code will execute some additional statement(s).
   After the expression, the `{` character defines the start of a
   _basic block_ which is a group of statements that are all executed
   together.

3. If `salary` is less than 100.0, then the `fmt.Println()` operation is
   performed. Don't worry that we haven't talked about this yet; its
   covered below in the section on the `fmt` package, but it is enough
   to know that this produces a line of output with the string
   "Not paid enough!". If, however, the value of `salary` is not less
   than 100.0, then the basic block is not executed, and the program
   continues with the statement after the block.

4. The `}` character defines the end of the basic block. If the `if`
   statement condition is not met, execution continues after this
   end-of-block indicator.

5. This statement will be executed next, regardless of whether
   `salary` was less than 100.0 or not. This statement updates the
   value of `total` to be the sum of `total` and the `salary`.

Instead of having the `if` statement advance to the next statement
if the condition is not true, a second _basic block_ can be defined
that has the statements to execute if the condition is false. That
is, the result of the expression will result in one or the other of
two basic blocks being executed.

```go
salary := hours * wage
if salary < 100 {
    scale = "small"
} else {
    scale = "large"
}
```

In this example, after calculating a value for `salary`, it is
compared to see if it is less than 100. If so, then the value
`scale` is set to the value `"small"`. But if the value of `salary`
is not less than 100, the value of `scale` is set to `"large"`.
Regardless of which basic block was executed, after the block
executes, the program resumes with the next statement after the
`if` statements.

### Switch <a name="switch"></a>

A `switch` statement selects one of several blocks to execute based
on the value of an expression. It is a more concise alternative to
a chain of `if`/`else if`/`else` statements. The expression value is
compared to each successive value in the `case` statements, and if the
expression matches the value, the statement(s) after the `case` statement are
executed. The execution resume after the `switch` statement is closed
by the `}` end-of-block.

```go
switch <expression> {
case <value1>:
    <statements>
case <value2>, <value3>:
    <statements>
default:
    <statements>
}
```

You can also create an expression that is executed and the result is
available inside the scope of the `switch` statement that can also be
used as the value to determine which `case` to select. For example,

```go
switch x := f(); x {
    case 1:
    ...
}
```

The expression `x := f()` is executed and the value of `f` becomes a
symbol available in the body of the `switch` statement block. The value
after the semicolon is the value switched on, but this can be an arbitrary
expression that need not be based on `x`. As an Ego-specific extension, you
can omit the semicolon and expression, and the value of the assignment (`x`
in the above example) is automatically selected as the _switch_ value.

The expression after `switch` is evaluated once and compared against
each `case` value in order. When a match is found, the statements in
that `case` block are executed, and execution continues after the `switch`
block (there is no fall-through between cases, unlike languages like _C_).
A `default` clause is optional; it executes when no `case` value matches.

```go
day := "Saturday"

switch day {
case "Saturday":
    fmt.Println("Weekend")
case "Sunday":
    fmt.Println("Weekend")
default:
    fmt.Println("Weekday")
}
```

A `switch` statement can also be written without an expression, in which
case each `case` provides a boolean condition. The first `case` whose
condition is true is selected:

```go
score := 85

switch {
case score >= 90:
    fmt.Println("A")
case score >= 80:
    fmt.Println("B")
case score >= 70:
    fmt.Println("C")
default:
    fmt.Println("F")
}
```

This form is equivalent to a chain of `if`/`else if` statements.

A `switch` can also branch on the concrete type of an `any`-typed value,
using `switch v := x.(type) { ... }`. See
[Type Assertions and Unwrapping](#typeAssertions) for details.

#### `break` inside a `switch`

Because a matched `case` never falls through into the next one, you do not
need a `break` statement just to stop a `case` body from running into the
next `case` — that already happens automatically. A `break` statement is
still useful, though, if you want to stop executing a `case` body early,
before reaching its last statement:

```go
switch x {
case 1:
    fmt.Println("checking x")
    if x < 0 {
        break               // stop here; skip the rest of this case body
    }
    fmt.Println("x is not negative")
}
```

A `break` used this way ends only the `switch` statement. If the `switch` is
itself inside a `for` loop, `break` does **not** exit the loop — execution
simply resumes with whatever statement follows the `switch` block, and the
loop continues with its next iteration:

```go
for i := 0; i < 5; i++ {
    switch i {
    case 3:
        break               // exits the switch only, not the for loop
    }
    fmt.Println(i)          // still runs on every iteration, including i == 3
}
```

This program prints `0`, `1`, `2`, `3`, and `4` — the loop runs to
completion. If you want a `break` written inside a `switch` to end an
enclosing loop instead, label the loop and use that label on the `break`
statement, as described in [Break and Continue](#break-continue):

```go
outer:
for i := 0; i < 5; i++ {
    switch i {
    case 3:
        break outer         // exits the labeled for loop, not just the switch
    }
    fmt.Println(i)
}
```

This prints only `0`, `1`, and `2`, since `break outer` exits the loop as
soon as `i` reaches `3`.

### For _condition_ <a name="for-conditional"></a>

The simplest form of iterative execution (also referred to as a
"loop") is the `for` statement, followed by a condition, and a
basic block that is executed as long as the condition is true.

```go
for <condition> {
  <statements>
}
```

Here is an example

```go
value := 0                  // (1)
for value < 5 {             // (2)
    fmt.Println(value)      // (3)
    value = value + 1       // (4)
}                           // (5)
```

1. This line initializes variable `value` to an integer zero.
2. The `for` statement specifies that as long as the `value`
   variable contains a number less than `5` the following basic
   block will be executed over and over.
3. This line of the basic block prints the current `value`
4. This line increments the variable by adding one to itself.
   This causes `value` to increase each time the loop runs.
5. This is the end of the basic block describing the body of
   the loop that will execute until the condition is no longer true.

This program results in the numbers `0` through `4` being printed
to the output. When `value` is incremented to be the value `5`,
the condition on line #2 is no longer true, and the loop stops
executing, and the program will continue with whatever follows line 5.

Note that in this example, without line number 4 the program would
run forever, because the variable `value` would be initialized to
zero but then never change, so the condition will always be true.

### For _index_ <a name="for-index"></a>

You can create a `for` loop that explicitly specifies an expression
that defines the starting value, ending condition, and how the value
changes with each iteration of a loop. For example,

```go
for i := 0; i < 10; i++ {
    fmt.Println(i)
}
```

This loop will print the values `0` through `9` to the standard
console. The index variable `i` is first initialized with the value
`0` from the first `for` statement clause. The second statement
clause describes the condition that must be true for the loop body
to be executed. This is evaluated before the loop body is run each
time. The third clause is the statement that modifies the index
value _after_ the body of the loop is run but _before_ the  next
evaluation of the clause that determines if the loop continues. In
this example, we could have used:

```go
for i := 0; i < 10; i = i + 1 {
    fmt.Println(i)
}
```

but the use of the increment operator `++` is more expressive of
the desired operation ("add one to this value").

The variable `i` in the above example is scoped to the `for`
statement and its loop body. That is, after this loop runs, the
variable `i` will no longer exist because it was created (using
the `:=` operator) in the scope of the loop. You can use a simple
assignment (`=`) of an existing variable if you want the updated
index value available after the loop body ends.

```go
var i int
for i = 0; i < 10; i++{
    fmt.Println(i)
}

fmt.Println("The final value of i is ", i)
```

This example uses a variable that already exists outside the scope
of the `for` loop, so the value continues to exist after the loop
runs. This allows the final statement to print the value of the
index variable that terminated the loop.

### For _range_ <a name="for-range"></a>

You can create a loop that indexes over all the values in an array,
in sequential order. The index value is the value of the array
element. For example,

```go
ids := [ 101, 143, 202, 17]
for i, j := range ids {
    fmt.Println("Array member ", i, " is ", j)
}
```

This example will print a line for each value in the array, in the
order they appear in the array. During each iteration of the loop,
the variable `i` will contain the numerical array index (0, 1, 2, …)
and the variable `j` will contain the actual value from the array
at that position. You can also use only one variable with `range`,
in which case you receive just the index, as in:

```go
for _, v := range ids {
    fmt.Println(v)
}
```

In this example, for each iteration of the loop, the variable `v`
will contain the actual values from the array. By using the reserved
name `_` for the index variable, the positional index is discarded —
useful when you only care about the values. Similarly, you can use
`range` with a single variable to iterate over just the index values:

```go
for i := range ids {
    fmt.Println(ids[i])
}
```

In this case, if the array `ids` has 4 values, then `i` takes the
values `0`, `1`, `2`, and `3` — the zero-based index positions. The
corresponding array element is accessed as `ids[i]`.

Similarly, you can use the `range` construct to step through the values
of a map data type. For example,

```go
inventory := map[string]int{}
inventory["wrenches"] = 5
inventory["pliers"] = 12
inventory["hammers"] = 2

for product, count := range inventory {
    fmt.Println("There are ", count, " ", product, " in stock.")
}
```

When the loop runs, the value of `product` is set to each key in the
map, and `count` is set to the value associated with that key. These
variables exist only within the body of the loop. Note that if you
omit either one and use the `_` variable instead, that item (key or
value) is not read from the map. You can use this to generate a list
of the keys, for example:

```go
names := []string{}
for name := range inventory {
    names = append(names, name)
}

fmt.Println("The products are all named", strings.Join(names, ", "))
```

This creates an array of string values, and stores the name of each
key in the array by appending them. The use of `strings.Join()` will
be discussed later in the `strings` package, but for now, it is
sufficient to know it creates a string from an array of strings,
separating each item by the ", " string. So an array

```go
[]string{"Tom", "Bob", "Sue"}
```

is formatted into the string "Tom, Bob, Sue".

You can also use `range` to step through the characters of a `string`
value. This matches Go's behavior exactly: the index variable receives
the **byte offset** of each character within the `string` (not a simple
0, 1, 2, … count), and the value variable receives the decoded Unicode
code point as an `int32` (Go calls this type `rune`, which is just
another name for `int32`) — **not** a one-character `string`. For example,

```go
for i, ch := range "Hi⌘!" {
    fmt.Printf("i=%d, ch=%v, type=%T\n", i, ch, ch)
}
```

prints:

```text
i=0, ch=72, type=int32
i=1, ch=105, type=int32
i=2, ch=8984, type=int32
i=5, ch=33, type=int32
```

Note that the index jumps from `2` to `5`, skipping `3` and `4`, because
`⌘` (U+2318) is encoded as 3 bytes in UTF-8 — the index tracks byte
position, not character count. If you need the character as a
one-character `string` instead of its integer code point, convert it
explicitly:

```go
for i, ch := range "Hi⌘!" {
    s := string([]int32{ch})   // "H", "i", "⌘", "!"
    fmt.Println(i, s)
}
```

### `break` and `continue` <a name="break-continue"></a>

Sometimes when running an loop, you may wish to change the flow of
execution in the loop based on conditions unrelated to the index
variable. For example, consider:

```go
for i := 1; i < 10; i = i + 1 {
    if i == 5 {                      // (1)
        continue                     // (2)
    }

    if i == 7 {                      // (3)
        break                        // (4)
    }

    fmt.Println("The value is ", i)  // (5)
}
```

This loop will print the values 1, 2, 3, 4, and 6. Here's what
each statement is doing:

1. During each execution of the loop body, the index value is
compared to 5. If it is equal to 5 (using the `==` operator),
the conditional statement is executed.
2. The `continue` statement causes control to branch back to the
top of the loop. The index value is incremented, and then tested
again to see if the loop should run again. The `continue` statement
means "stop executing the rest of _this_ loop body, but continue
looping".
3. Similarly, the index is compared to 7, and if it equal to 7 then
the conditional statement is executed.
4. The `break` statement exits the loop entirely. It means "without
changing any of the values, behave as if the loop condition had been
met and resume execution after the loop body.

Note that if you create a `for` loop that has no conditional, index,
or range value, then the loop body _must_ contain at least one `break`
statement. If there is no `break` statement, then the loop would never
end.

You can optionally specify a label on the ``break` or `continue` statement.
A bare statement will perform the operation on the inner-most loop, but
if you use a label you can unwind inner loops before breaking or continuing
the loop with the named label. Note that statement labels can _only_ be
used before a `for` statement.

&nbsp;
&nbsp;

## User Function Definitions <a name="user-functions"></a>

In addition to the [Builtin Functions](#builtInFunctions) listed
previously, the user program can create functions that can be
executed from the _Ego_ program. Just like variables, functions
have scope, and can only be accessed from within the program in
which they are declared. Most functions are defined in the program
file before the body of the program.

### The `func` Statement <a name="function-statement"></a>

Use the `func` statement to declare a function. The function must
have a name, optionally a list of parameter values that are passed
to the function, and a return type that indicates what the function
is expected to return. This is followed by the function body described
as a basic block. When the function is called, this block
is executed, with the function arguments all available as local
variables. For example,

```go
func addValues( v1 float64, v2 float64) float64 {
    x := v1 + v2

    return x
}

// Calculate what a variable value (10.5) added to 3.8 is
a := 10.5
x := addValues(a, 3.8)
fmt.Println("The sum is ", x)
```

In this example, the function `addValues` is created. It accepts two
parameters; each is of type float64 in this example. The parameter
values actually passed in by the caller will be stored in local
variables v1 and v2. The function definition also indicates that
the result of the function will also be a float64 value.

Parameter types and return type cause type _coercion_ to occur,
where values are converted to the required type if they are not
already the right value type. For example,

```go
y := addValues("15", 2)
```

Would result in `y` containing the floating point value 17.0.
This is because the string value "15" would be converted to a
float64 value, and the integer value 2 would be converted to a
float64 value before the body of the function is invoked. So
`typeof(v1)` in the function will return "float64" as the result,
regardless of the type of the value passed in when the function
was called.

Coercion applies to _scalar_ parameter types: bool, all integer
widths, both float widths, and string. If the argument value
cannot be converted to the required type — for example, passing
the string `"abc"` where an `int` is expected — a runtime error
is raised that identifies the argument position and original value.
This error is catchable with `try/catch`. Complex types (struct,
array, map, channel, function, pointer) are never silently coerced;
a type mismatch on these parameters always raises an error in strict
mode and is accepted as-is in dynamic mode.

The `func` statement allows for a special data type `any`
which really means "any type is allowed" and no conversion occurs.
If the function body needs to know the actual type of the value
passed, the `typeof()` function can be used.

A function that does not return a value at all should omit the
return type declaration.

A function can return a `chan` value, exactly like any other type — but
because Ego channels are always untyped (see [Channels](#channels)), the
return type must be written as bare `chan`, never `chan T`:

```go
func newRequestQueue() chan {
    return make(chan, 10)
}

q := newRequestQueue()
q <- "request 1"
fmt.Println(<-q)
```

This is the same rule that applies to `chan` everywhere else a type is
written — a `var` declaration, a function parameter, a struct field,
`make()`, or a type-assertion target — so it is worth calling out
specifically for return types: `chan T` is rejected there too, with the
same `channels do not have an element type; use "chan" alone, not "chan T"`
error you would get from any of those other contexts. This holds for both a
single bare return type (`func f() chan { ... }`) and a `chan` inside a
parenthesized multi-value return list (`func f() (chan, error) { ... }`).

There is an important difference between Ego and Go, regarding
nested function declarations. In Go, you cannot create a named
nested function declaration at all; only function closures
(basically, expressions defining a function) can be used. In
Ego, you can use function closures just like Go, but you can
also write nested named functions which have a nested scope
and can only reference parameters and locally-declared or
fully global variable names.

```go
// This is valid; the closure function has access to the symbols
// of the scope in which it is defined. This matches Go functionality.
func main() {
    name := "Tom"

    f := func() string {
        return name + " Cole"
    }

    fmt.Println(f())
}
```

Ego allows a named function to be declared with local scope, but
it cannot reference values outside it's definition. That is, it
can only reference variables in the scope of the function, or
global variables visible to all scopes. For example,

```go
// This is invalid; a named function declaration is valid in Ego (there
// is no equivalent in Go), but it has limited scope and can only reference
// it's parameters and values within the function. This will cause an
// error at compile time indicating the reference to 'name' is illegal.
func main() {
    name := "Tom"

    func last() string {
        return name + " Cole"
    }

    fmt.Println(last())
}
```

If you tried to compile the second example, it wouldn't run. The
compiler would report the attempt to reference a variable that is
outside the function scope but not global in scope as an error.
In this simple example, one fix would be to pass the value of the
`name` variable as a parameter to the inner function `last()`.

&nbsp;
&nbsp;

### The `return` Statement  <a name="return-statement"></a>

When a function is ready to return a value the `return` statement
is used. This identifies an expression that defines what is to be
returned. The `return` statement results in this expression being
_coerced_ to the data type named in the `func` statement as the
return value. If the example above had a `string` result type,

```go
func addValues( v1 float64, v2 float64) string {
    x := v1 + v2

    return x
}

y := addValues(true, 5)
```

The resulting value  for `y` would be the string "6". This is
because not only will the boolean `true` value and the integer
5 be converted to floating point values, but the result will
be converted to a string value when the function exits.

Note that if the `func` statement does not specify a type for
the result, the function is assumed not to return a result at
 all. In this case, the `return` statement cannot specify a
 function result, and if the `return` statement is the last
 statement then it is optional. For example,

```go
func show( first string, last string) {
    name := first + " " + last

    fmt.Println("The name is ", name)
}

show("Bob", "Smith")
```

This example will run the function `show` with the two string
values, and print the formatted message "The name is Bob Smith".
However, the function doesn't return a value (none specified
after the parameter list before the function body) so there is
no `return` statement needed in this case.

Also note that the invocation of the `show` function does not
specify a variable in which to store the result, because there
is none. In this way you can see that a function can be called
with a value that can be used in an assignment statement or
an expression, or just called and the result value ignored.
Even if the `show` function returned a value, the invocation
ignores the result and it is discarded.

### The `defer` Statement  <a name="defer-statement"></a>

Sometimes a function may have multiple places where it returns
from, but always wants to execute the same block of code to
clean up a function (for example, closing a file that had been
 opened).

```go
func getName() bool {
    f := io.Open("name")

    defer io.Close(f)

    s := io.ReadLine(f)
    if s == "" {
        return false
    }

    return true
}
```

In this example, the function opens a file (the `io` package is
discussed later). Because we have opened a file, we want to be
sure to close it when we're done. This function has two `return`
statements. We could code it such that before each one, we also
call the io.Close() function each time. But the `defer` statement
allows the function to specify a statement that will be executed
_whenever the function actually returns_ regardless of which
branch(es) through the conditional code are executed.

Each `defer` statement identifies a statement or a _basic block_
of statements (enclosed in "{" and "}") to be executed. If there
are multiple `defer` statements, they are all executed in the
reverse order that they were defined. So the first `defer`
statement is always the last one executed before the function
returns.

Note that `defer` statements are executed when the function comes
 to the end of the function body even if there is no `return`
 statement, as in the case of a function that does not return a
 value.

#### When are a deferred call's arguments evaluated?

A `defer` statement defers two different things, and it's important to keep
them separate: it defers _making the call_, but it does **not** defer
_evaluating the call's arguments_. The arguments are evaluated immediately,
at the point where the `defer` statement itself runs -- their values are
then frozen and simply carried along until the deferred call actually
happens later.

```go
func example() {
    x := 1
    defer fmt.Println("x was", x)   // "x was 1" -- x is read RIGHT NOW
    x = 2
    x = 3
    // when example() returns, the deferred call finally runs, printing
    // "x was 1" -- not "x was 3" -- because x's value was captured back
    // when the defer statement executed.
}
```

This is true no matter what form the deferred call takes:

- A named function call, `defer namedFunc(arg)`.
- A method call, `defer receiver.Method(arg)`.
- The closure-with-immediate-call form, `defer func(p T){ ... }(arg)`.

All three evaluate `arg` immediately and hand a snapshot of its value to the
deferred call. This is also why deferring inside a loop works the way you'd
expect -- each iteration's `defer` captures that iteration's value, not
whatever the loop variable ends up being by the time the function returns:

```go
func example() {
    for i := 0; i < 3; i++ {
        defer fmt.Println("i was", i)
    }
    // Prints, in this order when example() returns:
    //   i was 2
    //   i was 1
    //   i was 0
    // (reverse order, because defers run last-in-first-out -- but each one
    // still remembers its own iteration's value of i.)
}
```

Only the values passed as an argument are captured this way. Anything else
the deferred call refers to -- a variable from the surrounding scope that
isn't passed as an argument, or a field reached through a receiver -- is
still looked up normally when the deferred call actually runs, exactly like
an ordinary closure:

```go
func example() {
    total := 0
    defer func() {
        fmt.Println("final total was", total)   // reads "total" when it RUNS
    }()
    total = total + 1
    total = total + 1
    // Prints "final total was 2" -- because this closure takes no
    // arguments, so nothing was captured eagerly; it simply reads whatever
    // "total" holds by the time the deferred call executes.
}
```

### Function Variable Values  <a name="function-variables"></a>

Functions can be values themselves. For example, consider:

 ```go
p := fmt.Println
```

This statement gets the value of the function `fmt.Println` and
stores it in the variable p. From this point on, if you wanted
to call the package function that prints items to the console,
instead of using `fmt.Println` you can use the variable `p` to
invoke the function:

```go
p("The answer is", x)
```

This means that you can pass a function as a parameter to another
function. Consider,

```go
func show( fn any, name string) {
    fn("The name is ", name)
}

p := fmt.Println
show(p, "tom")
```

In this example, the `show` function has a first parameter that
is a function, and a second parameter that is a string. In the
body of the function, the variable `fn` is used to call the
`fmt.Println` function. You might use this if you wanted to send
output to either the console (using `fmt.Println`) or a file
 (using `io.WriteString`). The calling code could make the choice
  of which function is appropriate, and pass that directly into
  the `show` function which makes the call.

You can even create a function literal value, which defines the
 body of the function, and either store it in a variable or pass
 it as a parameter. For example,

```go
p := func(first string, last string) string {
            return first + " " + last
        }

name := p("Bob", "Smith")
```

Note that when defined as a function literal, the `func` keyword
is not followed by a function name, but instead contains the
parameter list, return value type, and function body directly.
There is no meaningful difference between the above and declaring
`func p(first string...` except that the variable `p` has scope
that might be limited to the current _basic block_ of code. You
can even define a function as a parameter to another function
directly, as in:

```go
func compare( fn any, v1 any, v2 any) bool {
    return fn(v1, v2)
}

x := compare( func(a1 bool, a2 bool) bool { return a1 == a2 }, true, false)
```

This somewhat artificial example shows a function named `compare`
that has a first parameter that will be a function name, and two
additional parameters of unknown type. The invocation of `compare`
passes a function literal as the first parameter, which is a
function that compares boolean values. Finally, the actual two
boolean values are passed as the second and third parameters
in the `compare` function call. The `compare` function will
immediately invoke the function literal (stored in `fn`) to compare
two boolean value, and return the result.

A more complex example might be a function whose job is to sort a
list of values. Sorting a list of scalar values is available as
built-in function to the sort package, but sorting a list of
complex types can't be done this way. You could write a sort function,
that accepts as a parameter the comparison operation, and that
function knows how to decide between two values as to which one sorts
first. This lets the sort function you create be generic without regard
for the data types.

### Nested Named Functions and Scope<a name="nested-functions"></a>

Ego allows a named function to be declared inside another named function. This
is not legal Go syntax (Go only allows function literals inside functions), but
Ego accepts it as a convenience for organizing helper logic.

**Important:** a nested named function does **not** capture the enclosing
function's scope. It can only see its own parameters, its own local variables,
and package-level or globally-defined names. Accessing a parameter or local
variable of the enclosing named function is a **compile-time error**:

```go
func outer(a int) int {
    func inner(b int) int {
        return a + b   // compile error: nested named function cannot access
                       // enclosing function variable; use a closure
    }
    return inner(5)
}
```

To share the enclosing scope, use a function literal (closure) instead:

```go
func outer(a int) int {
    adder := func(b int) int {
        return a + b   // OK: closure captures 'a' from outer
    }
    return adder(5)
}
```

This behavior differs from what a Go programmer might expect (since Go closures
always capture the enclosing scope), but it is intentional: named functions in
Ego behave like top-level functions — their scope is isolated — regardless of
where they are declared. The nested placement only affects visibility (the
nested function can only be called from within the enclosing function).

### Function Receivers  <a name="function-receivers"></a>

A function can be written such that it can only be used when
referenced via a variable of a specific type. This type is created
by the user, and then the functions that  are written to operate on
that type use any variable of the type as a _receiver_, which means
that variable in the function gets the value of the item in the
function invocation without it being an explicit parameter. This
also allows multiple functions of the same name to exist which
just reference different types. For example,

```go
type Employee struct {                        // (1)
    first string
    last  string
    age   int
}

func (e Employee) Name() string {             // (2)
    return e.first + " " + e.last
}

foo := Employee{                              // (3)
    first: "Bob",
    last:  "Smith"}
    
fmt.Println("The name is ", foo.Name())       // (4)
```

Let's take a look more closely at this example to see what's going
on.

1. This defines the type, called `Employee` which has a number of
   fields associated with it.
2. This declares a function that has a variable `e` of type
   `Employee` as its receiver. The
   job of this function is to form a string of the employee name.
   Note that the name of the variable is not a parameter, but is
   considered the receiver.

3. This creates an _instance_ of the type `Employee` and stores
   it in a variable `foo`.
4. This prints out a label and the value of the `Name` function
   when called using `foo` as
   the variable instance. The function `Name` cannot be called
   directly, it is an unknown symbol unless called with a
   receiver of type `Employee` in its invocation.

In the declaration line line (2) above, you could use an asterisk
("*") character before the receiver's type of `Employee`, as in:

```go
func (e *Employee) SetName(f string, l string)  { 
    e.first = f
    e.last = l
}
```

This function can be used to set the names in the receiver
variable. If the function was declared without the "*" marker,
the receiver would actually only be a copy of the instance. So
the function `Name()` above can read any of the values from the
receiver, if it sets them it is changing a copy of the instance
that only exists while the function is executing, and the
values in the `foo` instance are not changed.

By using the `*` as in the `SetName` example, the receiver `e`
isn't a copy of the instance, it is the actual instance. So
changes made to the receiver `e` will actually be made in the
instance variable `foo`. An easy way to remember this is that,
without the `*`, the receiver does not change the instance that
makes the call, but with the "*" then changes will affect
the instance that makes the call.

Because specifying a receiver variable and type means the name
of the function is scoped to the type itself, you can have
multiple functions with the same name that each has a
different receiver. An example would be to have each type
define a `String()` method whose job is to format the information
in the value to be read by a human. The program can then be
written such that it doesn't matter was the type of an instance
variable. By calling the `String()` function from any instance,
it will execute the _appropriate_ `String()` function based on
the type.

Note that types can be nested. Consider this example:

```go
type EmpInfo struct {
    First string
    Last string
    Age int
}

type Employee struct {
    Info  EmpInfo
    Manager bool
}

e := Employee{
    Info: EmpInfo{
        First: "Bob",
        Last:  "Smith",
        Age:   35,
    },
    Manager: false
}
```

The type `Employee` contains within it an item `Info` which is
of another type, `EmpInfo`. Note that when initializing the
fields of the newly-created instance variable `e`, you must
identify the type name for the `Info` field explicitly, since
it acts as an instance generator itself for an instance of
`EmpInfo` which is then stored in the field `Info` in the
structure.

&nbsp;
&nbsp;

## Error Handling <a name="errors"></a>

There are two kinds of errors that can be managed in an
_Ego_ program.

The first are user- or runtime-generated errors, which are
values of a data type called `error`. You create an error value
using `errors.New()` from the `errors` package:

```go
import "errors"

if v == 0 {
    return errors.New("invalid zero value")
}
```

This code, executed in a function, would return a value
of type `error` that, when printed, shows the message given.
An `error` can also have the value `nil`, which means no error
is present. Runtime functions that can fail return an error as
their last return value; your code checks `err != nil` to detect
a failure:

```go
result, err := someOperation()
if err != nil {
    fmt.Println("Error:", err)
}
```

The second kind are panic error, which are errors generated
by _Ego_ itself while running your program. For example, an
attempt to divide by zero generates a panic error. By default,
this causes the program to stop running and an error message
to be printed out.

### `try` and `catch` <a name="try-catch"></a>

> **Go developer note:** Go uses `defer` + `recover()` inside a function
> to catch panics. Ego replaces that pattern with the simpler `try/catch`
> block shown below. This is an Ego language extension; it has no direct
> Go equivalent.

You can use the `try` statement to run a block of code (in the same
scope as the enclosing statement) and catch any panic errors that
occur during the execution of that block. The error causes Ego
to execute the code in the `catch` clause of the statement.
If there are no errors,  execution continues after the catch block.
For example:

```go
x := 0
try {
    x = pay / hours
} catch {
    fmt.Println("Hours were zero!")
}

fmt.Println("The result is ", x)
```

If the value of `hours` is non-zero, the assignment statement will assign
the dividend to `x`. However, if hours is zero it will trigger a panic
divide-by-zero error. When this happens, the remainder of the statements
(if any) in the `try` block are skipped, and the `catch` block is executed.

You can optionally specify the name of a variable that will be created within
the catch block that contains the actual error encountered. Do this by
adding the name in parenthesis before the catch block.

```go
x := 0
try {
    x = 125 / x
} catch (e) {
    fmt.Println("unexpected error, ", e)
}
```

This can be used in the `catch` block if it needs handle more than one
possible error, for example.

Note that the `catch` clause is optional. If you omit the `catch` clause,
then the error is discarded. Note that the remainder of the `try` block
following when the error occurred isn't executed, but no error is generated
and there is no change in program execution after the `try` block. For example,
this section of code will provide a default value and then try a division
operation. If the operation fails (for example, with a divide-by-zero) then
the default value is unchanged:

```go
x := 1000
try {
    x = a / b
}
```

In this example, if the value of `b` is zero, then the value of `x` remains
set to the value 1000. If the value of `b` is non-zero (and no other errors
occur) the value of x is reset to the value of `a/b`. This is idiomatically
referred to as a "nice try".

### Conditional expression error handling

If you need to catch a possible error in an expression, you can
use a short-form of the `try` and `catch` that works within an
expression. Consider the following example:

```go
emp := { 
    name: "Donna", 
    age: 32,
}

hours := 40
pay := emp.wage * hours
```

This code will generate an error on the statement that attempts
to reference the structure member `wage`, which does not exist.
If you think the field might not exist, or you are doing an
operation that might result in an error (division by zero, perhaps)
that you have a useful default for, use the conditional expression
syntax:

```go
emp := { 
    name: "Donna", 
    age: 32,
}

hours := 40
pay := ?emp.wage : 25.0 * hours
```

The "?" indicates that the following expression component (up to the ":")
is wrapped in a try/catch block. If no error occurs, the expression is
used as specified. But if there is an error ("no such structure field",
for example) then the expression after the ":" is used instead. So in the
above example, because there isn't a `wage` field in this employee's
record, the program assumes a wage of $25/hour in the calculation of
the pay.

### Signaling Errors <a name="signaling"></a>

You can cause a panic error to be signaled from within your
code, which would optionally be caught by a try/catch block,
using the `panic` statement. This is an _Ego_ language extension
and requires that language extensions be enabled via `@extensions true`
or the `ego.compiler.extensions` setting.

```go
if x == 0 {
    panic("Invalid value for x")
}
```

When this code runs, if the value of `x` is zero, then a panic
error is signaled with an error message of "Invalid value
for x". This error is indistinguishable from a panic error
generated by _Ego_ itself. If there is a `catch{}` block, the
value of the `_error_` variable will be the string "Invalid
value for x".

### `throw` <a name="throw"></a>

The `throw` statement is an _Ego_ language extension that raises an
already-constructed `error` value as a catchable runtime error, if (and only
if) it is not `nil`. Like `try`/`catch`, it requires that language extensions
be enabled via `@extensions true` or the `ego.compiler.extensions` setting.

```go
throw expr
```

`expr` must evaluate to an `error` value. If the value is non-nil, it is
raised exactly as if a genuine runtime error (such as division by zero) had
occurred at that point, so it can be caught by an enclosing `try`/`catch`. If
the value is `nil` (or Ego's zero-value error — see the `errors` package
section below), `throw` does nothing and execution continues with the next
statement.

This makes `throw` a single-statement replacement for Go's common
`if err != nil { return err }` idiom, used purely to propagate an error
condition rather than to return a specific value:

```go
func mustBePositive(n int) {
    throw validate(n)
}

func validate(n int) error {
    if n < 0 {
        return errors.New("value must not be negative")
    }

    return nil
}
```

Calling `mustBePositive(-1)` from inside a `try` block raises the error
returned by `validate`, which an enclosing `catch` clause can inspect:

```go
try {
    mustBePositive(-1)
} catch (e) {
    fmt.Println("rejected:", e.Error())
}
```

Calling `mustBePositive(5)` does nothing, since `validate` returns `nil` for
a non-negative value — the `throw` statement is a no-op in that case.

&nbsp;
&nbsp;

## Threads <a name="threads"></a>

Like its inspiration, _Ego_ supports the idea of "go routines" which are threads
that can be started by an _Ego_ program, and which run asynchronously. A go routine
is always a function invocation, or a function constant value. That function is
started on a parallel thread, and will execute independently of the main program.

You can use _channels_ as a communication mechanism to communicate between the
go routines and the main program.

### Go

Use the `go` statement to start a thread. Here is a very simple example:

```go
func beepLater(duration string) {
    d, _ := time.ParseDuration(duration)
    time.Sleep(d)
    fmt.Println("BEEP!")
}

go beepLater("1s")
```

This example defines a function `beepLater` which is given a duration
string expression. The function converts the string expression of a
duration to a native time.Duration value, and then waits for that
duration of time to pass before printing the message to the console.

The `go` statement starts this thread, passing it the parameters from
the current scope, which are copied to the thread and stored in the
`duration` variable on that thread.

Note that this isn't a very interesting example, but worse; it shows an
issue with running go routines. If the program above is the only code
being executed in the program, it will produce no output. This is because
the main program ends (completes all statements) and that terminates
the _Ego_ session, even if a thread is still running. The thread is not
guaranteed to be allowed to run to completion if the program that
starts it finishes.

### Synchronization

Ego provides several data types used to synchronize execution of competing
threads, and to assist in managing access to resources in a predictable
way if needed.

| Datatype | Description |
| :------- | :---------- |
| sync.Mutex | A simple mutual exclusion lock for serializing access to a resource |
| sync.RWMutex | A reader/writer lock: any number of concurrent readers, or a single exclusive writer |
| sync.WaitGroup | A way to launch a varying number of go routines and wait for them to complete |

See the detailed descriptions in the later sections on the `sync` package
for more information.

### Channels

We address this synchronization issue (and also allow data to be
passed _back_ from the go routine) using channels. Here's a modified
version of the program:

```go
func beepLater(duration string, c chan) {
    d, _ := time.ParseDuration(duration)
    time.Sleep(d)
    c <- "BEEP"
}

var xc chan
go beepLater("1s", xc)

m := <- xc

fmt.Println(m)
```

In this example, the main program defines a variable `cx` which
is a _channel_ variable, of type `chan`. The duration and the channel
variable are passed to the go routine. Importantly, the program then
receives data from the channel, using the `<-` notation. This causes the
main program to wait until a message is put into the channel, and that
message is stored in the variable m

Meanwhile, the go routine starts running, and performs the wait as
before. Once the wait is completed, it puts a message (really, any
value) in the channel, again using a variant of the `<-` syntax to
show writing a value into a channel. When this write occurs, the
main program's receive operation completes and the message is
printed.

In this way, the go function performs its work, and then sends the
result back through the channel. The main program will wait for data
to be stored in the channel before proceeding. The go routine can
send more than one data item into the channel, simply by issuing
more channel write operations. The receiver can either know how
many times to read the channel, or can use a `for...range` operation
on the channel to simply keep receiving data until done.

```go
func beepLater(count int, c chan) {
    for i := 0; i < count; i = i + 1 {
        c <- "Item " + string(i)
    }
}

var xc chan
go beepLater(5, xc)

for msg := range xc {
    fmt.Println("Received ", msg)
}
```

In this case, the caller of the goroutine includes a count of the
number of messages to send, and that function sends that many
messages. The main program uses the `range` operation on the channel,
which means "_as many as you receive_" where each message is stored
in `msg`. The loop will terminate when the goroutine stops executing.
The goroutine can also explicitly tell the main program that it is
done by using the `close()` function on the channel. When this happens,
the range loop exits. Note that both the main program and the goroutine
will continue executing to the end even after the channel is closed.

A receive can also tell you whether the channel is still open, using the
two-value ("comma-ok") form:

```go
value, ok := <-xc
```

`ok` is `false` when the channel has been closed and fully drained (in which
case `value` is the zero value), and `true` otherwise. This is the usual way
to detect channel closure without relying on `for...range`.

`<-xc` is not limited to being the entire right-hand side of an assignment —
it works anywhere a value is expected, such as a function-call argument or an
operand of another operator:

```go
fmt.Println(<-xc)              // pass the received value directly to a function
greeting := "Got: " + <-xc     // use it as an operand
```

A function can also create and return a channel, which is a common way to
hand a caller a communication endpoint without exposing how it is populated:

```go
func requestQueue() chan {
    q := make(chan, 10)

    go func() {
        for i := 0; i < 3; i = i + 1 {
            q <- "job " + string(i)
        }
        close(q)
    }()

    return q
}

jobs := requestQueue()
for job := range jobs {
    fmt.Println("Processing", job)
}
```

A channel is a value like any other, so it can be stored in a struct field,
an array element, or a map value, and sent to or received from through that
storage location directly:

```go
type Worker struct {
    Name  string
    Inbox chan
}

w := Worker{Name: "w1", Inbox: make(chan, 5)}
w.Inbox <- "task 1"
fmt.Println(w.Name, "received", <-w.Inbox)

queues := make([]chan, 3)
for i := 0; i < 3; i = i + 1 {
    queues[i] = make(chan, 1)
    queues[i] <- i
}
fmt.Println(<-queues[0], <-queues[1], <-queues[2])

registry := make(map[string]chan)
registry["alerts"] = make(chan, 1)
registry["alerts"] <- "fire"
fmt.Println(<-registry["alerts"])
```

**A channel never has an element type — write `chan` alone, always, no
matter where the type appears.** Unlike Go, where `chan string` declares a
channel that only carries strings, Ego channels can carry any value and have
no per-channel element type at all. This rule is the same everywhere a type
is written: a `var` declaration, a function parameter, a function's return
type (including inside a parenthesized multi-value return list), a struct
field, `make()`, or a type-assertion target. Writing `chan T` in any of
these positions is a compile-time error:

```go
var xc chan string    // error: channels do not have an element type;
                       // use "chan" alone, not "chan T"
```

&nbsp;
&nbsp;

## Packages <a name="packages"></a>

Packages are a mechanism for grouping related functions together. These
functions are accessed using _dot notation_ to reference the package name
and then locate the function within that package to call.

Packages may be available to your program automatically if the `ego.compiler.auto-import`
preference is set to true. If not, you must import each package before you can use it.
Additionally, packages can be created by the user to extend the runtime support for
_Ego_; this is covered later.

### import <a name="import"></a>

Use the `import` statement to include other files in the compilation
of this program. The `import` statement cannot appear within any other
block or function definition. Logically, the statement stops the
current compilation, compiles the named object (adding any function
and constant definitions to the named package) and then resuming the
in-progress compilation.

```go
import factor
import "factor"
import "factor.ego"
```

All three of these have the same effect. The first assumes a file named
"factor.ego" is found in the current directory. The second and third
examples assume the quoted string contains a file path. If the suffix
".ego" is not included it is assumed.

You can optionally specify an alias for the import package by putting
an identifier before the package name or path as a string. For example,

```go
import str "strings"
```

In this example, the "strings" package is imported to the current program,
but will be referenced using the name `str` in the code. This allows you to
import multiple packages that would have the same name and use the alias
to define them unambiguously.

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

The following sections will describe the _built-in_ packages that are
provided automatically as part of Ego. You can extend the packages
by writing your own, as described later in the section on User Packages.

---

### base64 Package <a name="base64"></a>

The `base64` package supports encoding and decoding strings using standard
Base64 encoding (RFC 4648).

#### base64.Encode(data string) string

The `Encode` function encodes a string value as a Base64-encoded string using
standard encoding rules. For example,

```go
s := base64.Encode("Hello, World!")
```

produces the string `"SGVsbG8sIFdvcmxkIQ=="`.

#### base64.Decode(data string) (string, error)

The `Decode` function decodes a Base64-encoded string and returns the original
string value. If the input is not valid Base64, an error is returned. For
example,

```go
s, err := base64.Decode("SGVsbG8sIFdvcmxkIQ==")
```

produces the string `"Hello, World!"`.

---

### cipher Package <a name="cipher"></a>

The `cipher` package provides cryptographic primitives: one-way hashing, symmetric
encryption, cryptographically-random byte strings, in-place "sealing" of sensitive
string variables, and creation/validation of signed authentication tokens (the same
kind of token the Ego server issues to REST clients after a successful logon).

#### cipher.Hash(text string) string

Computes a SHA-256 digest of `text` and returns it as a 64-character lowercase
hexadecimal string. The digest is one-way (irreversible); the same input always
produces the same output.

```go
h := cipher.Hash("Hello, World!")
fmt.Println(h)
// dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
```

This is a general-purpose content digest — do not use it to store passwords.

#### cipher.Encrypt(text, key string) (string, error)

Encrypts `text` using `key` as the passphrase and returns the result as a
hexadecimal string. Internally this uses AES-256-GCM with an Argon2id-derived
key and a random salt and nonce, so encrypting the same text twice with the
same key produces different output each time. An error is returned only if
the underlying cryptographic operation itself fails (for example, if the
system's secure random source is unavailable).

```go
e, err := cipher.Encrypt("Hello, World!", "my secret key")
```

#### cipher.Decrypt(encryptedText, key string) (string, error)

Reverses `cipher.Encrypt()`, given the same `key` that was used to encrypt the
text. Returns an error if `encryptedText` is not a valid hexadecimal string,
or if decryption fails (wrong key, or the ciphertext was modified).

```go
e, _ := cipher.Encrypt("Hello, World!", "my secret key")
d, err := cipher.Decrypt(e, "my secret key")
fmt.Println(d)
// Hello, World!

_, err = cipher.Decrypt(e, "wrong key")
fmt.Println(err != nil)
// true
```

`Decrypt` also recognizes ciphertext produced by older, retired key-derivation
schemes, so text encrypted by a previous version of Ego continues to decrypt
correctly.

#### cipher.Random([bits int]) (string, error)

Generates a cryptographically random value and returns it as a URL-safe
Base64-encoded string. `bits` is the number of random bytes to generate; if
omitted, 32 bytes are generated. An error is returned only if the system's
secure random source is unavailable.

```go
r, err := cipher.Random()     // 32 random bytes, ~44-character result
r8, err := cipher.Random(8)   // 8 random bytes, ~12-character result
```

#### cipher.Seal(&text string) (string, error) / cipher.Unseal(sealedText string) string

`Seal` encrypts the string referenced by the pointer argument and returns the
result. As a side effect, the original variable is set to the empty string,
so the plaintext no longer lingers in a variable that might be logged or
inspected later. `Unseal` reverses the process. `Seal` returns an error if
its argument is not a pointer to a string.

```go
secret := "top secret"
sealed, err := cipher.Seal(&secret)

fmt.Println(secret)             // "" -- the original was cleared
fmt.Println(cipher.Unseal(sealed))  // "top secret"
```

Sealing the same string twice produces different sealed output each time
(the underlying encryption uses a random salt and nonce), but both unseal
back to the same original text.

#### cipher.New(name string [, data string [, expiration string]]) (string, error)

Creates a new signed authentication token containing `name` (typically a
username) and an optional `data` payload, and returns it as an opaque,
hex-encoded string suitable for use as a bearer token. `expiration` is a Go
duration string (e.g. `"15m"`, `"2h"`, `"24h"`); it defaults to the server's
configured token expiration, or `"15m"` if that has not been set. An error is
returned if `expiration` cannot be parsed as a duration.

```go
token, err := cipher.New("alice", "role=admin", "1h")
```

#### cipher.Validate(token string) bool

Reports whether `token` (as produced by `cipher.New()`) is currently valid —
correctly signed, not expired, and not blacklisted. It never returns an
error; a malformed or expired token simply validates as `false`. Use
`cipher.Extract()` instead when you need to know _why_ a token failed
validation.

```go
token, _ := cipher.New("alice")

if cipher.Validate(token) {
    fmt.Println("token is valid")
}
```

#### cipher.Extract(token string) (cipher.Token, error)

Decodes `token` and returns its contents as a `cipher.Token` struct. Returns
an error explaining _why_ the token is invalid if it is malformed, tampered
with, or expired.

```go
type Token struct {
    Name    string  // the name/identity the token was created for
    Data    string  // the optional data payload passed to New()
    TokenID string  // a unique ID for this specific token
    AuthID  string  // the UUID of the server instance that issued the token
    Expires string  // the token's expiration time, formatted per RFC 822 with zone
}
```

```go
token, _ := cipher.New("alice", "role=admin")

info, err := cipher.Extract(token)
if err == nil {
    fmt.Println(info.Name, info.Data)
    // alice role=admin
}
```

---

### cmplx Package<a name="cmplx"></a>

The `cmplx` package provides complex-number math functions, mirroring Go's standard
`math/cmplx` package. Unlike `math` (which is called with either `float32` or `float64`
values), every function here operates on `complex128` only -- Go's own `math/cmplx` has
no `complex64` variant either. Pass a `complex64` value through an explicit
`complex128(x)` cast first if needed.

See [Imaginary literals](#imaginary-literals) for how to write complex number constants
(`3+4i`), and the [`complex()`/`real()`/`imag()`](#builtInFunctions) builtins for
constructing a complex value from two real numbers or extracting its components.

#### cmplx.Abs(x) / cmplx.Phase(x)

```go
func cmplx.Abs(x complex128) float64
func cmplx.Phase(x complex128) float64
```

`Abs()` returns the absolute value (modulus) of `x`. `Phase()` returns the phase (angle,
in radians) of `x`.

```go
fmt.Println(cmplx.Abs(3+4i))     // 5
```

#### cmplx.Conj(x)

```go
func cmplx.Conj(x complex128) complex128
```

Returns the complex conjugate of `x` (the imaginary part negated).

```go
fmt.Println(cmplx.Conj(3+4i))    // (3-4i)
```

#### cmplx.Sqrt(x) / cmplx.Exp(x) / cmplx.Log(x) / cmplx.Log10(x)

```go
func cmplx.Sqrt(x complex128) complex128
func cmplx.Exp(x complex128) complex128
func cmplx.Log(x complex128) complex128
func cmplx.Log10(x complex128) complex128
```

`Sqrt()` returns the complex square root of `x` -- unlike `math.Sqrt`, this accepts a
negative real number without error, since the square root of a negative number is a
valid complex value:

```go
fmt.Println(cmplx.Sqrt(-1))      // (0+1i)
```

`Exp()`, `Log()`, and `Log10()` are the complex versions of the base, natural, and
base-10 exponential/logarithm functions.

#### cmplx.Sin(x) / cmplx.Cos(x) / cmplx.Tan(x)

```go
func cmplx.Sin(x complex128) complex128
func cmplx.Cos(x complex128) complex128
func cmplx.Tan(x complex128) complex128
```

The complex versions of the standard trigonometric functions.

#### cmplx.Polar(x) / cmplx.Rect(r, theta)

```go
func cmplx.Polar(x complex128) (r, theta float64)
func cmplx.Rect(r, theta float64) complex128
```

`Polar()` converts a complex value to polar coordinates: `r` (the modulus, same as
`Abs(x)`) and `theta` (the phase angle in radians, same as `Phase(x)`). `Rect()` is the
inverse: it builds a complex value from polar coordinates.

```go
r, theta := cmplx.Polar(1 + 1i)
fmt.Println(cmplx.Rect(r, theta))   // (1+1i), modulo floating-point rounding
```

#### cmplx.Pow(x, y)

```go
func cmplx.Pow(x, y complex128) complex128
```

Returns `x` raised to the complex power `y`.

#### cmplx.IsNaN(x) / cmplx.IsInf(x) / cmplx.NaN() / cmplx.Inf()

```go
func cmplx.IsNaN(x complex128) bool
func cmplx.IsInf(x complex128) bool
func cmplx.NaN() complex128
func cmplx.Inf() complex128
```

`IsNaN()` reports whether either the real or imaginary part of `x` is NaN (and neither is
an infinity). `IsInf()` reports whether either part is an infinity. `NaN()` and `Inf()`
return a complex value with both parts set to NaN or positive infinity, respectively.
Note that these functions deal with Go's native IEEE special values directly -- they are
unrelated to _Ego_'s own choice to raise a runtime error for complex division by zero
rather than producing one of these values (see [Imaginary literals](#imaginary-literals)).

---

### errors Package<a name="error"></a>

The `errors` package implements simple error types. There is a single method, `New`, which
is used to create a new error. The resulting error has a number of functions that can be
accessed.

#### errors.New()

The `New` function creates a new instance of an error. The first parameter is the text
of the error message, which must be a string value.

The optional second value is a context value, which is stored in the error. This is
displayed when the error is formatted, and can be retrieved using the Unwrap() function.

```go

    var e error

    fn := "foobar.txt"
    e = errors.New("not found", fn)

```

This results in creating a new error with a string value of "not found" and a context
value of the fn variable. This will be appended to the error message when it is formatted.
If a context value is not supplied (i.e. only one argument is passed to `New`) then there
is no context value output.

#### (e error) Error() string

The `Error()` function can be used with any error as the receiver value, and will
generate a textual representation of the error.

```go

    var e error

    fn := "foobar.txt"
    e = errors.New("not found", fn)

    m := e.Error()

```

After this code executes, `m` will contain the string value "not found: foobar.txt".

#### (e error) Is(other error) bool

The `Is()` function can be used with any error as the receiver value, and will
compare the error to the provided parameter which is also an error value. This
lets you compare error messages to see if they match. Note that this does not
compare the context, only the actual error message.

```go
e1 := errors.New("not found")
e2 := errors.New("not found").Context("foobar.txt")
e3 := errors.New("different message")

fmt.Println(e1.Is(e2))  // true -- same message, context is ignored
fmt.Println(e1.Is(e3))  // false -- different message
```

#### (e error) Unwrap() any

The `Unwrap()` function can be used with any error as the receiver value, and will
return the context value stored in the error. If there is no context, the result is
`nil`.

```go
var e error

fn := "foobar.txt"
e = errors.New("not found", fn)

f2 := e.Unwrap()
```

After this code executes, `f2` will contain the string value "foobar.txt".

#### (e error) Code() string

The `Code()` function can be used with any error as the receiver value, and will
return the Ego localizable error code string, such as "div.zero" for a "division
by zero" error. If the error is not an Ego error, but instead is an error from
the operating system, the code string will be "not.an.ego.error".

```go
try {
    x := hours/ 0.0
} catch (e) {
    code := e.Code()    // Will be "div.zero"
    fmt.Printf("The code is %s\n", code)
}
```

#### (e error) Next() error

The `Next()` function can be used with any error as the receiver value, and will
return the next error that is chained on this error. Errors from the system may
contain a list of errors. The `Next()` function gets the next error in the list
from the current error. If there is no next error, it returns `nil`.

#### (e error) Context(v any) error

The `Context()` function can be used with any error as the receiver value, and
returns a new error with the context value set to `v`. This is an alternate way
of attaching the context value described under `errors.New()` above -- useful
when the context is not known until after the error was created. Any previous
context value on the error is replaced. Like all of the error-derivation
functions on this page, `Context()` does not modify the receiver; it returns a
distinct error value, leaving the original unchanged.

```go
e := errors.New("not found")
e2 := e.Context("foobar.txt")

fmt.Println(e.Error())   // "not found"
fmt.Println(e2.Error())  // "not found: foobar.txt"
```

#### (e error) In(name string) error

The `In()` function can be used with any error as the receiver value, and
returns a new error with the location name set to `name`. This is typically
the name of a source file, package, or function, and is displayed as part of
the formatted error message. `In()` does not modify the receiver; it returns a
distinct error value.

```go
e := errors.New("not found")
e2 := e.In("readFile")

fmt.Println(e2.Error())  // "in readFile, not found"
```

#### (e error) At(line int) error

The `At()` function can be used with any error as the receiver value, and
returns a new error with the location line number set to `line`. This is
combined with the location name (see `In()`) when the error is formatted.
`At()` does not modify the receiver; it returns a distinct error value.

```go
e := errors.New("not found").In("readFile").At(42)

fmt.Println(e.Error())  // "at readFile(line 42), not found"
```

---

### exec Package<a name="exec"></a>

The `exec` package is a subset of the Go package that supports executing a command as
a subprocess of the current Ego program. This package allows the caller to create a
new `exec.Cmd` object, and then use that object to optionally set arguments, an
environment, a working directory, and stdin values for the command, execute the
command, and then access the stdout (and, for `Output()`, stderr) values.

Subprocess execution is a privileged operation. It is gated by the
`ego.runtime.exec` configuration setting (default `true`), and is unconditionally
disabled for sandboxed contexts (for example, code run through the server's admin
"run" dashboard endpoint on behalf of a non-admin user), regardless of that
setting. If execution is not permitted, every function and method in this package
returns the error `no privilege for operation`.

#### The `Cmd` structure

`exec.Command()` (below) returns a `Cmd` object with these fields:

| Field | Type | Description |
| ----- | ---- | ----------- |
| `Path` | `string` | Full resolved path of the command to run. Set automatically by `Command()`; can be overwritten before calling `Run()`/`Output()` to force a specific executable. |
| `Dir` | `string` | Working directory for the subprocess. If empty (the default), the subprocess inherits the current process's working directory. |
| `Args` | `[]string` | Command-line arguments, including the command name itself as `Args[0]` (matching Go's `os/exec.Cmd.Args` convention). Populated by `Command()`; can be modified before running. |
| `Env` | `[]string` | Environment variables for the subprocess, each formatted as `"name=value"`. If left empty, the subprocess does not inherit any variables from the Ego process's environment -- set this explicitly (typically alongside values read via `os.Getenv()` or `os.Environ()`) if the subprocess needs access to the caller's environment. |
| `Stdin` | `[]string` | Optional. If set before running, each element is joined with a newline and fed to the subprocess as its standard input. |
| `Stdout` | `[]string` | Populated after `Run()` or `Output()` completes: one element per line of the subprocess's standard output. |
| `Stderr` | `[]string` | Populated after `Output()` completes (empty on success): one element per line of the subprocess's standard error. **Not** populated by `Run()` -- see below. |

#### exec.Command()

```go
func exec.Command(commandText string, argument ...string) exec.Cmd
```

The `Command()` function creates a new `Cmd` object and returns it to the caller.
The first parameter is the name of the command to execute; it is resolved using the
same search rules as `exec.LookPath()` (below). Any additional string arguments are
passed to the program as its command-line arguments.

```go
c := exec.Command("ls", "-l", "/tmp")
```

This creates (but does not yet run) a command that will invoke `ls -l /tmp`. Note
that these are Unix-style commands; you would use Windows-style commands on a
Windows-based deployment of _Ego_.

#### Cmd.Run()

```go
func (c exec.Cmd) Run() error
```

Runs the command represented by `c`. The subprocess's standard input is taken from
`c.Stdin` if set; on completion, `c.Stdout` is set to the lines of standard output
produced by the command.

```go
func main() {
    c := exec.Command("ls", "-l")

    if err := c.Run(); err != nil {
        fmt.Println("command failed:", err)
        return
    }

    for _, line := range c.Stdout {
        fmt.Println(line)
    }
}
```

Also note that `Stdout` is a raw split of the captured output on newline boundaries,
so a trailing newline in the command's output produces a trailing empty string
element -- unlike `Output()`, which trims it (see below).

#### Cmd.Output()

```go
func (c exec.Cmd) Output() ([]string, error)
```

Runs the command represented by `c` and returns its standard output as a string
array, along with an error. Unlike `Run()`, this method returns the conventional
`(value, error)` pair, so the error can be checked normally without `try`/`catch`:

```go
c := exec.Command("git", "rev-parse", "HEAD")

out, err := c.Output()
if err != nil {
    fmt.Println("git failed:", err)
    return
}

fmt.Println("HEAD is at", out[0])
```

`Output()` also captures standard error separately (something `Run()` does not do),
storing it as a string array in `c.Stderr`. On success, `c.Stderr` is set to an
empty array. On failure, the returned string array and `c.Stdout` reflect whatever
standard output the command produced before it failed (which may be empty), and
`c.Stderr` holds the lines of error output. This matches the _behavior_ of Go's own
`os/exec.Cmd.Output()`, which likewise returns partial stdout alongside the error --
though Go exposes the captured stderr text via the `Stderr` field of the returned
`*exec.ExitError` rather than a field on `Cmd` itself:

```go
c := exec.Command("sh", "-c", "echo partial output; echo failure detail 1>&2; exit 1")

out, err := c.Output()
if err != nil {
    fmt.Println("error:", err)          // "error: exit status 1"
    fmt.Println("stdout:", out)          // ["partial output"]
    fmt.Println("stderr:", c.Stderr)     // ["failure detail"]
}
```

A trailing newline in either stream does not produce a trailing empty string
element -- `Output()` trims one trailing blank line from both `Stdout` and `Stderr`,
unlike `Run()`.

#### exec.LookPath()

```go
func exec.LookPath(file string) (string, error)
```

Searches the directories named by the `PATH` environment variable for an executable
named `file`, and returns its resolved path. If `file` contains a slash, it is used
directly (after verifying it is executable) rather than searched for. If no
executable is found, the second return value is a non-nil error and the first
return value is an empty string.

```go
path, err := exec.LookPath("git")
if err != nil {
    fmt.Println("git is not installed:", err)
} else {
    fmt.Println("found git at", path)
}
```

`exec.Command()` internally performs the equivalent of a `LookPath()` call to
resolve `Path` from the command name passed to it, so most programs do not need to
call `LookPath()` directly unless they want to test for a command's existence
before attempting to run it.

---

### filepath Package<a name="filepath"></a>

The `filepath` package is a thin, native passthrough to Go's standard
`path/filepath` package, so it behaves identically to Go for path
manipulation (string operations only — none of these functions touch the
filesystem, except `Abs`, which consults the current working directory).

All six functions accept a `path`/`partialPath`/`elements` argument that is
subject to sandbox path containment when running in a sandboxed context (see
`ego.runtime.sandbox.path` and the `@sandbox` test directive in
`docs/TESTING.md`): a sandboxed program cannot use `..` segments, an absolute
path, or any other trick to make these functions compute a path outside the
configured sandbox root.

#### filepath.Base(path string) string

Returns the last element of `path`. Trailing slashes are removed before
extracting the last element; if `path` is empty, returns `"."`.

```go
filepath.Base("/a/b/c.txt")   // "c.txt"
filepath.Base("/a/b/")        // "b"
```

#### filepath.Dir(path string) string

Returns all but the last element of `path`, effectively the directory
containing `path`.

```go
filepath.Dir("/a/b/c.txt")    // "/a/b"
```

#### filepath.Ext(path string) string

Returns the file extension of `path` — the suffix beginning at the last dot
in the final path element — or `""` if there is no dot.

```go
filepath.Ext("/a/b/c.txt")    // ".txt"
filepath.Ext("/a/b/c")        // ""
```

#### filepath.Clean(path string) string

Returns the shortest equivalent path by lexically resolving `.` and `..`
elements and removing redundant separators, following the same rules as
Go's `filepath.Clean`.

```go
filepath.Clean("/a/b/../c.txt")   // "/a/c.txt"
```

#### filepath.Join(elements... string) string

Joins any number of path elements into a single path with the operating
system's separator, then cleans the result the same way `Clean` does.

```go
filepath.Join("a", "b", "c.txt")   // "a/b/c.txt"
```

#### filepath.Abs(partialPath string) (string, error)

Returns an absolute representation of `partialPath`. If `partialPath` is not
already absolute, it is joined with the current working directory. The
result is `Clean`ed. An error is returned only if the current working
directory cannot be determined.

```go
p, err := filepath.Abs("c.txt")
```

---

### fmt Package<a name="fmt"></a>

The `fmt` package contains a function library for formatting and printing output to the
stdout console. These are generally analogous to the Go functions of the same name. Some
functions return two values (a result or length, and an error). If the caller does not
specify that the result is assigned to two variables, then the error is ignored.

Note that only a subset of the equivalent Go functions are supported in _Ego_.

#### fmt.Printf()

The `Printf` function formats one or more values using a format string, and sends the
resulting string to the standard out. It returns the length in characters of the
string written to the output, and an error which will be nil if no error occurred during
format processing.

```go
answer := 42
kind := "correct"
count, err := fmt.Printf("The %s answer is %d\n", kind, answer)
```

In this example, the format string is processed, and the substitution format operators
read parameters (in the order encountered) from the call. So the first operator `%s`
looks for a string value in the variable `kind` and inserts it into the message. It
uses the second operator `%d` to indicate that it is looking for an integer value which
is inserted in the string using the value of `answer`.

See the [official Go documentation](https://golang.org/pkg/fmt/#hdr-Printing) for
detailed information on the format operators supported by the fmt package.

#### fmt.Print()

The `Print` function prints one or more items using the default format for their data
type to the standard out. There are no formatting operations available, and no
newline is added at the end.

```go
fmt.Print("The answer is ", 42)
```

Unlike `Println` (below), `Print` does **not** always place a space between the items
it prints. A space is inserted between two adjacent items only when **neither** of
them is a string — matching Go's own `fmt.Print` behavior exactly:

```go
fmt.Print("hello", "world")   // "helloworld"  -- both strings, no space
fmt.Print(1, 2)                // "1 2"         -- neither is a string, space added
fmt.Print("hello", 42)         // "hello42"     -- one is a string, no space
```

This rule exists so that concatenating string fragments with `Print` does not
introduce unwanted spaces, while numeric or other non-string values are still kept
visually distinct from each other. If you want a space between every item regardless
of type, use `Println` or `Sprint`+`" "` (or, more simply, just include the space you
want directly in a string argument, as in the first example above).

#### fmt.Println()

The `Println` function prints one or more items using the default format for their
data type to the standard out. The output is followed by a newline character. It
follows the same spacing rules as `fmt.Print()`. There are no formatting operations
available.

```go
answer := 42

fmt.Println("The answer is", answer)
```

This results in the string `"The answer is 42"` followed by a newline character being
send to the output console.

#### fmt.Sscanf()

The `Sscanf()` function accepts a string of data, a format specification, and one or
more pointers to base-type values. The data string is processed using the format
specification, and the resulting values are written to the parameter variables.
The function returns the number of items processed, and any error (such as invalid
value for a given format).

```go
var age int
var temp float64

data := "35 101.2"

fmt.Sscanf(data, "%d%f", &age, &temp)
```

The `%d` format specification causes an integer value to be parsed from the string.
This is followed by a floating pointer number. These are stored in `age` and `temp`
respectively.

Any non-format characters in the format string must be present in the input string
exactly as shown. For example,

```go
data := "age 35 temp 101.2"

fmt.Sscanf(data, "age %d temp %f", &age, &temp)
```

Note that in both the data string and the format string, multiple white-space
characters (" ", etc) are ignored. The supported format values are:

&nbsp;

| Format | Description |
| :------: | :--------------------------------- |
| %t | Boolean defs.True or defs.False value |
| %f | Floating point value |
| %d | Integer value |
| %s | String values |

&nbsp;

Note that this is a subset of the format operations supported by Go's runtime.
Also note that _Ego_ does not support a width specification in the format.

#### fmt.Sprint()

The `Sprint()` function works exactly the same as the `Print()` function, but returns
the formatted string as its result value instead of printing it anywhere. It follows
the identical spacing rule: a space is inserted between two adjacent items only when
neither of them is a string.

```go
s1 := fmt.Sprint("hello", "world")   // "helloworld"
s2 := fmt.Sprint(1, 2)                // "1 2"
s3 := fmt.Sprint("Count: ", 5)        // "Count: 5"
```

This is useful for building a string value out of mixed-type items without having to
write an explicit format string, the way `Sprintf` requires. If you do want an
explicit format string, use `Sprintf` instead.

#### fmt.Sprintf()

The `Sprintf()` function works exactly the same as the `Printf()` function, but returns
the formatted string as its result value, instead of printing it anywhere. This lets
you use the formatting operations to construct a string value that includes other
values in the string.

```go
v := "foobar"
msg := fmt.Sprintf("Unrecognized value %s", v)
```

This creates a string named `msg` which contains "Unrecognized value foobar" as its
contents. The value is not printed to the console as part of this operation.

---

### http Package<a name="http"></a>

The `http` package provides the types used to write REST service endpoints hosted by
the Ego server: `Request` (the incoming request, passed as a handler's first argument),
`ResponseWriter` (used to build the response, passed as the second argument), `Header`
(the response's HTTP headers), plus the standard Go `net/http` status-code and
method-name constants. There is no client-side equivalent here -- to make outbound HTTP
calls from Ego code, use the [`rest` package](#rest) instead.

This package is meaningful only inside a service handler running under `ego server`; see
`docs/SERVER.md` for how a service file is structured and how `@endpoint` maps a URL to a
handler function. A handler function always has this shape:

```go
import "http"

func handler(req http.Request, w *http.ResponseWriter) {
    // ... inspect req, write a response via w ...
}
```

#### The `http.Request` structure

`req` describes everything known about the incoming request. All fields are read-only.

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `URL` | `http.URL` | The request URL, broken into `Path` (string) and `Parts` (see below). |
| `Endpoint` | `string` | The service's own endpoint path (the `@endpoint path=` value, without any `{{name}}` substitutions applied). |
| `Method` | `string` | The HTTP method, e.g. `"GET"`, `"POST"`. Compare against the `http.MethodXxx` constants rather than a literal string. |
| `Headers` | `map[string][]string` | The request's HTTP headers. The `Authorization` header is never included here. |
| `Parameters` | `map[string][]string` | The request's query-string parameters. |
| `Body` | `string` | The raw request body (for `POST`/`PUT`/`PATCH`/`DELETE` requests that included one). |
| `Username` | `string` | The authenticated caller's username, or `""` if the request was not authenticated. |
| `Authenticated` | `bool` | `true` if the caller was authenticated at all (by token or username/password), regardless of what permissions they hold. |
| `Authentication` | `string` | How the caller authenticated: `"none"`, `"user"` (username/password), or `"token"` (bearer token). |
| `IsAdmin` | `bool` | `true` if the caller holds `ego.root` (admin) privileges. |
| `IsJSON` | `bool` | `true` if the caller's `Accept` header indicates it wants a JSON response. |
| `IsText` | `bool` | `true` if the caller's `Accept` header indicates it wants a plain-text response. |
| `Permissions` | `[]string` | The authenticated caller's permission strings. |
| `SessionID` | `int` | The server's internal session/request number for this call, useful for correlating with server log entries. |
| `Router` | `interface{}` | The server's internal route dispatcher. Reserved for internal use -- there is no supported way to call into it from a handler. |

`req.URL` is itself a small structure:

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Path` | `string` | The request's path, **including its query string** (despite the field name) -- e.g. `/services/widgets?verbose=true`. |
| `Parts` | `map[string]interface{}` | One entry per `{{name}}` placeholder declared in the endpoint's `@endpoint path=`, keyed by name, holding the text captured from the actual request path. |

```go
@endpoint get path="/services/sample/users/{{name}}/{{field}}"

import "http"

func handler(req http.Request, w *http.ResponseWriter) {
    name := req.URL.Parts["name"]
    field := req.URL.Parts["field"]
    ...
}
```

`IsJSON`/`IsText` reflect content negotiation against the endpoint's own `media=`
declaration (see `docs/SERVER.md`'s `@endpoint` documentation): if the caller's `Accept`
header doesn't match any type the endpoint declared, the router rejects the request with
400 before the handler ever runs, so a handler only ever sees a type it already agreed to
serve.

#### The `http.ResponseWriter` structure

`w` is used to build the response. It has no directly readable fields -- everything is
done through its methods. Methods are commonly chained in the order shown here: set
headers, set the status, then write the body.

| Method | Description |
| :------- | :---------- |
| `Header() http.Header` | Returns the response's `Header` object, for adding/replacing/removing response headers. |
| `WriteHeader(status int)` | Sets the HTTP response status code. If never called, the response defaults to `200 OK`. |
| `Write(data interface{}) (int, error)` | Writes `data` to the response body and returns the number of bytes written. See below for how `data`'s type affects the output. |
| `WriteJSON(i interface{})` | Marshals `i` to JSON and writes it to the body -- shorthand for `json.Marshal` followed by `Write`. |
| `WriteTemplate(filename string, i interface{})` | Renders the named template file (relative to `ego.runtime.path`) using `i` as the template data, and writes the result to the body. On error, writes a `500` response with the error text instead. |

`Write()`'s behavior depends on the type of `data`:

- A `[]byte` value is written to the body exactly as given.
- Any other value is formatted according to the request's negotiated content type: as
  JSON (via `json.Marshal`) if `req.IsJSON`, otherwise as plain text (its default string
  formatting, with a trailing newline). If both are accepted, JSON takes precedence.

```go
import "http"

func handler(req http.Request, w *http.ResponseWriter) {
    w.Header().Add("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.WriteJSON({message: "hello", count: 3})
}
```

`Header()` returns an `http.Header`, which wraps the response's `map[string][]string` of
headers with methods matching Go's `net/http.Header`:

| Method | Description |
| :------- | :---------- |
| `Add(key, value string)` | Appends `value` to any existing values already set for `key`. |
| `Set(key, value string)` | Replaces all existing values for `key` with `value`. |
| `Del(key string)` | Removes `key` entirely. |

#### Status code and method constants

The `http` package defines the same status-code and method-name constants as Go's
`net/http`, for use with `w.WriteHeader()` and when comparing `req.Method`:

```go
if req.Method != http.MethodGet {
    w.WriteHeader(http.StatusMethodNotAllowed)
    return
}

w.WriteHeader(http.StatusOK)
```

`http.StatusOK` (`200`), `http.StatusBadRequest` (`400`), `http.StatusUnauthorized`
(`401`), `http.StatusForbidden` (`403`), `http.StatusNotFound` (`404`), and
`http.StatusInternalServerError` (`500`) cover the great majority of handlers; the full
set (matching the standard IANA status code registry) is also available, along with
`http.MethodGet`, `http.MethodPost`, `http.MethodPut`, `http.MethodPatch`,
`http.MethodDelete`, and the rest of the standard HTTP method names.

---

### i18n Package<a name="i18n"></a>

Unlike most Ego packages, `i18n` has no direct equivalent in Go's standard
library. It provides internationalization/localization support: looking up
a message by key in whichever language is active, and substituting named
values (with optional formatting) into the resulting text. It works hand in
hand with the [`@localization` directive](#at-localization), which defines
a program's own per-language message catalog, and falls back to Ego's own
built-in message catalog — the same one CLI and error messages are drawn
from — for any key a program's own `@localization` block doesn't define
(or when a program has no `@localization` block at all).

#### i18n.Language() string

Returns the current default language code (`"en"`, `"fr"`, `"es"`, etc).
This is resolved once per process, in order: the `EGO_LANG` environment
variable, then the more general `LANG` environment variable, then a
built-in default of `"en"` if neither is set (or neither names a real
language — see the note on locale strings below).

```go
lang := i18n.Language()
fmt.Println(lang)   // "en"
```

#### i18n.T(key string [, parameters map[string]any [, language string]]) (string, error)

Looks up `key` and returns its localized, substituted text.

- If the program has an active `@localization` block, `key` is looked up
  under the requested language within it, falling back to `"en"` within
  that same block if the requested language isn't present there.
- If there is no `@localization` block, or `key` isn't found within it,
  `key` is looked up in Ego's own built-in message catalog instead.
- If `key` isn't found anywhere, the literal `key` text is returned
  unchanged (so a missing catalog entry is obvious in the output, rather
  than silently producing an empty string).

```go
@localization {
    "en": { "hello.msg": "hello, {{Name}}" },
    "fr": { "hello.msg": "bonjour, {{Name}}" },
}

m, err := i18n.T("hello.msg", map[string]any{"Name": "Tom"}, "fr")
fmt.Println(m, err)   // bonjour, Tom <nil>
```

`parameters` is optional and supplies values for any `{{tag}}` placeholders
in the message text — see [Substitutions](#at-localization) under the
`@localization` directive for the full placeholder and formatting-operator
syntax. `language` is also optional; if omitted, `i18n.Language()`'s
current default is used. This third argument is the way to look up text in
a specific language regardless of the process default — useful, for
example, when a REST service wants to honor an individual caller's
language preference rather than the server's own default.

```go
// No @localization block in this program at all -- falls straight
// through to Ego's own built-in catalog.
greeting, err := i18n.T("ego.hello", map[string]any{"name": "Alice"})
fmt.Println(greeting, err)   // Hello, Alice! <nil>
```

#### i18n.Format(text string [, parameters map[string]any]) (string, error)

Applies `{{tag}}` placeholder substitution directly to `text`, without a
catalog lookup — this is the function `i18n.T()` itself calls once it has
resolved a key to its message text. Use it directly when you already have
the message text in hand (for example, a value read from configuration or
composed at runtime) rather than a catalog key.

```go
s, err := i18n.Format("Total: {{amount|%.2f}}", map[string]any{"amount": 9.5})
fmt.Println(s, err)   // Total: 9.50 <nil>
```

If `parameters` is omitted, `nil`, or an empty map, `text` is returned
completely unchanged (including any `{{tag}}` placeholders, left as literal
text) with no error — substitution is only attempted when at least one
parameter is supplied. When `parameters` is non-empty but doesn't contain a
value for every placeholder actually referenced in `text`, the result still
contains the best-effort substituted text (unresolved placeholders are left
as literal `{{...}}` text) _and_ a non-nil, catchable error identifying the
missing key:

```go
s, err := i18n.Format("hello {{missing}}", map[string]any{"other": "x"})
fmt.Println(s, err)   // hello {{missing}} substitution key not found: "missing"
```

`parameters` may be an Ego `map[string]any` (as in the examples above) or a
struct, in which case each field name becomes a placeholder name:

```go
type Greeting struct {
    Name string
}

s, _ := i18n.Format("hi {{Name}}", Greeting{Name: "Sam"})
fmt.Println(s)   // hi Sam
```

Parameter values keep their original Ego type (`float64`, `int`, `string`,
...) so that Go `fmt`-verb formatting operators like `%.2f` or `%03d` work
correctly against numeric values — see the format operator table under
[Substitutions](#at-localization).

#### A note on locale strings

`EGO_LANG`/`LANG` values are parsed as POSIX/BCP-47-style locale strings —
`"en_US.UTF-8"`, `"fr_FR@euro"`, or a bare `"en"` are all recognized, and
only the language subtag before the first `_`, `.`, or `@` is used. The
special locale names `"C"` and `"POSIX"` (common defaults in containers and
minimal shells, meaning "no locale configured" rather than naming a real
language) are treated the same as an unset value, falling back to `"en"`.

---

### io Package<a name="io"></a>

The io package supports input/output operations using native files in the file system
of the computer running _Ego_. Line-oriented text access (`ReadString`/`WriteString`)
and raw byte access (`Write`/`WriteAt`) are both supported, but there is currently no
raw byte _read_ method on `io.File` -- see the note at the end of "The `File`
structure" section below if you need to read binary data back out of a file.

Like the `exec` package, file access can be restricted: when a sandbox path is
configured (the `ego.runtime.sandbox.path` setting), paths passed to any function in
this package are confined to that sandbox directory, and relative paths containing
`..` segments are rejected.

#### io.Open(filename [, mode])

```go
func io.Open(filename string, mode ...string) (io.File, error)
```

The `Open()` function opens a file, and returns a `File` object that can be used to
perform further operations on the file, along with an error (`nil` on success).

```go
f, err := io.Open("mydata.txt", "create")
if err != nil {
    fmt.Println("could not open file:", err)
    return
}

defer f.Close()
```

This opens a file named "mydata.txt" for output, creating it if it does not already
exist. The `mode` argument is optional (it defaults to `"read"`) and can be one of
the following values:

&nbsp;

| Mode | Description |
| :----: | :---------- |
| `read` or `input` | The file must already exist, and is opened for reading only. |
| `write` or `output` | The file must already exist, and is opened for writing only, starting at the beginning of the file. |
| `create` | The file is created (any previous contents are discarded) and opened for writing. |
| `append` | The file must exist, and is opened for writing. All new data is written to the end of the file. |

&nbsp;

An invalid mode string (anything other than the values above) returns a non-nil
error rather than a valid `File`, and -- like every other error in this package --
that error is a normal, catchable value; no `try`/`catch` is required to observe it:

```go
f, err := io.Open("mydata.txt", "not-a-real-mode")
fmt.Println(f, err)   // <nil>  invalid file open mode: not-a-real-mode
```

#### The `File` structure

Once a `File` is created by `Open()`, its methods perform additional operations on
the underlying file until `Close()` is called, after which the object can no longer
be used for I/O (though it remains a valid value you can still inspect with
`String()`).

| Method | Description |
| :------- | :---------- |
| `Close() error` | Closes the file. After this, no other method (except `String()`) can be used on the object. |
| `ReadString() (string, error)` | Reads the next line of text from the file (without its trailing newline). Returns an `errors.ErrScanEOF` ("EOF") error once the end of the file is reached. |
| `WriteString(s string) (int, error)` | Writes a string to the file, followed by a newline, and returns the number of bytes written. |
| `Write(b []byte) (int, error)` | Writes a byte array's raw bytes to the file at the current position, and returns the number of bytes written. |
| `WriteAt(b []byte, offset int) (int, error)` | Writes a byte array's raw bytes to the file at the given byte offset, and returns the number of bytes written. |
| `String() string` | Returns a human-readable description of the file object's state, mostly useful for debugging (e.g. `<file; open; name "mydata.txt"; fileptr ...>`). |

A file also exposes read-only fields: `f.Name` (the absolute path of the file),
`f.Mode` (one of `"input"`, `"output"`, or `"append"`, reflecting the mode it was
opened with -- or `"closed"` after `Close()`), and `f.Valid` (`true` until the file
is closed).

**Reading text a line at a time:** because `ReadString()` reports end-of-file as an
error rather than an empty result, the natural loop is a `try`/`catch` (or an
explicit `.Is()` check) around repeated reads:

```go
f, err := io.Open("mydata.txt", "read")
if err != nil {
    fmt.Println("could not open file:", err)
    return
}

defer f.Close()

for {
    line, err := f.ReadString()
    if err != nil {
        break   // end of file (or a real read error -- err.Is(errors.New("scan.eof")) tells them apart)
    }

    fmt.Println(line)
}
```

**Writing raw bytes:**

```go
f, err := io.Open("data.bin", "create")
if err != nil {
    fmt.Println("could not open file:", err)
    return
}

n, err := f.Write([]byte{0x01, 0x02, 0x03})
fmt.Println(n, err)   // 3  <nil>

f.Close()
```

`Write()` and `WriteAt()` write the byte array's contents exactly as given -- unlike
`WriteString()`, they do not add a trailing newline, and they do not interpret or
re-encode the bytes in any way.

**No raw byte read method:** `io.File` has `Write`/`WriteAt` for raw bytes, but only
`ReadString` (line-oriented text) for reading -- there is no `Read`/`ReadAt`
counterpart. If you need to read binary data back out of a file, use `os.Open()`
instead, which returns a different file object with `Read([]byte)` and
`ReadAt([]byte, int64)` methods, or read the whole file at once with
`os.ReadFile(filename)`.

#### io.ReadDir(path)

```go
func io.ReadDir(path string) ([]io.Entry, error)
```

The `ReadDir()` function reads the list of files in a given directory path,
returning an array of structures describing each entry, and an error (`nil` on
success). An empty array (not an error) is returned if the directory is empty.

```go
entries, err := io.ReadDir("/tmp")
if err != nil {
    fmt.Println("could not read directory:", err)
    return
}

for _, e := range entries {
    fmt.Println(e.Name, e.Size, e.IsDirectory)
}
```

Each `io.Entry` structure has the following fields:

&nbsp;

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Name` | `string` | The name of the file (not the full path -- join it with the directory you passed to `ReadDir` if you need the full path). |
| `IsDirectory` | `bool` | `true` if the entry is itself a subdirectory, else `false`. |
| `Mode` | `string` | Unix-style mode string for the file's permissions (e.g. `"-rw-r--r--"`). |
| `Size` | `int` | The size of the file's contents, in bytes. |
| `Modified` | `time.Time` | An Ego `time.Time` value for the last modification time; use its `Format()` method to render it as text. |

&nbsp;

#### io.Expand(path [, filter])

```go
func io.Expand(path string, filter ...string) ([]string, error)
```

The `Expand()` function produces an array of the absolute path names of all files
found within `path`, along with an error (`nil` on success). If `path` names a single
file rather than a directory, the result is a one-element array containing that
file's path. The optional `filter` argument, if given, restricts the results to
names ending in that suffix (typically a file extension such as `".go"`).

```go
fns, err := io.Expand("/tmp")
if err != nil {
    fmt.Println("could not expand path:", err)
    return
}

for _, fn := range fns {
    fmt.Println(fn)
}
```

`Expand()` recurses into subdirectories, so the result can include paths several
levels below `path`.

#### io.DirList(path)

```go
func io.DirList(path string) string
```

The `DirList()` function produces a single string containing a human-formatted
directory listing, similar to the Unix `ls -l` command, with one line per entry
(mode, modification time, size, and name) already separated by newlines. It is
implemented in terms of `io.ReadDir()` -- if the directory cannot be read, the
returned string describes the error instead of listing any files, rather than
returning a separate error value.

```go
fmt.Print(io.DirList("/tmp"))
```

#### io.Prompt(text)

```go
func io.Prompt(text string) string
```

The `Prompt()` function writes `text` to the console as a prompt, then reads and
returns a single line of text typed by the user (with the trailing newline
removed). If `text` begins with the special prefix `"password~"`, the remainder of
`text` is used as the prompt, but the typed input is not echoed back to the
console -- useful for interactively collecting a password or other secret.

```go
name := io.Prompt("Enter your name: ")
secret := io.Prompt("password~Enter your password: ")
```

---

### json Package<a name="json"></a>

The `json` package converts _Ego_ data values to and from JSON text, and provides a
small query language for pulling a single value out of a JSON string without fully
decoding it. It is not a complete port of Go's `encoding/json` (there is no support
for struct tags, `json.Marshaler`/`Unmarshaler` interfaces, streaming encoders/decoders,
etc.), but the functions it does provide follow the same rules and produce the same
output as their Go counterparts.

#### json.Marshal(v [, ...])

```go
func json.Marshal(v any, more ...any) ([]byte, error)
```

The `Marshal()` function converts a value into a JSON byte array, along with an error
(`nil` on success). As in Go, map keys are always sorted alphabetically in the output,
regardless of insertion order -- so a struct's fields (which _Ego_ represents
internally as a map) are always emitted in alphabetical order by field name, not
declaration order.

```go
a := { name: "Tom", age: 44 }

b, err := json.Marshal(a)
if err != nil {
    fmt.Println("marshal failed:", err)
    return
}

fmt.Println(string(b))   // {"age":44,"name":"Tom"}
```

_Ego_ offers an extension to the Go version of this function: if you call `json.Marshal()`
with more than one argument, the result is a single JSON array containing the JSON
representation of every argument, in order.

```go
a := { name: "Tom", age: 44 }

s, err := json.Marshal(7334, a, true)
fmt.Println(string(s))   // [7334,{"age":44,"name":"Tom"},true]
```

A value that cannot be represented as JSON (for example, a `float64` holding `NaN` or
`Inf`, or a channel) produces a non-nil error rather than a partial result -- this is
true whether `Marshal()` is called with one argument or several.

#### json.MarshalIndent(v, prefix, indent)

```go
func json.MarshalIndent(v any, prefix string, indent string) ([]byte, error)
```

The `MarshalIndent()` function works like `Marshal()`, but the resulting JSON is
formatted for readability: each nested level is placed on its own line. `prefix` is
written at the start of every line after the first, and `indent` is written once for
each level of nesting beyond that -- both are ordinary strings, most commonly a
handful of spaces or a tab character.

```go
a := { name: "Tom", age: 44 }

b, err := json.MarshalIndent(a, "", "   ")
if err != nil {
    fmt.Println("marshal failed:", err)
    return
}

fmt.Println(string(b))
```

This prints:

```json
{
   "age": 44,
   "name": "Tom"
}
```

`MarshalIndent()` does not accept `json.Marshal()`'s multiple-argument extension --
it always takes exactly one value to encode.

#### json.Unmarshal(data, &value)

```go
func json.Unmarshal(data []byte, value *any) error
```

The `Unmarshal()` function parses the JSON text in `data` (a `[]byte`, though a plain
`string` is also accepted) and writes the result into `value`, which must be a pointer.
The existing value pointed to acts as a _model_: its concrete type determines how the
JSON is converted, and (for structs, arrays, and maps) its existing contents are
updated in place rather than replaced outright.

```go
a := { name: "Tom", age: 44 }
s, _ := json.Marshal(a)

r := { age: 0, name: "" }

err := json.Unmarshal(s, &r)
if err != nil {
    fmt.Println("unmarshal failed:", err)
    return
}

fmt.Println(r)   // struct{ age: 44, name: "Tom" }
```

A few behaviors worth knowing, all of which match Go's `encoding/json`:

- **Unknown JSON fields are silently ignored.** If `data` contains a field with no
  matching field in the destination struct, it is skipped rather than causing an error.
- **Existing fields not present in the JSON are left unchanged.** `Unmarshal()` only
  overwrites the fields it finds in `data`.
- **A `nil` destination map or slice is allocated automatically.** `var m map[string]int; json.Unmarshal(data, &m)`
  works the same way it does in Go -- there is no need to `make()` the map first. A
  destination that already has entries keeps any that aren't overwritten by the JSON.
- **Type mismatches are returned as a normal error, not raised as an exception.** For
  example, unmarshaling the JSON string `"notanumber"` into an `int` destination
  returns a non-nil error from `Unmarshal()` itself; no `try`/`catch` is required.
- **A generic destination (`interface{}`/`any`, or a map/array of it) decodes JSON
  numbers as `float64`** and JSON objects/arrays as `map[string]any`/`[]any`
  (converted to Ego maps/arrays), exactly as Go's `encoding/json` does for an `any`
  destination.

#### json.Parse(text, expression)

```go
func json.Parse(text string, expression string) (string, error)
```

The `Parse()` function extracts a single value out of a JSON string using a small
query language (provided by the underlying `jaxon` package), without requiring a
destination model the way `Unmarshal()` does. The result is **always returned as a
string**, regardless of the underlying JSON type -- a JSON number, boolean, or `null`
comes back as its text form (`"42"`, `"true"`, `"null"`), and an object or array comes
back as a formatted JSON string of just that fragment. Convert the result yourself
(with `strconv.Atoi()`, for example) if you need something other than a string.

```go
text := `{"user": {"name": "Jane", "age": 28}}`

name, err := json.Parse(text, "user.name")
fmt.Println(name, err)   // Jane <nil>

age, err := json.Parse(text, "user.age")
fmt.Println(age, err)   // 28 <nil>
```

**Query expression syntax:**

| Expression | Matches |
| :--------- | :------ |
| `.` (or an empty string) | The entire value. |
| `name` or `.name` | The field called `name` in an object. |
| `name.other` | Field `other` nested inside field `name`. |
| `2` or `[2]` | The element at (zero-based) array index 2. |
| `items[1].age` | Field `age` inside the object at index 1 of array `items`. |
| `name?fallback` | Field `name`, or the literal string `fallback` if that field is not present. |

A query segment can name an object field, or index into an array, and segments are
chained with `.` to walk into nested structures -- see the examples above.

The underlying `jaxon` package also supports range queries (`"0:2"`, `"2:"`, `":2"`)
and wildcards (`items.*.name`, or `items.*?.name` to skip elements that don't match
instead of failing) that select _multiple_ values at once. `json.Parse()` only ever
returns a single result, so an expression that matches more than one value returns an
error ("ambiguous query returns multiple values") rather than a list:

```go
_, err := json.Parse(`[1,15,66]`, "0:1")
fmt.Println(err)   // ambiguous query returns multiple values: 0:1
```

Errors are also returned (not raised) for malformed JSON input, a missing field, an
out-of-range array index, or applying array/object syntax to the wrong kind of value.

#### json.WriteFile(filename, v)

```go
func json.WriteFile(filename string, v any) error
```

The `WriteFile()` function marshals `v` to JSON (using the same rules as `Marshal()`)
and writes it to `filename`, creating or truncating the file as needed. It returns an
error (`nil` on success) if `v` cannot be marshaled or the file cannot be written.

```go
a := { name: "Tom", age: 44 }

if err := json.WriteFile("person.json", a); err != nil {
    fmt.Println("could not write file:", err)
}
```

#### json.ReadFile(filename)

```go
func json.ReadFile(filename string) (any, error)
```

The `ReadFile()` function reads `filename` and parses its contents as JSON, returning
the decoded value along with an error (`nil` on success). Unlike `Unmarshal()`, there
is no destination model to guide the conversion, so the result follows the same
generic decoding rules Go's `encoding/json` uses for an `any` destination: a JSON
object becomes an Ego map (numbers as `float64`), a JSON array becomes an Ego array,
and a top-level scalar is returned directly.

```go
v, err := json.ReadFile("person.json")
if err != nil {
    fmt.Println("could not read file:", err)
    return
}

fmt.Println(v["name"], v["age"])   // Tom 44
```

Use `reflect.Members()` (see the `reflect` package) if you need to check what fields
are present before accessing them.

---

### math Package<a name="math"></a>

The `math` package provides a comprehensive set of math operations on common _Ego_ numeric
data types (usually `int` and `float64` values). Most functions mirror the Go standard
`math` package directly.

#### math Constants

The `math` package defines the following numeric constants:

| Constant | Value | Description |
| :------- | :---- | :---------- |
| `E` | 2.718281828… | Base of natural logarithms |
| `Ln10` | 2.302585093… | Natural logarithm of 10 |
| `Ln2` | 0.6931471806… | Natural logarithm of 2 |
| `Log10E` | 0.4342944819… | Base-10 logarithm of E |
| `Log2E` | 1.442695041… | Base-2 logarithm of E |
| `MaxFloat32` | 3.402823466e+38 | Largest finite `float32` value |
| `MaxFloat64` | 1.797693135e+308 | Largest finite `float64` value |
| `MaxInt` | 9223372036854775807 | Largest `int` value (64-bit) |
| `MaxInt8` | 127 | Largest `int8` value |
| `MaxInt16` | 32767 | Largest `int16` value |
| `MaxInt32` | 2147483647 | Largest `int32` value |
| `MaxInt64` | 9223372036854775807 | Largest `int64` value |
| `MaxUint32` | 4294967295 | Largest `uint32` value |
| `MaxUint64` | 18446744073709551615 | Largest `uint64` value |
| `MinInt` | -9223372036854775808 | Smallest `int` value (64-bit) |
| `MinInt8` | -128 | Smallest `int8` value |
| `MinInt16` | -32768 | Smallest `int16` value |
| `MinInt32` | -2147483648 | Smallest `int32` value |
| `MinInt64` | -9223372036854775808 | Smallest `int64` value |
| `Phi` | 1.618033989… | The golden ratio (φ) |
| `Pi` | 3.141592654… | The ratio of a circle's circumference to its diameter |
| `SmallestNonzeroFloat32` | 1.401298464e-45 | Smallest positive nonzero `float32` |
| `SmallestNonzeroFloat64` | 4.940656458e-324 | Smallest positive nonzero `float64` |
| `Sqrt2` | 1.414213562… | Square root of 2 |
| `SqrtE` | 1.648721271… | Square root of E |
| `SqrtPhi` | 1.272019650… | Square root of Phi |
| `SqrtPi` | 1.772453851… | Square root of Pi |

#### math.Abs(n)

For a given numeric value, return the absolute value of the number.

```go
posInt := math.Abs(signedInt)
```

In this example, `posInt` will always be a positive or zero value.

#### math.Acos(f)

Returns the arc cosine of `f`, in radians. The argument must be in the range [-1, 1];
values outside this range return `NaN`.

```go
a := math.Acos(0.5)
```

The value of `a` is approximately `1.0472` (π/3 radians, or 60°).

#### math.Acosh(f)

Returns the inverse hyperbolic cosine of `f`. The argument must be ≥ 1; smaller
values return `NaN`.

#### math.Asin(f)

Returns the arc sine of `f`, in radians. The argument must be in the range [-1, 1].

```go
a := math.Asin(1.0)
```

The value of `a` is approximately `1.5708` (π/2 radians, or 90°).

#### math.Asinh(f)

Returns the inverse hyperbolic sine of `f`.

#### math.Atan(f)

Returns the arc tangent of `f`, in radians, in the range [-π/2, π/2].

```go
a := math.Atan(1.0)
```

The value of `a` is approximately `0.7854` (π/4 radians, or 45°).

#### math.Atanh(f)

Returns the inverse hyperbolic tangent of `f`. The argument must be in the range
(-1, 1); values outside this range return `NaN` or ±Inf.

#### math.Cbrt(f)

Returns the cube root of `f`.

```go
a := math.Cbrt(27.0)
```

The value of `a` is `3.0`.

#### math.Ceil(f)

Returns the smallest integer value greater than or equal to `f` (rounds toward
positive infinity).

```go
a := math.Ceil(1.2)
b := math.Ceil(-1.2)
```

The value of `a` is `2.0` and the value of `b` is `-1.0`.

#### math.Cos(f)

Returns the cosine of the angle `f` expressed in radians.

```go
a := math.Cos(math.Pi)
```

The value of `a` is `-1.0`.

#### math.Cosh(f)

Returns the hyperbolic cosine of `f`.

#### math.Erf(f)

Returns the error function of `f`. This is used in statistics and probability
theory to describe diffusion.

#### math.Erfc(f)

Returns the complementary error function of `f`, which is `1 - Erf(f)`.

#### math.Erfcinv(f)

Returns the inverse of `Erfc(f)`.

#### math.Erfinv(f)

Returns the inverse error function of `f`. The argument must be in the range
(-1, 1).

#### math.Exp2(f)

Returns `2` raised to the power `f` (2^f).

```go
a := math.Exp2(10)
```

The value of `a` is `1024.0`.

#### math.Expm1(f)

Returns `e^f - 1`. This is more accurate than computing `math.Exp(f) - 1`
directly when `f` is close to zero.

#### math.Factor(i)

For a given positive integer `i`, return an array of all the unique factors for that
value. The array is always an array of integers. For a prime number, this will always
return an array with two elements, one and the prime number. For all other numbers,
it returns an array that contains one, the number, and all factors of the number.

 ```go
a := math.Factor(11)
b := math.Factor(12)
```

For the first example, `a` contains [1, 11] because 11 is a prime number. The value of
`b` contains [1, 2, 3, 4, 6, 12].

#### math.Floor(f)

Returns the largest integer value less than or equal to `f` (rounds toward
negative infinity).

```go
a := math.Floor(1.9)
b := math.Floor(-1.2)
```

The value of `a` is `1.0` and the value of `b` is `-2.0`.

#### math.Gamma(f)

Returns the Gamma function of `f`. The Gamma function is a generalization of
the factorial — for positive integers, `Gamma(n) == (n-1)!`.

```go
a := math.Gamma(5.0)
```

The value of `a` is `24.0` (which is 4!).

#### math.Inf(sign)

Returns positive infinity if `sign` ≥ 0, and negative infinity if `sign` < 0.

```go
a := math.Inf(1)
b := math.Inf(-1)
```

The value of `a` is `+Inf` and `b` is `-Inf`.

#### math.IsInf(f, sign)

Reports whether `f` is an infinity. If `sign` > 0, reports whether `f` is
positive infinity; if `sign` < 0, reports whether `f` is negative infinity;
if `sign` == 0, reports whether `f` is either infinity.

```go
a := math.IsInf(math.Inf(1), 1)
```

The value of `a` is `true`.

#### math.IsNan(f)

Reports whether `f` is a `NaN` (not-a-number) value.

```go
a := math.IsNan(math.NaN())
```

The value of `a` is `true`.

#### math.Log(f)

For a given floating point value `f`, return the natural logarithm of the value.

```go
f := math.Log(2.1)
```

The value of `f` is 0.7419373447293773.

#### math.Max(...)

For an arbitrary list of numeric values, return the largest value in the list. The list
can be sent as individual items, or as an array of items.

```go
a := math.Max(n, 100)

b := [1, 2, 6, 3, 0]
c := math.Max(b...)
```

The value of `a` is the larger of the value of `n` and the value 100. This is comparable
to _use the value of `n` but it must be at least 100_. The value of `c` will be 6. The
ellipsis "..." notation indicate that the array b is to be treated as individual parameters
to the function, and the largest value in the array `b` is 6.

#### math.Min(...)

For an arbitrary list of numeric values, return the smallest value in the list. The list
can be sent as individual items, or as an array of items.

```go
a := math.Min(n, 10)

b := [1, 2, 6, 3, 0]
c := math.Min(b...)
```

The value of `a` is the smaller of the value of `n` and the value 10. This is comparable
to _use the value of `n` but it must be no larger than 10_. The value of `c` will
be 0. The ellipsis "..." notation indicates that the array b is to be treated as individual
parameters to the function, and the smallest value in the array `b` is 0.

#### math.Mod(dividend, divisor)

Returns the floating-point remainder of `dividend / divisor`. The result has
the same sign as `dividend`.

```go
a := math.Mod(10.5, 3.0)
```

The value of `a` is `1.5`.

#### math.NaN()

Returns an IEEE 754 "not-a-number" value. Use `math.IsNan()` to test whether a
value is `NaN`; direct equality comparisons with `NaN` always return `false`.

```go
a := math.NaN()
```

#### math.Normalize(a, b...)

Returns all arguments promoted to the most precise numeric type among them. If
any argument is a `float64` then all arguments are returned as `float64`; if
all arguments are `int`, they are returned as `int`.

```go
x, y := math.Normalize(3, 1.5)
```

Here `x` is `3.0` (promoted from `int` to `float64`) and `y` is `1.5`. This is
useful before arithmetic where mixed integer and floating-point arguments would
otherwise produce unexpected truncation.

#### math.Primes(i)

The `Primes` function accepts a positive integer value and returns an array of all the
prime numbers less than that value. Note that this can take a very long time to compute
for larger values.

```go
a := math.Primes(10)
```

The array `a` will contain the integers [3, 5, 7]. The values '1' and '2' are not considered
to be prime numbers.

#### math.Random(max)

Returns a non-negative pseudo-random integer in the range [0, max). Each call
produces a different value.

```go
a := math.Random(100)
```

The value of `a` is a random integer from 0 to 99 inclusive.

#### math.Remainder(dividend, divisor)

Returns the IEEE 754 floating-point remainder of `dividend / divisor`. Unlike
`Mod`, the result is the nearest value to zero rather than truncating toward
zero, and the result magnitude is always less than `|divisor| / 2`.

```go
a := math.Remainder(10.0, 3.0)
```

The value of `a` is `1.0`.

#### math.Round(f)

Returns the nearest integer to `f`, rounding half-values away from zero.

```go
a := math.Round(1.5)
b := math.Round(2.5)
```

The value of `a` is `2.0` and the value of `b` is `3.0`.

#### math.RoundToEven(f)

Returns the nearest integer to `f`, rounding ties to the nearest even integer
(banker's rounding). This minimizes cumulative rounding error in repeated
operations.

```go
a := math.RoundToEven(1.5)
b := math.RoundToEven(2.5)
```

Both `a` and `b` are `2.0` because 2 is the nearest even integer in each case.

#### math.Sin(f)

Returns the sine of the angle `f` expressed in radians.

```go
a := math.Sin(math.Pi / 2)
```

The value of `a` is `1.0`.

#### math.Sinh(f)

Returns the hyperbolic sine of `f`.

#### math.Sqrt(f)

Calculate the square root of the numeric value given.

```go
a := math.Sqrt(2)
```

The value of `a` will be approximately 1.4142135623730951.

#### math.Sum(...)

The `Sum` function returns the arithmetic sum of all the numeric values. These can be
passed as individual values or as an array.

```go
a := math.Sum(n, 10)

b := [5, 15, 25, 35]
c := math.Sum(b...)
```

The value of `a` is the sum of `n` and 10, and is identical to the expression `a := n + 10`. The
value of `c` is 80, which is the sum of all the values in the array. Note that the ellipsis "..."
notation indicates that the array should be converted to a list of parameters.

#### math.Tan(f)

Returns the tangent of the angle `f` expressed in radians.

```go
a := math.Tan(math.Pi / 4)
```

The value of `a` is approximately `1.0` (45°).

#### math.Tanh(f)

Returns the hyperbolic tangent of `f`.

#### math.Trunc(f)

Returns the integer portion of `f`, discarding the fractional part (rounds
toward zero).

```go
a := math.Trunc(1.9)
b := math.Trunc(-1.9)
```

The value of `a` is `1.0` and `b` is `-1.0`.

---

### os Package<a name="os"></a>

The `os` package provides functions that access operating system features -- command-line
arguments, environment variables, and the file system -- for whatever operating system
(macOS, Windows, Linux, etc.) you are running on. The results and behavior of some
functions can be specific to that operating system; the examples shown here are for macOS
(the "darwin" Go build).

Several functions in this package (anything that takes a file or directory path, plus
`os.Args()` and `os.Executable()`) are restricted the same way `exec` and `io` are: when a
sandbox path is configured (the `ego.runtime.sandbox.path` setting), paths are confined to
that sandbox directory.

**Octal file modes:** functions that take a numeric file permission mode (`os.WriteFile()`,
`os.Chmod()`) expect the same bit layout as Go's `os.FileMode` -- most commonly written as
an octal literal such as `0o644` or `0644`. Both forms are equivalent; see
[radix literals](#radix-literals) above.

#### os.Args()

```go
func os.Args() []string
```

The `Args()` function returns an array of the command-line arguments an _Ego_ program was
run with.

```go
func main() {
    fmt.Println(os.Args())
}
```

If this is placed in a file -- for example, "args.ego" -- then it can be run with a command
line similar to:

```bash
tom$ ego run args.ego stuff I want
```

The "tom$" is the shell prompt; the remainder of the command is the command line entered.
This prints:

```text
["stuff", "I", "want"]
```

Note that the array does **not** include the program name itself (unlike Go's `os.Args`,
whose element 0 is always the executable path) -- it contains only the tokens that came
after the source file name on the command line.

#### os.Exit(code)

```go
func os.Exit(code int)
```

The `Exit()` function immediately stops execution of the current _Ego_ program and returns
control to the shell, with `code` as the process's exit status (read via `$?` in most
Linux/Unix shells). The `code` argument is optional; if omitted, the exit status is 0.

```go
import "os"

func main() {
    if true {
        os.Exit(55)
    }
}
```

Running this and then checking the shell's exit status shows `55`.

If `main()` returns an `int` value instead of (or without ever reaching) an explicit
`os.Exit()` call, that returned value becomes the exit status, exactly as if `os.Exit()`
had been called with it:

```go
func main() int {
    return 42
}
```

Running this exits with status `42`. If `main()` declares no return value and completes
without calling `os.Exit()`, the exit status is 0.

#### Environment variables

```go
func os.Getenv(name string) string
func os.LookupEnv(name string) (string, bool)
func os.Setenv(name string, value string) error
func os.Environ() []string
func os.Clearenv()
func os.ExpandEnv(s string) string
func os.Expand(s string, mapping func(string) string) string
```

`Getenv()` retrieves the named environment variable's value, or an empty string if it is
not set (there is no way to distinguish "not set" from "set to an empty string" with
`Getenv()` alone -- use `LookupEnv()` for that).

```go
shell := os.Getenv("SHELL")
fmt.Println("You are running the", shell, "shell program")
```

`LookupEnv()` returns the value and a boolean that is `true` only if the variable is
actually set:

```go
v, found := os.LookupEnv("SHELL")
if !found {
    fmt.Println("SHELL is not set")
}
```

`Setenv()` sets an environment variable for the current process (and anything it
subsequently spawns via `exec`), returning an error if the name or value is invalid.
`Environ()` returns every environment variable as `"name=value"` strings. `Clearenv()`
deletes all environment variables for the current process.

`ExpandEnv()` replaces `$name` or `${name}` references in `s` with the corresponding
environment variable's value (an unset variable expands to an empty string):

```go
fmt.Println(os.ExpandEnv("home is $HOME"))   // home is /Users/tom
```

`Expand()` works like `ExpandEnv()`, but instead of consulting the process environment, it
calls `mapping` with each `${name}`/`$name` reference found and substitutes whatever string
that function returns:

```go
s := os.Expand("hello ${name}!", func(key string) string {
    if key == "name" {
        return "world"
    }
    return ""
})
fmt.Println(s)   // hello world!
```

#### os.Open/os.Create/os.CreateTemp -- the `os.File` type

```go
func os.Open(name string) (os.File, error)
func os.Create(name string) (os.File, error)
func os.CreateTemp(dir string, pattern string) (os.File, error)
```

These functions open a native file and return an `os.File` handle, along with an error
(`nil` on success). `Open()` opens an existing file for reading only. `Create()` creates a
new file for reading and writing, truncating it first if it already exists. `CreateTemp()`
creates a new temporary file in `dir` (or the system default temporary directory if `dir`
is `""`) whose name is derived from `pattern` -- a `*` in `pattern` is replaced with a
random string, matching Go's `os.CreateTemp()`.

```go
f, err := os.Create("mydata.txt")
if err != nil {
    fmt.Println("could not create file:", err)
    return
}

defer f.Close()

f.WriteString("hello")
```

`os.File` supports these methods, each returning the usual `(value, error)` pair (or just
`error` for `Close()` and `Chdir()`):

| Method | Description |
| :----- | :---------- |
| `Read(buf []byte) (int, error)` | Reads up to `len(buf)` bytes starting at the file's current position, returning the number of bytes read. |
| `ReadAt(buf []byte, offset int64) (int, error)` | Like `Read()`, but starting at the given byte offset instead of the current position. |
| `Write(buf []byte) (int, error)` | Writes `buf`'s bytes at the current position. |
| `WriteAt(buf []byte, offset int64) (int, error)` | Writes `buf`'s bytes at the given byte offset. |
| `WriteString(s string) (int, error)` | Writes the bytes of `s` at the current position (no trailing newline is added). |
| `Close() error` | Closes the file. |
| `Name() string` | Returns the name the file was opened/created with. |
| `Chdir() error` | Changes the current working directory to the file, which must have been opened as a directory. |
| `Chown(uid int, gid int) error` | Changes the file's owning user and group IDs. |

```go
f, _ := os.Create("data.bin")

f.Write([]byte("abc"))
f.WriteString("def")
f.WriteAt([]byte("XY"), 1)

fmt.Println(f.Name())
f.Close()
```

After this runs, `data.bin` contains `aXYdef`: `Write("abc")` writes at position 0,
`WriteString("def")` continues from position 3, and `WriteAt` then overwrites bytes 1-2
in place.

#### os.ReadFile(filename) / os.WriteFile(filename, data, mode)

```go
func os.ReadFile(filename string) ([]byte, error)
func os.WriteFile(filename string, data []byte, mode int) error
```

`ReadFile()` reads an entire file's contents into a byte array in one call, without
needing to `Open()`/`Close()` it explicitly. As an _Ego_-specific convenience, a filename
of exactly `"."` reads a single line of text from the console (stdin) instead of the file
system.

```go
b, err := os.ReadFile("mydata.txt")
if err != nil {
    fmt.Println("could not read file:", err)
    return
}

lines := strings.Split(string(b), "\n")
```

`WriteFile()` writes `data` to `filename` in one call, creating the file if it doesn't
exist and truncating it if it does. As an _Ego_ extension, `data` may be a plain `string`
instead of a `[]byte`. `mode` sets the file's permissions if it is newly created (as with
any file mode, this is usually written as an octal value, either `0o644` or `0644`):

```go
err := os.WriteFile("mydata.txt", "hello, world", 0o644)
if err != nil {
    fmt.Println("could not write file:", err)
}
```

#### os.Remove(filename) / os.RemoveAll(path)

```go
func os.Remove(filename string) error
func os.RemoveAll(path string) error
```

The `Remove()` function deletes a single file (or empty directory) from the file system,
returning an error if it does not exist or cannot be removed.

```go
if err := os.Remove("NewData.txt"); err != nil {
    fmt.Println("could not remove file:", err)
}
```

`RemoveAll()` removes `path` and, if it is a directory, everything it contains, recursively.
Matching Go's `os.RemoveAll()`, it returns a `nil` error if `path` does not exist (there is
nothing to remove), so it is convenient for tearing down a scratch directory tree in one call:

```go
if err := os.RemoveAll(scratchDir); err != nil {
    fmt.Println("could not remove directory tree:", err)
}
```

Both `Remove()` and `RemoveAll()` are disallowed entirely when running in sandboxed mode --
they are not merely confined to the sandbox root the way the read/write functions are.

#### os.Mkdir(path, mode) / os.MkdirAll(path, mode)

```go
func os.Mkdir(path string, mode int) error
func os.MkdirAll(path string, mode int) error
```

`Mkdir()` creates a single directory named `path`. It returns an error if `path` already
exists, or if its parent directory does not exist. `MkdirAll()` creates `path` along with
any parent directories that are missing, and -- matching Go's `os.MkdirAll()` -- returns a
`nil` error if the directory already exists. As with any file mode, `mode` is usually written
as an octal value (`0o755` or `0755`):

```go
// Create a two-level scratch area, then a file inside it.
root := filepath.Join(os.TempDir(), "myapp-"+uuid.New().String(), "work")
if err := os.MkdirAll(root, 0o755); err != nil {
    fmt.Println("could not create directory:", err)
    return
}

_ = os.WriteFile(filepath.Join(root, "data.txt"), "hello", 0o644)
```

#### os.TempDir()

```go
func os.TempDir() string
```

`TempDir()` returns the default directory to use for temporary files, exactly as Go's
`os.TempDir()` does (honoring `$TMPDIR` and the platform default, e.g. `/tmp`). It does not
create the directory or verify that it exists, and never returns an error -- it just reports
the path. Combine it with `MkdirAll()` (as above) to build a private scratch area.

#### os.Chmod(file, mode) / os.Chown(path, uid, gid)

```go
func os.Chmod(file string, mode int) error
func os.Chown(path string, uid int, gid int) error
```

`Chmod()` changes a file's permission bits (again, typically written as an octal `mode`
value, `0o600` or `0600`). `Chown()` changes a file's owning user and group IDs -- on
most systems this requires elevated privileges unless you are already the file's owner.

```go
if err := os.Chmod("mydata.txt", 0o600); err != nil {
    fmt.Println("could not change permissions:", err)
}
```

#### os.Stat(name) -- the `os.FileInfo` type

```go
func os.Stat(name string) (os.FileInfo, error)
```

`Stat()` returns information about the named file or directory without opening it,
along with an error (`nil` on success; for example if `name` does not exist).

Unlike Go's `os.FileInfo`, which is an interface accessed through method calls
(`info.Name()`, `info.Size()`, ...), _Ego_'s `os.FileInfo` is a plain struct with
read-only fields:

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Name` | `string` | The base name of the file (not the full path). |
| `Size` | `int64` | The file's size in bytes. |
| `Mode` | `int` | The raw permission and file-type bits, in the same representation `os.Chmod()`'s `mode` parameter uses -- so `os.Chmod(path, info.Mode)` round-trips a file's existing permissions. For a directory or other non-regular file, this includes type bits above the low 9 permission bits (matching Go's `os.FileMode` layout), so mask with `0o777` first if you only want the permission bits. |
| `ModTime` | `time.Time` | The file's last-modified time. |
| `IsDir` | `bool` | `true` if `name` is a directory. |

```go
info, err := os.Stat("mydata.txt")
if err != nil {
    fmt.Println("could not stat file:", err)
    return
}

fmt.Println(info.Name, info.Size, info.IsDir)
fmt.Println("permissions:", info.Mode & 0o777)
```

#### os.Hostname() / os.Executable()

```go
func os.Hostname() (string, error)
func os.Executable() (string, error)
```

`Hostname()` returns the name of the host the _Ego_ process is running on. `Executable()`
returns the absolute path of the currently running `ego` executable itself. Both return a
non-nil error if the underlying operating system call fails.

---

### profile Package<a name="profile"></a>

The `profile` package manages persistent configuration settings for the current user. There
is no equivalent package in Go's standard library -- this is entirely an _Ego_-specific
mechanism, the same one behind the `ego config` command line tool and the settings described
throughout this project's documentation (`ego.compiler.types`, `ego.runtime.path`, etc.).

Profile settings are stored in `~/.ego/` as JSON files -- `default.profile` for the default
profile, and `<name>.profile` for any other named profile selected with `ego --profile <name>`
or `ego -p <name>`. A setting changed with `profile.Set()`/`profile.Delete()` is written back
to that file the next time _Ego_ exits normally.

#### Three kinds of setting name

Every setting is identified by a string key, and how `profile.Get()`/`Set()`/`Delete()`
treat that key depends entirely on its name:

1. **Restricted keys.** A small, fixed set of especially sensitive keys (the server's token
   signing key, saved logon tokens, the console history file path, and similar) cannot be
   read, written, or deleted through this package **at all**, under any circumstances -- not
   even to set them to a new value for the current process only. `Get()`, `Set()`, and
   `Delete()` all return an error immediately for one of these. This is a security boundary,
   not a limitation you can work around; it exists so that _Ego_ code (which might come from
   an untrusted source, e.g. a script uploaded to a server) can never exfiltrate or overwrite
   credentials via the profile mechanism.

2. **`ego.*` keys.** Everything under this prefix is reserved for the compiler, runtime, and
   server's own settings (`ego.compiler.types`, `ego.runtime.path`, `ego.server.rest.port`,
   etc.). `profile.Set()` on one of these has two additional rules beyond the restricted-key
   check above:
   - The key must **already exist** -- you cannot invent a brand-new `ego.*` setting from
     Ego code.
   - Changes to anything under `ego.runtime.*`, `ego.server.*`, or `ego.compiler.*` are
     **rejected outright** when running a normal program (there is a narrow exception used
     internally by `ego test` so test files can exercise these settings, but ordinary `ego
     run` programs cannot use it).

   A `Set()` on an `ego.*` key that _is_ allowed only ever changes the **in-memory** copy for
   the lifetime of the current process -- it is never written back to the profile file on
   disk, regardless of how the process exits. This lets a script temporarily adjust its own
   behavior (for example, `ego.compiler.type.shadowing`) without permanently changing the
   user's saved configuration.

3. **Everything else -- your own application settings.** Any key that does not start with
   `ego.` is entirely yours: you can create, read, update, and delete these freely, and every
   change **is** persisted to the profile file on disk, exactly like settings changed with
   `ego config`. Use your own prefix (e.g. `myapp.`) to keep your settings distinct from
   other tools' and from Ego's own reserved names.

#### profile.Get(key)

```go
func profile.Get(key string) (string, error)
```

Retrieves the current value of `key` as a string, along with an error (`nil` on success).
Requesting a key that simply doesn't exist is not an error -- it returns an empty string. An
error is only returned for a restricted key (see above).

```go
path, err := profile.Get("ego.runtime.path")
if err != nil {
    fmt.Println("could not read setting:", err)
    return
}

fmt.Println("Ego path is", path)
```

#### profile.Set(key, value)

```go
func profile.Set(key string, value string) error
```

Creates or updates the setting named `key` to `value`, returning an error (`nil` on
success). Setting `value` to `""` (the empty string) is equivalent to calling
`profile.Delete(key)`. See "Three kinds of setting name" above for what can fail and why.

```go
if err := profile.Set("myapp.greeting", "hello there"); err != nil {
    fmt.Println("could not save setting:", err)
}
```

#### profile.Delete(key)

```go
func profile.Delete(key string) error
```

Deletes the setting named `key`, returning an error (`nil` on success). Deleting a key that
doesn't exist is not an error. Subject to the same restricted-key and `ego.*` rules as
`Set()`.

```go
if err := profile.Delete("myapp.greeting"); err != nil {
    fmt.Println("could not delete setting:", err)
}
```

#### profile.Keys()

```go
func profile.Keys() []string
```

Returns the names of every currently-set profile value, sorted alphabetically, excluding
any restricted keys.

```go
for _, key := range profile.Keys() {
    fmt.Println(key)
}
```

#### profile.Config()

```go
func profile.Config() map[string]string
```

Returns every currently-set profile value as a single map of key to value, excluding any
restricted keys -- a convenient way to inspect or dump the whole active configuration at
once rather than calling `Get()` once per key from the list `Keys()` returns.

```go
for key, value := range profile.Config() {
    fmt.Println(key, "=", value)
}
```

---

### reflect Package<a name="reflect"></a>

The `reflect` package lets an _Ego_ program discover information about its own values and
types at runtime. Go's standard `reflect` package is far more extensive than this -- it
exposes deep structural access to arbitrary Go values, including unexported fields, memory
layout, and the ability to construct values dynamically. _Ego_ deliberately does not attempt
to match that: Ego already wraps its data in protective layers (structs, arrays, and maps are
managed types with their own access rules, not raw memory Go's `reflect` could walk), and this
package only exposes what an Ego program can already do safely -- inspect a value's type and
members, get a zero-value instance of a type, or make an independent deep copy of a value.

#### Types as first-class values

Before covering the package's functions, it's worth understanding a piece of the language
that makes them useful: in _Ego_, a type itself is a value that can be stored in a variable,
passed around, compared, and reassigned, just like any other value (in dynamic mode -- the
default). The built-in type names (`int`, `string`, `bool`, etc.), user-defined type names,
and the results of `reflect.Type()` (below) are all values of this kind.

```go
t := int      // t now holds the type value "int"

var x t       // declares x with the type held in t -- x is an int, initialized to 0

fmt.Println(x, typeof(x))   // 0 int
```

Because `t` is an ordinary (mutable, in dynamic mode) variable, it can be reassigned to a
different type later, and a subsequent `var` declaration using it picks up the new type:

```go
t = string
var y t
fmt.Println(y, typeof(y))   // "" string
```

This is what makes `reflect.InstanceOf()` and `reflect.Type()` (below) useful as ordinary
values rather than special-cased syntax -- a type returned by one can be stored, compared
with `==`/`!=`, or handed to a `var` declaration exactly like `int` or `string` written
directly in source.

#### reflect.Type(value) / typeof(value)

```go
func reflect.Type(value any) type
```

Returns the type of `value` as a first-class type value (see above). The result can be
compared against a built-in type name, a user-defined type name, or another value's type,
using `==`/`!=`:

```go
a := 425.3
b := reflect.Type(a)

if b == float64 {
    fmt.Println("The value is a float64")
}
```

This works for more complex types too -- assign the comparison to a variable first if the
type expression itself contains brackets (`map[...]`, `[]...`), since writing one directly
inside an `if` condition can be misparsed as the start of a map literal:

```go
m := map[string]int{"Fred": 35}
same := (reflect.Type(m) == map[string]int)

fmt.Println(same)   // true
```

A pointer value reflects as a pointer to the pointee's own type (not just a generic pointer),
and passing a type name itself (rather than a value of that type) returns its meta-type:

```go
x := 42
p := &x
fmt.Println(reflect.Type(p))   // *int

type Foo struct { a int }
fmt.Println(reflect.Type(Foo)) // type Foo struct{a int}   (Foo the type itself)

f := Foo{a: 1}
fmt.Println(reflect.Type(f))   // Foo struct{a int}         (the type of a Foo value)
```

`reflect.Type()` is also available as the built-in function `typeof()`, with one difference:
`typeof()` is a language extension and only works when extensions are enabled (the
`ego.compiler.extensions` setting, on by default); `reflect.Type()` works regardless of that
setting, since it's a normal package function call rather than special syntax.

```go
fmt.Println(typeof(42))   // int
```

#### reflect.Reflect(value)

```go
func reflect.Reflect(value any) reflect.Reflection
```

Returns a `reflect.Reflection` structure describing `value` in more detail than
`reflect.Type()` alone. Not every field is populated for every kind of value -- for example,
`Members` is only meaningful for a struct, map, or package, and `Declaration` only for a
function.

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Type` | `string` | The _Ego_ type name of the value (`"struct"`, `"int"`, `"func"`, `"builtin"`, `"package"`, `"error"`, etc.) |
| `BaseType` | `string` | The underlying type -- a struct's field layout, an array's element type, a function's label and name, etc. |
| `Name` | `string` | The name of the value, for named functions and type definitions; empty otherwise. |
| `Package` | `string` | The name of the package a struct's type belongs to, if any; empty for anonymous/local types. |
| `IsType` | `bool` | `true` if `value` was itself a type (e.g. you passed `reflect.Type(x)`'s result, or a type name like `int`), rather than an ordinary value. |
| `Native` | `bool` | `true` if this wraps a native Go type or function rather than one defined in Ego source. |
| `Imports` | `bool` | For a package: `true` if it has Ego-source library members (from `lib/packages/`). |
| `Builtins` | `bool` | For a package: `true` if it has native Go-implemented members. |
| `Members` | `[]string` | Field names (struct), keys (map), or exported member names (package). |
| `Declaration` | `reflect.Function` | The function's name, parameters, return types, and argument-count range; only set for functions. |
| `Size` | `int` | Byte size for scalars, or element/member count for arrays, maps, and packages. |
| `Error` | `error` | The error's i18n key, only set when reflecting an error value. |
| `Text` | `string` | The error's human-readable message, only set when reflecting an error value. |
| `Context` | `string` (or struct) | Additional error context, only set when reflecting an error value that has one. |

```go
e := { name: "Dave", age: 33 }
r := reflect.Reflect(e)

fmt.Println(r.Type)     // struct
fmt.Println(r.BaseType) // struct{name string, age int}
fmt.Println(r.Members)  // ["name", "age"]
```

A `reflect.Reflection` value also has a `String()` method that renders it as a compact,
human-readable summary (used automatically by `fmt.Println`), and it's what you see if you
print the value directly rather than accessing individual fields.

#### reflect.Members(value)

```go
func reflect.Members(value any) ([]string, error)
```

Returns the names of the fields (for a struct), keys (for a map, sorted alphabetically), or
exported members (for a package, sorted alphabetically) in `value`, along with an error
(`nil` on success). Any other kind of value (a scalar, array, etc.) produces a non-nil error.

```go
e := { name: "Dave", age: 33 }

m, err := reflect.Members(e)
if err != nil {
    fmt.Println("could not get members:", err)
    return
}

fmt.Println(m)   // ["name", "age"]
```

For a struct, the names are returned in field-declaration order, not alphabetically --
`{ name: "Dave", age: 33 }` yields `["name", "age"]`, matching the order the fields were
written in, not `["age", "name"]`. The resulting names can be used to access struct fields
by index notation:

```go
e[m[0]] = "Changed"   // sets e.name, since m[0] == "name"
```

#### reflect.InstanceOf(value)

```go
func reflect.InstanceOf(value any) any
```

Returns a "zero value" based on `value`'s type -- `0` for numeric types, `""` for `string`,
`false` for `bool`, an empty (nil-state) map or zero-length array for a map or array, and a
fresh zero-valued instance for a struct. `value` can be:

- **An example value of any kind** -- the result is the zero value of _that value's own
  type_, not a copy of the value itself:

  ```go
  fmt.Println(reflect.InstanceOf(42))        // 0
  fmt.Println(reflect.InstanceOf("hello"))   // ""
  fmt.Println(reflect.InstanceOf(int32(7)))  // 0   (as an int32)
  ```

- **A type value** (a built-in type name, a user-defined type, or the result of
  `reflect.Type()`) -- useful when you don't have a convenient example value on hand:

  ```go
  fmt.Println(reflect.InstanceOf(int))     // 0
  fmt.Println(reflect.InstanceOf(string))  // ""

  t := int
  fmt.Println(reflect.InstanceOf(t))       // 0
  ```

- **A struct, array, or map value** -- returns an empty/zero instance of the _same type or
  shape_, not a copy of the contents:

  ```go
  type Config struct { host string; port int }
  c := Config{ host: "localhost", port: 9090 }

  zero := reflect.InstanceOf(c)
  fmt.Println(zero)   // Config{ host: "", port: 0 }
  fmt.Println(c)      // Config{ host: "localhost", port: 9090 } -- unchanged
  ```

- **A channel** -- returned unchanged, since a channel has no zero-value concept of its own.

#### reflect.DeepCopy(value [, depth])

```go
func reflect.DeepCopy(value any, depth ...int) (any, error)
```

Makes an independent copy of `value`, along with an error (`nil` on success; the only
failure mode is a non-numeric `depth`). Scalars are copied trivially, since they're already
independent value types. For a struct, array, map, or package, every nested member is
copied recursively as well, so the result shares no mutable state with the original --
modifying a nested field, element, or entry in the copy never affects the source, and vice
versa.

```go
a := { name: "Dave", tags: []string{"a", "b"} }
b := reflect.DeepCopy(a)

b.tags[0] = "Z"

fmt.Println(a.tags)   // ["a", "b"]  -- unaffected
fmt.Println(b.tags)   // ["Z", "b"]
```

The optional `depth` argument caps how many levels of nesting are copied before `DeepCopy`
gives up and stops recursing (default 100); you are unlikely to need to change this unless
you have unusually deep data structures.

---

### rest Package<a name="rest"></a>

The `rest` package provides a generalized HTTP/HTTPS client for communicating with a
server: authenticating with it (username/password, a bearer token, or the token from
a prior `ego logon`), and performing GET, POST, and DELETE requests against URL
endpoints. There is no equivalent package in Go's standard library -- this is an
_Ego_-specific convenience wrapper  (like `reflect` and `profile`).

A JSON response body is automatically decoded into an Ego-native value (a map, array,
or scalar, per `json.Unmarshal()`'s rules) when the client's media type is JSON (the
default); otherwise the response body is returned as a plain string.

#### rest.New([username, password])

```go
func rest.New(username ...string, password ...string) (rest.Client, error)
```

Creates a new `rest.Client`, along with an error (`nil` on success). If `username` or
`password` is given, the client uses HTTP Basic authentication with those credentials.

```go
client, err := rest.New()
if err != nil {
    fmt.Println("could not create client:", err)
    return
}
```

**Security note on the ambient logon token:**
A `rest.Client`'s `Base()` can point to any server, including a host you
don't control -- so to prevent a script (trusted or not) from accidentally or
deliberately exfiltrating the user's login token to an arbitrary third party,
`New()` called from within _any_ running Ego program **never** attaches the
token automatically, regardless of arguments. A script that genuinely needs
to call back into its own Ego server using the ambient identity must opt in
explicitly with `UseToken(true)` (below).

#### The `rest.Client` structure

A `rest.Client` exposes these read-only fields, updated after each `Get()`/`Post()`/`Delete()`:

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Status` | `int` | The HTTP status code of the most recent response (e.g. `200`, `404`). |
| `Response` | `any` | The most recent response body -- a decoded Ego value for a JSON response, or a plain string otherwise. |
| `Headers` | `map[string]string` | The most recent response's HTTP headers. |
| `Cookies` | `[]any` | The most recent response's cookies, each as a `map[string]interface{}` with `name`, `value`, `domain`, `path`, and `expires` keys. |
| `MediaType` | `string` | The media type currently in effect (set via `Media()`, defaults to `"application/json"`). |

Its methods fall into two groups. The first group are fluent setters: each configures
one aspect of the client and returns the client itself, so calls can be chained.

| Method | Description |
| :------- | :---------- |
| `Base(url) rest.Client` | Sets a base URL prepended to the endpoint passed to `Get()`/`Post()`/`Delete()`. |
| `Media(type) rest.Client` | Sets the media/content type for the exchange (e.g. `"application/json"`, `"text/plain"`). |
| `Verify(flag) rest.Client` | Enables (`true`, the default) or disables (`false`) TLS server certificate validation -- turn this off only for testing against a self-signed certificate. |
| `Auth(username, password) rest.Client` | Sets HTTP Basic authentication credentials. |
| `Token(token) rest.Client` | Sets Bearer token authentication using the given token string, or the current `ego logon` token if `token` is omitted. |
| `UseToken(flag) rest.Client` | See below. |

`UseToken(flag)` is the explicit opt-in mentioned above: `UseToken(true)` attaches the
current `ego logon` token as this client's bearer token.  `UseToken(false)` removes any
bearer token currently set on the client. Both forms are refused outright (an error,
not a value, is returned -- see below) when the current execution context is sandboxed
(e.g. a server-hosted dashboard "run" session executing untrusted code): a restricted
runtime has no legitimate need for the server's own token, whether by accident or on purpose.

The second group performs the actual HTTP request and returns `(value, error)`, like
every other fallible function in the runtime:

| Method | Description |
| :------- | :---------- |
| `Get(endpoint) (any, error)` | Sends a GET request. |
| `Post(endpoint [, body]) (any, error)` | Sends a POST request, with `body` (if given) as the request payload. |
| `Put(endpoint [, body]) (any, error)` | Sends a PUT request, with `body` (if given) as the request payload. |
| `Patch(endpoint [, body]) (any, error)` | Sends a PATCH request, with `body` (if given) as the request payload. |
| `Delete(endpoint [, body]) (any, error)` | Sends a DELETE request, with `body` (if given) as the request payload. |
| `Close() error` | Closes idle connections held by the client. |

The returned `error` reflects a **transport-level** failure only -- the connection
couldn't be made at all (host unreachable, DNS failure, TLS handshake failure, etc.).
An HTTP-level error status (`404`, `500`, and so on) is not an error as far as these
methods are concerned: the call still succeeds, decodes whatever body the server sent,
and you check `client.Status` yourself to see whether the request was accepted:

```go
client, _ := rest.New()
client.Base("https://example.com").Verify(false).Media("application/json")

factors, err := client.Get("/services/factor/10")
if err != nil {
    // A transport-level failure -- could not reach the server at all.
    fmt.Println("request failed:", err)
    return
}

if client.Status != 200 {
    fmt.Println("server returned status", client.Status, ":", client.Response)
    return
}

fmt.Println(factors)   // [1, 2, 5, 10]
```

`Post()`/`Put()`/`Patch()`/`Delete()` work the same way; each takes an optional second
argument as the request body (any Ego value -- it is marshaled to JSON automatically
when the media type is JSON):

```go
result, err := client.Post("/services/factor/10", map[string]int{"note": 1})
if err != nil {
    fmt.Println("request failed:", err)
    return
}

fmt.Println(client.Status, result)
```

#### rest.Status(code)

```go
func rest.Status(code int) string
```

Returns a short human-readable description of an HTTP status code (e.g. `200` ->
`"OK"`, `404` -> `"Not found"`), falling back to `"HTTP status <code>"` for a code it
doesn't recognize.

```go
fmt.Println(rest.Status(200))   // OK
fmt.Println(rest.Status(404))   // Not found
```

#### rest.ParseURL(url [, template])

```go
func rest.ParseURL(url string, template ...string) (map[string]any, error)
```

Parses `url` into its component parts, returned as a map, along with an error (`nil`
on success).

| Key | Description |
| :----------- | :------- |
| `urlScheme` | The URL scheme, such as `"http"` or `"https"`. |
| `urlHost` | The URL host, such as `"example.com"`. |
| `urlPort` | The URL port, if one was given. |
| `urlUsername` | The username from the URL, if given. |
| `urlPassword` | The password from the URL, if given. |
| `urlPath` | The raw path string from the URL. |
| `urlQuery` | A `map[string][]string` of query parameter values, if any were given. |

A part that wasn't present in `url` has no corresponding map entry at all (rather than
an empty-string entry).

```go
parts, err := rest.ParseURL("https://user:pass@example.com:8080/api/v1/items?x=1")
if err != nil {
    fmt.Println("invalid URL:", err)
    return
}

fmt.Println(parts.urlHost, parts.urlPort, parts.urlPath)   // example.com 8080 /api/v1/items
```

If `template` is given, `url`'s path is additionally matched against it: a literal
segment in `template` that matches the corresponding segment of `url`'s path adds an
entry to the map (name equal to the segment text, value `true`); a `{{name}}`
placeholder segment instead captures whatever text appears in that position of `url`'s
path, under the key `name`. This is convenient for pulling apart a REST endpoint path
using the same `{{...}}` placeholder syntax the path was originally defined with:

```go
parts, err := rest.ParseURL("/services/factor/12", "/services/factor/{{n}}")
fmt.Println(parts.n)   // 12
```

---

### runtime Package<a name="runtime"></a>

The `runtime` package exposes a small amount of information about the platform Ego is
running on and the Ego runtime itself, along with a few functions mirroring Go's own
`runtime` package.

#### runtime.GOOS / runtime.GOARCH

```go
runtime.GOOS   string
runtime.GOARCH string
```

These constants hold the operating system and processor architecture Ego was built for,
using the same values Go's own `runtime.GOOS`/`runtime.GOARCH` do (`"darwin"`, `"linux"`,
`"windows"`, etc. for `GOOS`; `"amd64"`, `"arm64"`, etc. for `GOARCH`).

```go
import "runtime"

func main() {
    fmt.Println(runtime.GOOS, runtime.GOARCH)
}
```

On an Apple Silicon Mac, this prints:

```text
darwin arm64
```

#### runtime.Version()

```go
func runtime.Version() string
```

Returns the version string of the Go compiler used to build the running `ego` executable
(for example, `"go1.26.1"`) -- this is Go's own build, not Ego's version; see
`runtime.Ego()` below for that.

#### runtime.NumCPU()

```go
func runtime.NumCPU() int
```

Returns the number of logical CPUs available to the current process, exactly like Go's
`runtime.NumCPU()`.

#### runtime.GOMAXPROCS(n)

```go
func runtime.GOMAXPROCS(n int) int
```

Sets the maximum number of operating system threads that can execute Ego (and Go runtime)
code simultaneously to `n`, and returns the _previous_ setting -- identical to Go's
`runtime.GOMAXPROCS()`. Pass `0` to query the current value without changing it:

```go
previous := runtime.GOMAXPROCS(0)
fmt.Println("currently allowed:", previous)
```

#### runtime.GC()

```go
func runtime.GC()
```

Forces an immediate garbage-collection cycle, exactly like Go's `runtime.GC()`. This is
rarely needed; it exists mainly for diagnosing memory behavior.

#### runtime.Ego() / runtime.Buildtime()

```go
func runtime.Ego() string
func runtime.Buildtime() (time.Time, error)
```

`Ego()` returns the version string of the running `ego` executable itself (for example,
`"ego1.8-1888"`), as opposed to `runtime.Version()`, which reports the Go compiler version.
`Buildtime()` returns the timestamp the executable was built, along with an error if that
timestamp could not be parsed (for example, in a `go build` binary that was not built with
the project's normal build script, which injects the timestamp via linker flags -- see
`./tools/build` in the project `CLAUDE.md`).

```go
built, err := runtime.Buildtime()
if err != nil {
    fmt.Println("build time unavailable:", err)
} else {
    fmt.Println("built:", built.Format(time.RFC1123))
}
```

#### runtime.Frames(count) / the `runtime.Frame` type

```go
func runtime.Frames(count int) []runtime.Frame
```

Returns up to `count` call frames describing the active call stack at the point
`Frames()` is called, starting with the immediate caller and working outward. Each
`runtime.Frame` has these fields:

| Field | Type | Description |
| :--------- | :------: | :--------------------------------------------------------------- |
| `Module` | `string` | The name of the function (or source file, for the outermost frame) at that point in the call stack. |
| `Table` | `string` | The name of the symbol table scope active at that frame -- mostly useful for debugging. |
| `Line` | `int` | The source line number being executed in that frame. |

Fewer than `count` frames are returned if the call stack isn't that deep -- there is no
error in that case, the result array is just shorter.

```go
import "runtime"

func inner() {
    frames := runtime.Frames(5)
    for _, f := range frames {
        fmt.Println(f.Module, f.Line)
    }
}

func outer() {
    inner()
}

func main() {
    outer()
}
```

This prints something like:

```text
inner 4
outer 11
main 15
 18
```

(the final, blank-`Module` entry is the top-level program itself, not a named function).

#### runtime.Stack(buf, all)

```go
func runtime.Stack(buf []byte, all bool) int
```

Formats a human-readable call stack trace (similar to what an uncaught error's stack
trace shows) into `buf`, and returns the number of bytes written -- truncating if the
formatted text doesn't fit. This mirrors Go's own `runtime.Stack()` function signature,
though the `all` parameter (which in Go includes every other goroutine's stack in the
output) is currently accepted but has no effect -- Ego's version always formats only the
calling goroutine's own call stack, regardless of the value passed for `all`.

```go
buf := make([]byte, 1024)
n := runtime.Stack(buf, true)
fmt.Println(string(buf[:n]))
```

---

### sort Package<a name="sort"></a>

The `sort` package provides sorting and searching functions modeled on Go's
`sort` package. It is not as complete as Go's version -- there is no
`sort.Interface`-based generic sort, for example -- but what is implemented is
functionally equivalent. All sort functions modify the array **in place** and
also return the same array as their primary value, so both of these styles
work:

```go
a := []int{5, 3, 8, 0, -1}
sort.Ints(a)          // in-place; result discarded

b := []int{5, 3, 8, 0, -1}
b2, err := sort.Ints(b)   // in-place; b2 is the same array as b
```

Every function in the package returns `(value, error)`, like every other
fallible function in the runtime. A wrong-type argument is the most common
failure and is always a catchable error, not a program-terminating panic:

```go
_, err := sort.Strings([]int{5, 2, 6})
fmt.Println(err)   // in Strings, incorrect function argument type: []int
```

#### Type-specific sorts

Each of these sorts an array of exactly the named element type, ascending,
and returns `(array, error)`. Passing an array of any other element type
returns an error rather than sorting anything.

| Function | Element type |
| :------- | :----------- |
| `sort.Ints(array)` | `[]int` |
| `sort.Int32s(array)` | `[]int32` |
| `sort.Int64s(array)` | `[]int64` |
| `sort.Float32s(array)` | `[]float32` |
| `sort.Float64s(array)` | `[]float64` |
| `sort.Strings(array)` | `[]string` |
| `sort.Bytes(array)` | `[]byte` |

```go
a := []int{5, 3, 8, 0, -1}
sort.Ints(a)
fmt.Println(a)   // [-1, 0, 3, 5, 8]

f := []float64{5.3, 3, 8.001, 0, -1.5}
sort.Float64s(f)
fmt.Println(f)   // [-1.5, 0, 3, 5.3, 8.001]

s := []string{"apple", "pear", "", "cherry"}
sort.Strings(s)
fmt.Println(s)   // ["", "apple", "cherry", "pear"]

b := []byte{5, 3, 8, 0, 1}
sort.Bytes(b)
fmt.Println(b)   // [0, 1, 3, 5, 8]
```

#### sort.Sort(array) / sort.Stable(array)

`Sort` sorts an array of any single supported scalar element type (`byte`,
`int`, `int32`, `int64`, `float32`, `float64`, or `string`), determining the
element type automatically -- you don't have to call the type-specific
function by name. `Stable` does the same thing but guarantees elements that
compare equal keep their original relative order (at some cost in
performance).

```go
a := []int32{9, 3, 7, 1, 5}
v, err := sort.Sort(a)
fmt.Println(v, err)   // [1, 3, 5, 7, 9] <nil>

b := []int{5, 3, 1, 4, 2}
sort.Stable(b)
fmt.Println(b)   // [1, 2, 3, 4, 5]
```

An array whose element type isn't one of the supported scalar kinds (a struct
or bool array, for example) returns an error:

```go
_, err := sort.Sort([]bool{true, false})
fmt.Println(err)   // in Sort, incorrect function argument type
```

#### sort.Slice(array, less) / sort.SliceStable(array, less)

`Slice` sorts an array of **any** type -- including structs and other
non-scalar types that the type-specific sorts and `Sort`/`Stable` can't
handle -- using a custom comparison function you supply. `SliceStable` is
identical but guarantees elements that compare equal keep their original
relative order.

The comparison function must accept two `int` arguments (`i`, `j`) and return
a `bool`: `true` if the element at index `i` should sort before the element
at index `j`. It's defined as a closure so it can see the array being sorted:

```go
a := []int{101, 5, 33, -55, 239, 3, 66}

sort.Slice(a, func(i int, j int) bool {
    return a[i] < a[j]
})

fmt.Println(a)   // [-55, 3, 5, 33, 66, 101, 239]
```

This is how you sort an array of structs, since there's no scalar type to
dispatch on automatically:

```go
type Person struct {
    Name string
    Age  int
}

people := []Person{
    {Name: "Charlie", Age: 35},
    {Name: "Alice", Age: 30},
    {Name: "Bob", Age: 25},
}

sort.Slice(people, func(i int, j int) bool {
    return people[i].Age < people[j].Age
})

for _, p := range people {
    fmt.Println(p.Name, p.Age)
}
// Bob 25
// Alice 30
// Charlie 35
```

If the comparison function itself fails (for example, it makes an invalid
type assertion), `Slice`/`SliceStable` return that failure as their error
value rather than aborting the program.

#### sort.Search(n, f)

`Search` performs a binary search over the (conceptual) index range `[0, n)`,
using a predicate function `f(i int) bool` that you supply. `f` must be
_monotone_: `false` for every index below the answer, `true` for every index
at or above it. `Search` returns the smallest index where `f` is `true`, or
`n` if `f` is `false` for every index (nothing found).

The canonical use is finding the insertion point (or an exact match) in an
already-sorted array:

```go
a := []int{1, 3, 5, 7, 9, 11}

i, err := sort.Search(len(a), func(i int) bool { return a[i] >= 7 })
fmt.Println(i, err)   // 3 <nil>
fmt.Println(a[i])     // 7

// Not found: Search returns len(a)
j, _ := sort.Search(len(a), func(i int) bool { return a[i] >= 100 })
fmt.Println(j)   // 6
```

#### sort.SearchInts(array, x), sort.SearchFloat64s(array, x), sort.SearchStrings(array, x)

These are convenience wrappers around `Search` for the common case of
searching an already-sorted array of a known scalar type for a specific
value `x`, without having to write the predicate function yourself. Each
returns `(int, error)`: the smallest index at which `x` could be inserted to
keep the array sorted -- which is `x`'s own index if it's already present, or
`len(array)` if `x` is larger than every element. An array of the wrong
element type returns an error rather than searching.

```go
a := []int{1, 3, 5, 7, 9, 11}

i, err := sort.SearchInts(a, 7)
fmt.Println(i, err)   // 3 <nil>

// Not present: this is where 6 would be inserted to keep a sorted
j, _ := sort.SearchInts(a, 6)
fmt.Println(j)   // 3

f := []float64{1.1, 3.3, 5.5, 7.7}
fi, _ := sort.SearchFloat64s(f, 5.5)
fmt.Println(fi)   // 2

s := []string{"apple", "banana", "cherry", "date"}
si, _ := sort.SearchStrings(s, "cherry")
fmt.Println(si)   // 2
```

#### sort.IsSorted(array), sort.IntsAreSorted(array), sort.Float64sAreSorted(array), sort.StringsAreSorted(array)

Each of these returns `(bool, error)` -- `true` if the array is already in
non-decreasing order. `IsSorted` accepts any of the scalar element types
`Sort`/`Stable` support and determines the type automatically; the other
three additionally verify the array holds the named element type, returning
an error otherwise, just like the type-specific sorts.

```go
// Passed directly as a single fmt.Println() argument like this, only the
// primary (bool) value comes through -- the error is dropped, exactly like
// any other multi-return call used as a plain expression rather than
// assigned with ":=". Assign both return values if you need the error too.
fmt.Println(sort.IsSorted([]int{1, 2, 3}))          // true
fmt.Println(sort.IsSorted([]int{3, 1, 2}))          // false

fmt.Println(sort.IntsAreSorted([]int{-5, 0, 3}))       // true
fmt.Println(sort.Float64sAreSorted([]float64{1, 2}))   // true
fmt.Println(sort.StringsAreSorted([]string{"a", "b"})) // true
```

---

### sql Package<a name="sql"></a>

The `sql` package provides database access, mirroring Go's own
`database/sql` package with some Ego-specific conveniences. It supports:

- A Postgres database, or any database using the Postgres wire protocol.
- A SQLite database in the file system.

`sql.Open()` creates a `sql.Database` connection handle. With it, you can
execute a SQL statement and get back either the entire result set as an
Ego array (`QueryResult()` -- convenient for small result sets), or a
`sql.Rows` cursor object used to step through a result set of arbitrary
size a row at a time (`Query()`).

Every fallible function and method in this package returns `(value, error)`,
like everywhere else in the runtime.

#### sql.Open(driver, connectionString)

```go
func sql.Open(driver string, connectionString string) (sql.Database, error)
```

`driver` must be one of:

| Driver | Description |
| :----- | :----------- |
| `"sqlite"` | A SQLite database file; `connectionString` is the file system path. `"sqlite3"` is accepted as a deprecated alias for the same driver. |
| `"postgres"` | A Postgres connection string or URL, e.g. `"postgres://user:pass@host/dbname?sslmode=disable"`. |
| `"dsn"` | The name of a data source pre-registered with the Ego server; `connectionString` is that DSN's name. |

```go
d, err := sql.Open("sqlite", "/tmp/example.db")
if err != nil {
    fmt.Println("could not open database:", err)
    return
}

_, err = d.Execute("CREATE TABLE member (id INTEGER, name TEXT, age INTEGER)")
_, err = d.Execute("INSERT INTO member VALUES (1, 'Alice', 30)")
_, err = d.Execute("INSERT INTO member VALUES (2, 'Bob', 25)")

r, err := d.QueryResult("select * from member")
fmt.Println(r, err)

d.Close()
```

`QueryResult()` always fetches the entire result set at once, which can use a
lot of memory for a query with no filtering -- use `Query()`'s cursor instead
for large result sets. Query parameters are passed as additional arguments
and substituted positionally using `$1`, `$2`, etc.:

```go
age := 21
r, err := d.QueryResult("select * from member where age >= $1", age)
```

As a security precaution, opening a SQLite database whose file name matches
the server's own credentials database (`ego.server.userdata`, normally
`ego-system.db`) is always rejected, regardless of directory, to prevent
sandboxed Ego code from reading or tampering with server authentication data.

#### The `sql.Database` structure

| Field | Type | Description |
| :---------- | :------: | :--------------------------------------------------------------- |
| `StructMode` | `bool` | Set via `AsStruct()`; when `true`, `Query()`/`QueryResult()` rows are structs keyed by column name instead of arrays. |
| `Rowcount` | `int` | Number of rows affected or returned by the most recent call. Reset to `-1` by `Close()`. |
| `Constr` | `string` | The (password-redacted) connection string used to open this database. |

```go
d, _ := sql.Open("sqlite", "/tmp/example.db")
fmt.Println(d.Constr)      // /tmp/example.db
fmt.Println(d.StructMode)  // false
fmt.Println(d.Rowcount)    // 0

d.Close()
fmt.Println(d.Rowcount)    // -1
```

Its methods:

| Method | Description |
| :------- | :---------- |
| `Begin() error` | Starts a transaction. Only one transaction may be active per connection at a time. |
| `Commit() error` | Commits the active transaction. |
| `Rollback() error` | Discards the active transaction. |
| `Execute(sql [, args...]) (int, error)` | Runs a non-`SELECT` statement; the value is the number of rows affected (typically `0` for DDL). |
| `Query(sql [, args...]) (sql.Rows, error)` | Runs a `SELECT` and returns a cursor for stepping through the result one row at a time. |
| `QueryResult(sql [, args...]) ([][]interface{}, error)` | Runs a `SELECT` and eagerly fetches the entire result set. |
| `AsStruct(flag) (sql.Database, error)` | Sets `StructMode`; returns the `Database` itself so calls can be chained. |
| `Close() error` | Closes the connection and frees its resources. |

While a transaction is active (after `Begin()`, before `Commit()`/`Rollback()`),
`Execute()`, `Query()`, and `QueryResult()` automatically run inside it:

```go
err := d.Begin()
_, _ = d.Execute("INSERT INTO member VALUES (3, 'Eve', 40)")
err = d.Rollback()   // discards the insert

err = d.Begin()
_, _ = d.Execute("INSERT INTO member VALUES (4, 'Frank', 50)")
err = d.Commit()     // makes the insert permanent
```

`AsStruct(true)` switches `QueryResult()` and `Query()`/`Scan()` rows from
positional arrays to structs keyed by column name:

```go
d.AsStruct(false)   // default
rows, _ := d.QueryResult("SELECT id, name FROM member")
fmt.Println(rows[0][1])       // "Alice" -- indexed by column position

d.AsStruct(true)
rows2, _ := d.QueryResult("SELECT id, name FROM member")
fmt.Println(rows2[0].name)    // "Alice" -- indexed by column name
```

#### The `sql.Rows` cursor

Returned by `Database.Query()`, for stepping through a result set one row at
a time without loading it all into memory:

| Method | Description |
| :------- | :---------- |
| `Next() (bool, error)` | Advances to the next row; the value is `false` once the result set is exhausted (or the cursor is already closed). Must be called before each `Scan()`. |
| `Scan(...) (interface{}, error)` | Reads the current row. See below for its two calling conventions. |
| `Headings() ([]string, error)` | Column names, in `SELECT`-list order. |
| `Close() error` | Releases the cursor's resources. Always close a cursor when done with it, even if every row was consumed. |

`Scan()` has two calling conventions:

- **No arguments** -- returns the row as its value: an array of column
  values in `SELECT`-list order (`Database.StructMode == false`, the
  default), or a struct keyed by column name (`StructMode == true`).
- **One `*interface{}` pointer per column** -- mimics Go's own
  `sql.Rows.Scan()`: writes each column value into the corresponding
  pointer and returns a `nil` value.

```go
rows, err := d.Query("SELECT id, name, age FROM member ORDER BY id")
fmt.Println(err)

headings, _ := rows.Headings()
fmt.Println(headings)   // ["id", "name", "age"]

for rows.Next() {
    row, _ := rows.Scan()
    fmt.Println(row)
}

rows.Close()
```

Struct mode, with a parameterized query:

```go
d.AsStruct(true)

rows2, _ := d.Query("SELECT id, name, age FROM member WHERE id = $1", 2)
for rows2.Next() {
    info, _ := rows2.Scan()
    fmt.Println(info.name, info.age)
}

rows2.Close()
```

---

### strconv Package<a name="strconv"></a>

The `strconv` package performs conversions to and from string representations
of other data types, mirroring Go's own `strconv` package. Nearly every
function here is a direct native passthrough to the identically-named Go
function -- the parameter and return types below are exactly Go's, so Go's
`strconv` documentation applies for any behavioral detail not covered here.
The two exceptions are `Itor`/`Rtoi`, which are Ego-only additions with no
Go equivalent.

#### Parsing strings to numbers

| Function | Description |
| :------- | :----------- |
| `Atoi(s string) (int, error)` | Parses a base-10 integer. Equivalent to `ParseInt(s, 10, 0)` converted to `int`. |
| `ParseInt(s string, base int, bitSize int) (int64, error)` | Parses a signed integer. `base` `0` infers the base from the string's prefix (`0x`, `0o`, `0b`, or none for decimal); `bitSize` (`0`, `8`, `16`, `32`, `64`) is the integer type the result must fit in (`0` means `int`). |
| `ParseUint(s string, base int, bitSize int) (uint64, error)` | Like `ParseInt`, but for unsigned integers -- a leading `-` is always an error. |
| `ParseFloat(s string, bitSize int) (float64, error)` | Parses a floating-point value. `bitSize` (`32` or `64`) controls how the result is rounded, though the return value is always `float64`. |
| `ParseComplex(s string, bitSize int) (complex128, error)` | Parses a complex value written as `"3+4i"` (or `"(3+4i)"`). `bitSize` (`64` or `128`) controls how each real/imaginary component is rounded, though the return value is always `complex128`. |
| `ParseBool(s string) (bool, error)` | Accepts `"1"`, `"t"`, `"T"`, `"TRUE"`, `"true"`, `"True"` as `true`, and `"0"`, `"f"`, `"F"`, `"FALSE"`, `"false"`, `"False"` as `false`. Anything else is an error. |

```go
v, err := strconv.Atoi("42")
fmt.Println(v, err)   // 42 <nil>

_, err = strconv.Atoi("abc")
fmt.Println(err)      // strconv.Atoi: parsing "abc": invalid syntax

n, err := strconv.ParseInt("ff", 16, 64)
fmt.Println(n, err)   // 255 <nil>

u, err := strconv.ParseUint("42", 10, 64)
fmt.Println(u, err)   // 42 <nil>

f, err := strconv.ParseFloat("3.14", 64)
fmt.Println(f, err)   // 3.14 <nil>

c, err := strconv.ParseComplex("3+4i", 128)
fmt.Println(c, err)   // (3+4i) <nil>

b, err := strconv.ParseBool("T")
fmt.Println(b, err)   // true <nil>
```

`Atoi` does **not** understand `0x`/`0o`/`0b` prefixes -- it is always base
10, matching Go's own `Atoi` (which is defined as `ParseInt(s, 10, 0)`). Use
`ParseInt(s, 0, 64)` if you need the base inferred from the string's prefix.

#### Formatting numbers as strings

| Function | Description |
| :------- | :----------- |
| `Itoa(i int) string` | Formats an `int` in base 10. |
| `FormatInt(i int64, base int) string` | Formats a signed integer in the given `base` (2-36). Requires an `int64` argument. |
| `FormatUint(i uint64, base int) string` | Like `FormatInt`, but for unsigned integers. Requires a `uint64` argument. |
| `FormatFloat(f float64, format byte, precision int, bitSize int) string` | Formats a float; see below for `format`. |
| `FormatComplex(c complex128, format byte, precision int, bitSize int) string` | Formats a complex value as `"(3+4i)"`; `format`, `precision`, and `bitSize` apply to each real/imaginary component the same way they do for `FormatFloat`. |
| `FormatBool(b bool) string` | Returns `"true"` or `"false"`. |

```go
fmt.Println(strconv.Itoa(42))                          // 42
fmt.Println(strconv.FormatInt(int64(255), 16))         // ff
fmt.Println(strconv.FormatUint(uint64(255), 16))       // ff
fmt.Println(strconv.FormatFloat(3.14159, 'f', 2, 64))  // 3.14
fmt.Println(strconv.FormatComplex(3+4i, 'g', -1, 128)) // (3+4i)
fmt.Println(strconv.FormatBool(true))                  // true
```

`FormatInt` and `FormatUint` take Go's native `int64`/`uint64` types, not
`int`. In dynamic mode a plain `int` argument (e.g. `strconv.FormatInt(42,
10)`) is coerced automatically; strict mode requires the explicit cast shown
above (`int64(42)`) -- write the cast either way if your code needs to run
under both type modes.

`format` is a single byte selecting the output style:

| `format` | Style |
| :------- | :---- |
| `'b'` | `-ddddp±ddd`, a binary exponent |
| `'e'` | `-d.dddde±dd`, a decimal exponent |
| `'E'` | `-d.ddddE±dd`, a decimal exponent |
| `'f'` | `-ddd.dddd`, no exponent |
| `'g'` | `'e'` for large exponents, `'f'` otherwise |
| `'G'` | `'E'` for large exponents, `'f'` otherwise |
| `'x'` | `-0xd.ddddp±ddd`, a hexadecimal fraction and binary exponent |
| `'X'` | `-0Xd.ddddP±ddd`, a hexadecimal fraction and binary exponent |

`precision` is the number of digits after the decimal point (`-1` selects the
smallest number of digits that round-trips exactly); `bitSize` (`32` or `64`)
tells `FormatFloat` whether to round as if `f` were a `float32` or a native
`float64` before formatting.

#### Roman numerals

`Itor` and `Rtoi` are Ego-only additions with no equivalent in Go's
`strconv` package.

| Function | Description |
| :------- | :----------- |
| `Itor(i int) (string, error)` | Converts an integer in the range 1-3999 to a Roman numeral string. Values outside that range (there is no standard Roman representation of 0 or negative numbers, and this package does not implement the vinculum notation used for numbers 4000 and above) return an error. |
| `Rtoi(s string) (int, error)` | Converts a Roman numeral string back to an integer. Case-insensitive; leading/trailing whitespace is ignored. An empty (or all-whitespace) string returns `0` with no error. |

```go
r, err := strconv.Itor(1994)
fmt.Println(r, err)   // MCMXCIV <nil>

n, err := strconv.Rtoi("MCMXCIV")
fmt.Println(n, err)   // 1994 <nil>

_, err = strconv.Itor(4000)
fmt.Println(err)      // in Itor, Roman integers must be in range of 1..3999
```

#### Quoting strings and runes

| Function | Description |
| :------- | :----------- |
| `Quote(s string) string` | Wraps `s` in double quotes, Go-escaping any special characters. |
| `Unquote(s string) (string, error)` | Reverses `Quote` -- also accepts a single-quoted rune literal or a raw backquoted string. Returns an error if `s` isn't validly quoted. |
| `QuoteRune(r rune) string` | Formats `r` as a single-quoted Go rune literal, e.g. `'A'`. |
| `QuoteRuneToASCII(r rune) string` | Like `QuoteRune`, but escapes non-ASCII runes. |
| `QuoteRuneToGraphic(r rune) string` | Like `QuoteRune`, but only escapes non-graphic runes. |
| `QuoteToASCII(s string) string` | Like `Quote`, but escapes non-ASCII runes. |
| `QuoteToGraphic(s string) string` | Like `Quote`, but only escapes non-graphic runes. |

```go
fmt.Println(strconv.Quote(`say "hi"`))                 // "say \"hi\""

s, err := strconv.Unquote(`"say \"hi\""`)
fmt.Println(s, err)                                     // say "hi" <nil>

fmt.Println(strconv.QuoteRune('A'))                     // 'A'
fmt.Println(strconv.QuoteRuneToASCII('A'))              // 'A'
fmt.Println(strconv.QuoteRuneToGraphic('A'))            // 'A'
fmt.Println(strconv.QuoteToASCII("hello"))              // "hello"
fmt.Println(strconv.QuoteToGraphic("hello"))            // "hello"
```

#### Inspecting characters and strings

| Function | Description |
| :------- | :----------- |
| `CanBackquote(s string) bool` | Reports whether `s` can be represented unchanged as a raw (backquoted) string literal -- `false` if it contains a backquote, a newline, or most other control characters. |
| `IsPrint(r rune) bool` | Reports whether `r` is defined as printable by Go (letters, marks, numbers, punctuation, symbols, and the ASCII space). |
| `IsGraphic(r rune) bool` | Like `IsPrint`, but also includes the Unicode `Space` category beyond ASCII space (a broader definition of "graphic" per the Unicode standard). |

```go
fmt.Println(strconv.CanBackquote("hello"))          // true
fmt.Println(strconv.CanBackquote("with\nnewline"))  // false
fmt.Println(strconv.IsPrint('A'))                   // true
fmt.Println(strconv.IsGraphic('A'))                 // true
```

---

### strings Package<a name="strings"></a>

The `strings` package contains a library of functions to support manipulation
of string data. Unless otherwise noted, strings are interpreted as a set of
characters, so some unicode characters can take more than one byte of storage.

The `strings` package includes two type definitions, `strings.Builder` and
`strings.Reader` as well as a large number of standard functions, based on the
Go standard library.

#### strings.Builder

This is a type that defines an object used to efficiently construct a string by
adding strings and runes to the object, and then extracting the fully-composited
string when building is complete. Here are the methods available for an instance
of a `strings.Builder{}` object:

| Method | Description |
| ------ | ----------- |
| (b Builder) Cap() int | Returns the currently allocated capacity of the builder, in bytes |
| (b Builder) Grow(n int) int | Increase the capacity bytes. Returns new total Cap value |
| (b Builder) Len() int | Returns the length of the currently composite string in bytes |
| (b Builder) Reset() | Empties out the builder as if it was just created |
| (b Builder) String() string | Returns the composited string value |
| (b Builder) Write(d []byte) (int, error) | Write a byte array to the builder |
| (b Builder) WriteByte(c byte) (int, error) | Write a single byte to the builder |
| (b Builder) WriteRune(r int32) (int, error) | Write a single rune to the builder |
| (b Builder) WriteString(s string) (int, error) | Write a string to the builder |

For functions that return (int, error) the integer is the new length of the composited
string, and the error is nil if no error occurs.

#### strings.Reader

This is a type that creates a io.Reader object for a string value. The resulting Reader
can be used anywhere an io.Reader is used. It supports the `Read(buff []byte)` method which
reads bytes from the string into the buffer, and returns the number of bytes read and any
error that occurred.

#### strings.Camel(string)

The `Camel()` function converts a string to title case: the first character is
converted to upper case and all remaining characters are converted to lower case.

```go
a := strings.Camel("teST")
b := strings.Camel("HELLO")
```

In these examples `a` will be `"Test"` and `b` will be `"Hello"`. An empty
string returns an empty string; a single character is returned in upper case.

#### strings.Chars(s)

The `Chars` function returns an array of string values. Each value represents
a single character for that position in the string.

```go
runes := strings.Chars("test")
```

The value of `runes` is an string array with values ["t", "e", "s", "t"].
If the string is an empty string, it results in an array of zero elements.

#### strings.Clone(text)

The `Clone` function returns a copy of the given string. Because Ego strings
are already immutable values, this is primarily useful for parity with Go's
`strings` package.

```go
a := "hello"
b := strings.Clone(a)
```

#### strings.Compare(a, b)

The `Compare` function compares two string values, and returns an integer containing
-1 if the first string is less than the second, 0 if they are equal, or 1 if the
second value is less than the first value.

```go
fmt.Println(strings.Compare("peach", "apple"))
```

This will print the value 1 as the second value sorts higher in order than
the first value.

#### strings.Contains(str, substr)

The `Contains` function scans a string for a substring and returns a boolean
value indicating if the substring exists in the string

```go
a := strings.Contains("This is a test", "is a")
b := strings.Contains("This is a test", "isa")
```

In this example, `a` contains the value `true`, and `b` contains the value `false`.
Note that the substring must match exactly, including whitespace, to be considered
a match.

#### strings.ContainsAny(str, chars)

The `ContainsAny` function scans a string to see if instances of any of the
characters from a substring appear in the string.

```go
a := strings.ContainsAny("this is a test", "satx")
b := strings.ContainsAny("this is a test", "xyz")
```

In this example, `a` is true because the string contains at least one of the
characters in the substring (there are instances of "s", "a", and "t"). The
value of `b` is false because the string does not contain any instances of
("x", "y", or "z")

#### strings.ContainsRune(str, r)

The `ContainsRune` function reports whether the given rune (character)
appears anywhere in the string.

```go
a := strings.ContainsRune("hello", 'e')
b := strings.ContainsRune("hello", 'z')
```

In this example, `a` is `true` and `b` is `false`.

#### strings.Count(string, substr)

The `Count()` function counts the number of non-overlapping occurrences of a
substring within a string.

```go
n := strings.Count("cheese", "e")
```

This returns `3` because the letter `"e"` appears three times in `"cheese"`. If
the substring is an empty string, `Count` returns the number of characters in
the string plus one.

#### strings.Cut(string, sep)

The `Cut()` function searches the string for the first occurrence of the
separator and returns the portion before it, the portion after it, and a boolean
indicating whether the separator was found.

```go
before, after, found := strings.Cut("user@example.com", "@")
```

In this example `before` is `"user"`, `after` is `"example.com"`, and `found`
is `true`. If the separator is not present, `before` is the original string,
`after` is an empty string, and `found` is `false`.

#### strings.CutPrefix(text, prefix)

The `CutPrefix` function removes the given prefix from the string, if
present, returning the remainder and a boolean indicating whether the prefix
was found.

```go
after, found := strings.CutPrefix("foobar", "foo")
```

In this example, `after` is `"bar"` and `found` is `true`. If the prefix is
not present, `after` is the original string and `found` is `false`.

#### strings.CutSuffix(text, suffix)

The `CutSuffix` function removes the given suffix from the string, if
present, returning the remainder and a boolean indicating whether the suffix
was found.

```go
before, found := strings.CutSuffix("foobar", "bar")
```

In this example, `before` is `"foo"` and `found` is `true`. If the suffix is
not present, `before` is the original string and `found` is `false`.

#### strings.EqualFold(a, b)

The `EqualFold` function compares two strings for equality, ignoring differences
in case.

```go
a := strings.EqualFold("symphony b", "Symphony B")
b := strings.EqualFold("åto", "Åto")
```

In both these examples, the result is `true`.

#### strings.Fields

The `Fields` function breaks a string down into individual strings based on
whitespace characters.

```go
s := "this is    a test"
b := strings.Fields(s)
```

The result is that `b` will contain the array ["this", "is", "a", "test"]

#### strings.Generate

The `Generate` function generates random strings of English words. These can be
used to create random names, password strings, etc.

```go
name := strings.Generate(3)
```

This generates a string containing three random English words, from an internal
word dictionary. By default, the words are in "Pascal case" which means that each
word in the string starts with a capital letter.

You can optionally specify a string that is placed between each word in the list.
For example:

```go
name = strings.Generate(3, "-")
```

This generates a string containing three words, separated by a dash (`"-"`) character.
When a separator is provided, the words are all in lower case by default.

You can also provide a third boolean argument to explicitly control case. When
`true`, words use Pascal case (each word capitalized); when `false`, all words
are lower case regardless of whether a separator was supplied.

```go
name = strings.Generate(3, "-", true)
```

This generates three Pascal-case words joined by dashes, such as `"Brown-Fox-Jumps"`.

#### strings.Join

The `Join` function joins together an array of strings with a separator string.
The separator is placed between items, but not at the start or end of the
resulting string.

```go
a := []string{ "usr", "local", "bin"}
b := strings.Join(a, "/")
```

The result is that `b` contains a string "usr/local/bin". This function is most
commonly used to create lists (with a "," for separator) or path names (using a
host-specific path separator like "/" or "\").

#### strings.Format(format, args...)

The `Format()` function is equivalent to `fmt.Sprintf()`: the first argument
is a format string using the standard `%d`/`%s`/`%f`/`%q`/etc. verbs, and the
remaining arguments are substituted into it in order. The formatted result is
returned as a string instead of being printed.

```go
fmt.Println(strings.Format("%s has %d items", "cart", 3))  // cart has 3 items
fmt.Println(strings.Format("%05d", 42))                     // 00042
fmt.Println(strings.Format("plain text"))                   // plain text
```

#### strings.HasPrefix(text, prefix) / strings.HasSuffix(text, suffix)

The `HasPrefix` and `HasSuffix` functions report whether a string begins or
ends with the given substring, respectively.

```go
a := strings.HasPrefix("hello world", "hello")
b := strings.HasSuffix("hello world", "world")
```

In this example, both `a` and `b` are `true`. An empty prefix or suffix
always matches.

#### strings.Index(string, test)

The `Index` function searches a string for the first occurrence of the test
string. If it is found, it returns the character position of the first
character in `string` that contains the value of `test`. If no instance of
the test string is found, the function returns -1.

#### strings.IndexAny(text, chars)

The `IndexAny` function returns the index of the first instance of any
character from `chars` found in `text`, or -1 if none of the characters are
present.

```go
n := strings.IndexAny("hello", "aeiou")
```

In this example, `n` is `1`, the position of the first vowel.

#### strings.IndexByte(text, c)

The `IndexByte` function returns the index of the first instance of the byte
`c` in `text`, or -1 if not present. The byte value must be cast explicitly,
for example `byte('l')`.

```go
n := strings.IndexByte("hello", byte('l'))
```

In this example, `n` is `2`.

#### strings.IndexRune(text, r)

The `IndexRune` function returns the index of the first instance of the rune
(character) `r` in `text`, or -1 if not present.

```go
n := strings.IndexRune("hello", 'l')
```

In this example, `n` is `2`.

#### strings.Ints(string)

The `Ints` function returns an array of integer values. Each value represents
the Unicode character for that position in the string, expressed as an integer
value.

```go
runes := strings.Ints("test")
```

The value of `runes` is an integer array with values [116, 101, 115, 116] which
are the Unicode character values for the letters "t", "e", "s", and "t". If
the string passed is is am empty string, the `Ints` function returns an empty
array.

#### strings.LastIndex(text, substr) / strings.LastIndexAny(text, chars) / strings.LastIndexByte(text, c)

These functions are the same as `Index`, `IndexAny`, and `IndexByte`, except
they return the position of the _last_ matching occurrence instead of the
first. Each returns -1 if there is no match.

```go
a := strings.LastIndex("go gopher", "go")
b := strings.LastIndexAny("hello", "aeiou")
c := strings.LastIndexByte("hello", byte('l'))
```

In this example, `a` is `3`, `b` is `4`, and `c` is `3`.

#### strings.Left(string, count)

The `Left()` function returns the left-most characters of the given string. If
the value of the count parameter is less than 1, an empty string is returned.
If the count value is larger than the string length, then the entire string
is returned.

```go
name := "Bob Smith"
first := strings.Left(name, 3)
```

In this example, the value of `first` will be "Bob".

#### strings.Length(string)

The `Length()` function returns the length of a string _in characters_. This
is different than the builtin `len()` function which returns the length of a
string in bytes. This difference is because a character can take up more than
one byte. For example,

```go
str := "\u2813foo\u2813"
a := len(str)
b := strings.Length(str)
```

In this example, the value of `a` will be 9, which is the number of bytes stored
in the string. But because the first and last characters are unicode characters
that take multiple bytes, the value of `b` will be 5, indicating that there are
five characters in the string.

#### strings.NewReader(text)

The `NewReader()` function creates a `strings.Reader` object from the given
string. The result implements the `io.Reader` interface and can be passed
anywhere an `io.Reader` is expected.

```go
r := strings.NewReader("Hello, World!")
```

The resulting reader delivers the bytes of the string sequentially. See
`strings.Reader` above for the methods available on the returned object.

#### strings.Repeat(text, count)

The `Repeat()` function returns a new string consisting of the given string
repeated `count` times.

```go
a := strings.Repeat("-", 20)
```

The value of `a` will be a string of twenty dash characters. If `count` is zero, an empty string is
returned.

#### strings.Replace(text, old, new, count)

The `Replace()` function returns a copy of `text` with the first `count`
non-overlapping occurrences of `old` replaced by `new`. If `count` is `-1`,
all occurrences are replaced (equivalent to `ReplaceAll`).

```go
a := strings.Replace("oink oink oink", "oink", "moo", 2)
```

The value of `a` is `"moo moo oink"` — only the first two occurrences are
replaced.

#### strings.ReplaceAll(text, old, new)

The `ReplaceAll()` function returns a copy of `text` with every non-overlapping
occurrence of `old` replaced by `new`.

```go
a := strings.ReplaceAll("oink oink oink", "oink", "moo")
```

The value of `a` is `"moo moo moo"`.

#### strings.Right(string, count)

The `Right()` function returns the right-most characters of the given string.
If the value of the count parameter is less than 1, an empty string is returned.
If the count value is larger than the string length, then the entire string
is returned.

```go
name := "Bob Smith"
last := strings.Right(name, 5)
```

In this example, the value of `last` will be "Smith".

#### strings.Split(string [, delimiter])

The `Split()` function will split a string into an array of strings, based on a
provided delimiter character. If the character is not present, then a newline
is assumed as the delimiter character.

```go
a := "This is\na test\nstring"
b := strings.Split(a)
```

In this example, `b` will be an array of strings with three members, one for each
line of the string:  ["This is", "a test", "string"]. If the string given was an
empty string, the result is an empty array.

If you wish to use your own delimiter, you can supply that as the second parameter.
For example,

```go
a := "101, 553, 223, 59"
b := strings.Split(a, ", ")
```

This uses the string ", " as the delimiter. Note that this must exactly match, so
the space is significant. The value of b will be "101", "553", "223", "59"].

#### strings.SplitAfter(text, separator)

Like `Split`, but each element of the resulting array includes the separator
that follows it (except possibly the last).

```go
a := strings.SplitAfter("a,b,c", ",")
```

In this example, `a` is `["a,", "b,", "c"]`.

#### strings.SplitAfterN(text, separator, n) / strings.SplitN(text, separator, n)

These behave like `SplitAfter` and `Split` respectively, but stop after at
most `n` substrings. A negative `n` means no limit (the same as the
unbounded form); an `n` of zero returns an empty array.

```go
a := strings.SplitAfterN("a,b,c", ",", 2)
b := strings.SplitN("a,b,c", ",", 2)
```

In this example, `a` is `["a,", "b,c"]` and `b` is `["a", "b,c"]`.

#### strings.String(n1, n2...)

The `String()` function will construct a string from an array of numeric values or
string values.

```go
a := strings.String(115, 101, 116, 115)
```

This results in `a` containing the value "sets", where each integer value was used
as a Unicode character specification to construct the string.

```go
b := strings.String("this", "and", "that")
```

You can also specify arguments that are string values (including individual
characters) and they are concatenated together to make a string. In the above
example, `b` contains the string "thisandthat".

#### strings.Substitution(text, values)

The `Substitution()` function replaces `{{key}}` markers in a template string
with values drawn from a struct or map. The key inside the braces names a field
in the struct or a key in the map.

```go
person := struct{ First string; Last string }{ First: "Tom", Last: "Smith" }
msg := strings.Substitution("Hello, {{First}} {{Last}}!", person)
```

The value of `msg` will be `"Hello, Tom Smith!"`. Any marker whose key is not
found in `values` is left unchanged in the result.

#### strings.Substring(string, start, count)

The `Substring()` function extracts a portion of the string provided. The
start position is the first character position to include (1-based), and
the count is the number of characters to include in the result. For example,

```go
name := "Abe Lincoln"
part := strings.Substring(name, 5, 4)
```

This would result in `part` containing the string "Linc", representing the
starting with the fifth character, and being four characters long.

Out-of-range arguments never produce an error; they return an empty string
(or a truncated result) instead:

```go
fmt.Println(strings.Substring("Abe Lincoln", -1, 4))    // "" -- negative start
fmt.Println(strings.Substring("Abe Lincoln", 5, -1))    // "" -- negative count
fmt.Println(strings.Substring("Abe Lincoln", 100, 4))   // "" -- start beyond the string
fmt.Println(strings.Substring("Abe Lincoln", 9, 10))    // "oln" -- count truncated to fit
```

#### strings.Template(name [, struct])

The `Template()` function executes a template operation, using the supplied
data structures. See the `@template` directive for more details on creating
a template name. The struct contains values that can be substituted into the
template as it is processed. The structure's fields are used as substitution
names in the template, and the field values is used in its place in the
string.

```go
@template myNameIs `Hello, my name is {{.First}} {{.Last}}`

person := { First: "Tom", Last: "Smith"}
label , err := strings.Template( myNameIs, person )

if err != nil {
    fmt.Println(err)
} else {
    fmt.Println(label)
}
```

This program will print the string "Hello, my name is Tom Smith". The template
substitutions `{{.First}}` and `{{.Last}}` extract the specified field names
from the structure. Note that, unlike Go templates, you can reference a template
in another template without taking any special additional actions. So the template
can use the `{{template "name"}}` syntax, and as long as you have executed an
@template operation for the _name_ template, then it is automatically included
in the template before executing the query.

Note that @template creates a symbol with the given template, but that value
can only be used in the call to strings.Template() to identify the specific
template to use.

#### strings.Title(text)

The `Title()` function returns a copy of the string with the first letter of
each space-separated word converted to uppercase.

```go
a := strings.Title("hello world")
```

In this example, `a` is `"Hello World"`. Note that Go itself deprecates this
function in favor of Unicode-aware casing packages for anything beyond basic
ASCII text; it is included here for parity with the full `strings` API.

#### strings.ToLower(string)

The `ToLower()` function converts the characters of a string to the lowercase
version of that character, if there is one. If there is no lowercase for a
given character, the character is not affected in the result.

```go
a := "Mazda626"
b := strings.ToLower(a)
```

In this example, the value of `b` will be "mazda626".

#### strings.ToUpper(string)

The `ToUpper()` function converts the characters of a string to the uppercase
version of that character, if there is one. If there is no uppercase value for a
given character, the character is not affected in the result.

```go
a := "Bang+Olafsen"
b := strings.ToUpper(a)
```

In this example, the value of `b` will be "BANG+OLAFSEN".

#### strings.ToTitle(string)

The `ToTitle()` function converts every character of a string to its
Unicode title case (for most characters, this is the same as uppercase).

```go
a := strings.ToTitle("hello")
```

In this example, the value of `a` will be "HELLO".

#### strings.ToValidUTF8(text, replacement)

The `ToValidUTF8()` function returns a copy of the string with each run of
invalid UTF-8 byte sequences replaced by the given replacement string.
Adjacent invalid bytes are collapsed into a single replacement.

```go
a := strings.ToValidUTF8("hello", "?")
```

In this example, since `"hello"` is already valid UTF-8, `a` is unchanged:
`"hello"`.

#### strings.Tokenize(string)

The `Tokenize()` function uses the built-in tokenizer to break
a string into its tokens based on the _Ego_ language rules. The
result is passed back as an array, where each array contains a
structure with the token type and the token spelling

```go
s := "x{} <- f(3, 4)"
t := strings.Tokenize(s)
```

This results in `t` being a []struct array. For example, t[0] contains the
structure `{kind:"Identifier", spelling:"x"}` and t[1] contains the structure
`{kind:"Special":, spelling:"{}"}`. Note that {} is considered a single token
in the language, as is &lt;- so they each occupy a single token in the resulting
array of token structures.

The tokenizer also converts integer radix values (such as the binary value 0b101)
into decimal integers, so the resulting structure for this token would be

```text
{kind: "Integer", spelling: "5"}
```

#### strings.Trim(text, cutset) / strings.TrimLeft(text, cutset) / strings.TrimRight(text, cutset)

The `Trim` function removes any leading and trailing characters that appear
in `cutset` from the string. `TrimLeft` and `TrimRight` do the same, but only
from the beginning or end of the string, respectively.

```go
a := strings.Trim("  hello  ", " ")
b := strings.TrimLeft("xxhello", "x")
c := strings.TrimRight("helloxx", "x")
```

In this example, `a` is `"hello"`, `b` is `"hello"`, and `c` is `"hello"`.
Unlike `TrimSpace`, the cutset is a set of individual characters to remove,
not a fixed substring.

#### strings.TrimPrefix(text, prefix)

The `TrimPrefix()` function returns the string with the specified leading prefix
removed. If the string does not begin with the prefix, it is returned unchanged.

```go
a := strings.TrimPrefix("/usr/local/bin", "/usr")
```

The value of `a` is `"/local/bin"`. Unlike `Replace`, only a single leading
occurrence of the prefix is removed.

#### strings.TrimSpace(text)

The `TrimSpace()` function returns the string with all leading and trailing
white space removed. White space includes spaces, tabs, newlines, and other
Unicode white-space characters.

```go
a := strings.TrimSpace("   hello   ")
```

The value of `a` is `"hello"`. Characters in the middle of the string are not
affected.

#### strings.TrimSuffix(text, suffix)

The `TrimSuffix()` function returns the string with the specified trailing suffix
removed. If the string does not end with the suffix, it is returned unchanged.

```go
a := strings.TrimSuffix("index.ego", ".ego")
```

The value of `a` is `"index"`. Unlike `Replace`, only a single trailing
occurrence of the suffix is removed.

#### strings.Truncate(string, len)

The `Truncate()` function will truncate a string that is too long, and add
the ellipsis ("...") character at the end to show that there is more information
that was not included.

```go
a := "highway bridge out of order"
msg := strings.Truncate(a, 10)
```

In this example, the value of `msg` is "highway...". This is to ensure that the
resulting string is only ten characters long (the length specified as the second
parameter). If the string is not longer than the given count, the entire string is
returned.

#### strings.URLPattern()

The `URLPattern()` function can be used in a web service to determine what parts of
a URL are present. This is particularly useful when using collection-style URL names,
where each part of the path could define a collection type, followed optionally by
an instance identifier of a specific member of tht collection, etc. Consider
the following example:

```go
p := "/services/proc/{{pid}}/memory"
u := "/services/proc/1553/memory"

m := strings.URLPattern(u,p)
```

The `p` variable holds a pattern. It contains a number of segments of the URL, and
for one of them, specifies the indicator for a substitution value. That is, in this
part of the URL, any value is accepted and will be named "pid". The resulting
map generated by the call looks like this:

```json
{
    "services" : true,
    "proc": true,
    "pid": "1553",
    "memory": true,
}
```

For items that are constant segments of the URL, the map contains a boolean value
indicating if it was found in the pattern. For the substitution operator(s) in the
pattern, the map key is the name from the pattern, and the value is the value from
the URL. Note that this can be used to determine partial paths:

```go
p := "/services/proc/{{pid}}/memory"
u := "/services/proc/"

m := strings.URLPattern(u,p)
```

In this case, the resulting map will have a "pid" member that is an empty string,
and a "memory" value that is false, which indicates neither the substitution value
or the named field in the URL are present.

You an specify a pattern that covers an entire hierarchy, and the return will
indicate how much of the hierarchy was returned.

```go
p := "/services/location/state/{{stateName}}/city/{{cityName}}
```

If the supplied URL was `/services/location/state/nc/city/cary` then the map would
be:

```go
{
    "services" : true,
    "location" : true,
    "state" : true,
    "stateName" : "nc",
    "city" : true,
    "cityName" : "cary",
}
```

But, if the url provided only had `services/location/state/nc` then resulting map
would be:

```go
{
    "services" : true,
    "location" : true,
    "state" : true,
    "stateName" : "nc",
    "city" : false,
    "cityName" : "",
}
```

If the url did not include the state name field, that would be blank, which could
tell the service that a GET on this URL was meant to return a list of the state
values stored, as opposed to information about a specific state.

---

### sync Package<a name="sync"></a>

The `sync` package provides access to low-level primitive operations used to
synchronize operations between different go routine threads that might be
running concurrently.

#### sync.Mutex

This is a type provided by the `sync` package, used to perform simple mutual
exclusion operations to control access to resources. The mutex can be locked
by a user, in which case any other thread's attempt to lock the item will
result in that thread waiting until the mutex is unlocked by the first owner.
Consider the following code:

```go
var counter int

func worker(id int) {
    counter = counter + 1
    myCount := counter

    fmt.Printf("thread %d, counter %d\n", id, myCount)
}

func main() int {
    workers := 5

    for i := 0 ; i < workers; i = i + 1 {
        go worker(i)
    }

    sec := time.ParseDuration("1s")
    time.Sleep(sec)

    return 0
}
```

As written above, the code will launch five go routines that will all do the same
simple operation -- increment the counter and then print its value at the time the
go routine ran. We know that go routines run in unpredictable order, but even if we
saw the numbers printed out of order, we would still see the counter values increment
as 1, 2, 3, 4, and 5.

But, because the go routines are running simultaneously, between the time one routine
gets the value of counter, adds one to it, and puts it back, another routine could have
performed the same operation. This means we would over-write the value from the other
thread. In this case, the output of the count value might be more like this:

```text
    thread 0, counter 1
    thread 4, counter 1
    thread 2, counter 2
    thread 3, counter 2
    thread 1, counter 3
```

To fix this, we use a mutex value to block access to the counter for each
thread, so they are forced to take turns incrementing the counter.

```go
var counter int
var mutex sync.Mutex

func worker(id int) {
    mutex.Lock()

    counter = counter + 1
    myCount := counter
    
    mutex.Unlock()
    fmt.Printf("thread %d, counter %d\n", id, myCount)
}

func main() int {
    workers := 5

    for i := 0 ; i < workers; i = i + 1 {
        go worker(i)
    }

    sec := time.ParseDuration("1s")
    time.Sleep(sec)

    return 0
}
```

Now that there is a mutex protecting access to the counter, no matter what order the
go routines run, the increment of the count value will always be sequential, resulting
in output that might look like this:

```text
    thread 4, counter 1
    thread 0, counter 2
    thread 2, counter 3
    thread 3, counter 4
    thread 1, counter 5
```

`Mutex` also supports `TryLock()`, which attempts to acquire the lock without
blocking: it returns `true` if the lock was acquired (in which case the
caller is now responsible for calling `Unlock()`), or `false` immediately if
some other goroutine already holds it.

```go
if mutex.TryLock() {
    // lock acquired -- do the work, then release it
    counter = counter + 1
    mutex.Unlock()
} else {
    // someone else has it right now; do something else instead of waiting
    fmt.Println("busy, skipping this round")
}
```

Calling `Unlock()` on a `Mutex` that is not currently locked (for example, a
duplicate `Unlock()` call) is a catchable Ego error rather than a program
crash, unlike real Go, where the equivalent mistake is an unrecoverable
fatal error that cannot be caught with `recover()`:

```go
try {
    mutex.Unlock()
} catch(e) {
    fmt.Println(e)   // e.Is(errors.New("mutex.not.locked")) is true
}
```

#### sync.RWMutex

`RWMutex` is a reader/writer mutual-exclusion lock: any number of readers can
hold the lock at the same time via `RLock()`/`RUnlock()`, but a writer
holding it via `Lock()`/`Unlock()` has it exclusively -- no readers and no
other writer can hold it at the same time. This is useful when a resource is
read far more often than it is written, since concurrent readers don't need
to wait on each other the way they would with a plain `Mutex`.

```go
import "sync"

var data map[string]int
var mu sync.RWMutex

func read(key string) (int, bool) {
    mu.RLock()
    defer mu.RUnlock()

    v, ok := data[key]

    return v, ok
}

func write(key string, value int) {
    mu.Lock()
    defer mu.Unlock()

    data[key] = value
}
```

`RWMutex` also supports non-blocking acquisition, matching Go's own API
exactly: `TryLock()` attempts to acquire the exclusive write lock, and
`TryRLock()` attempts to acquire a read lock; both return `true` on success
without blocking, or `false` immediately if the lock isn't currently
available.

```go
if mu.TryRLock() {
    v := data["key"]
    mu.RUnlock()
    fmt.Println(v)
}
```

As with `Mutex`, calling `Unlock()` without a currently-held write lock, or
`RUnlock()` without a currently-held read lock, is a catchable Ego error
(`e.Is(errors.New("mutex.not.locked"))`) rather than the unrecoverable fatal
error real Go produces for the same mistake.

#### sync.WaitGroup

This is a type provided by the `sync` package. You can declare a variable of this
type and a WaitGroup is created, and can be stored in a variable. This value is
used as a _counting semaphore_ and usually supports arbitrary numbers of go
routine starts and completions.

Consider this example code:

```go
func thread(id int, wg *sync.WaitGroup) {               // [1]
    fmt.Printf("Thread %d\n", id)
    wg.Done()                                           // [2]
}

func main() int {
    var wg sync.WaitGroup                               // [3]
    
    count := 5
    for i := 1; i <= count; i = i + 1 {
        wg.Add(1)                                       // [4]
        go thread(i, &wg)                               // [5]
    }
    
    wg.Wait()                                           // [6]
    
    fmt.Println("all done")
}
```

This program launches five instances of a go routine thread, and waits for them
to complete. This simplified example does not return a value from the threads, so
channels are not used. However, the caller must know when all the go routines have
completed to know it is safe to exit this function. Note that if the code did not
include the `wg.Wait()` call then the `main` function would exit before most of the
go routines had a chance to complete.

Here's a breakdown of important steps in this example:

1. In this declaration of the function used as the go routine, a parameter is
   passed that is a _pointer_ to the WaitGroup variable. This is important
   because operations on the `WaitGroup` variable must be done on the same
   instance of that variable.

2. The `Done()` call is made by the go routine when it has completed all its
   operations. To ensure that it is always executed whenever the function exits.
   this could also be implemented as `defer wg.Done()`

3. This declares the instance of the `WaitGroup` variable that will be used
    for this example. There is no initialization needed; the variable instance
    starts as a _zero state_ value.

4. Before launching a go routine, add 1 to the `WaitGroup` value (this can actually
    be a number other than 1, but must correlate exactly to the number of `Done()`
    calls that will be made to indicate completion of the task). It is essential
    that this call be made _before_ the `go` statement to ensure that the go routine
    does not complete before the `Add()` call can be made.

5. Note that the `WaitGroup` variable must be passed by address. By default, _Ego_
   passes all parameters by value (that is, a copy of the value is passed to the
   function). But because the functions must operate on the exact same instance
   of a `WaitGroup` variable, we must pass the address of the value allocated.
   Note that it is important that this value not go out of scope before the
   `Wait()` call can be made.

6. The `Wait()` call essentially waits until as many `Done()` calls are made as
   were indicated by the matching `Add()` calls. Until then, the current program
   will simply wait, and then resume execution after the last `Done()` is called.

---

### tables Package<a name="tables"></a>

The `tables` package is an Ego-specific addition with no direct Go standard
library equivalent -- it helps programs build and print text tables of data.
Create a table object with a given set of column names, add rows, sort it,
set formatting options, and render it as text or JSON.

Every fallible function and method in this package returns `(value, error)`
(or, for methods that only conceptually return an error, just `error`), like
everywhere else in the runtime.

#### tables.New(columnName...)

```go
func tables.New(columnName ...string) (tables.Table, error)
```

Creates a new table object. The column names can be passed as discrete
arguments or as a single array of strings. A table needs at least one column
before you can add rows to it -- either pass column names to `New()`, or add
them afterward with `AddColumn()`/`AddColumns()`.

A column name may start and/or end with a `:` to control that column's
alignment: a leading `:` left-aligns, a trailing `:` right-aligns, both
left-and-right aligns it centered, and neither defaults to left-aligned. The
colon itself is stripped from the displayed heading.

```go
t, err := tables.New(":Identity", "Age:", "Address")
if err != nil {
    fmt.Println("could not create table:", err)
    return
}

t.AddRow( {Identity: "Tony", Age: 61, Address: "Main St"} )
t.AddRow( {Identity: "Mary", Age: 60, Address: "Elm St"} )
t.AddRow( {Identity: "Andy", Age: 61, Address: "Elm St"} )

t.Sort("Age", "Identity")
t.Format(true, false)
t.Print()
t.Close()
```

Rows can be added as a struct (field names matching the column headings) or
as positional arguments (matching the order columns were defined in) -- see
`AddRow()` below. This example sorts by `Age` then `Identity`, prints column
headings without the underline row beneath them, prints the table to the
console, and releases the table's resources.

#### The `tables.Table` structure

| Field | Type | Description |
| :---------- | :------: | :--------------------------------------------------------------- |
| `Headings` | `[]string` | Read-only; the column names, in order. Updated automatically by `AddColumn()`/`AddColumns()`. |

Its methods:

| Method | Description |
| :------- | :---------- |
| `AddColumn(heading) error` | Appends a single column to the right of the existing ones. |
| `AddColumns(heading...) error` | Appends multiple columns in one call. |
| `AddRow(value...) error` | Adds a row; see below for its two calling conventions. |
| `Sort(columnName...) error` | Sorts rows by one or more columns; see below. |
| `Format(headings, underlines) error` | Controls whether the heading row and its underline are printed. |
| `Align(columnOrIndex, alignment) error` | Sets a column's text alignment to `"left"`, `"right"`, or `"center"`. |
| `Print(format...) error` | Renders the table to the console. |
| `String(format...) (string, error)` | Renders the table and returns it as a string instead of printing it. |
| `Pagination(height, width) error` | Sets terminal dimensions for paginated output; `(0, 0)` disables pagination. |
| `Len() (int, error)` | Number of rows. |
| `Width() (int, error)` | Number of columns. |
| `Get(rowIndex, columnName) (string, error)` | A single cell's value. |
| `GetRow(rowIndex) ([]string, error)` | All of a row's values, in column order. |
| `Find(func(column...) bool) ([]int, error)` | Row indexes matching a predicate; see below. |
| `Close() error` | Releases the table's resources. |

#### AddColumn / AddColumns

```go
t, _ := tables.New()
t.AddColumn("Name")
t.AddColumn("Age")
```

This results in a table `t` with two columns, "Name" and "Age". `AddColumns`
adds several at once:

```go
t, _ := tables.New()
t.AddColumns("Name", "Age", "Size")
```

If the table already has rows, each existing row gets a blank field for the
new column.

#### AddRow(value...)

Adds one row. There are two calling conventions:

- **A single struct argument** -- each field name must match a column
  heading; the field's value becomes that column's cell.
- **One argument per column** (or a single array of that many values) --
  values are assigned to columns in the order the columns were defined.

```go
t, _ := tables.New("Name", "Age")

err := t.AddRow( {Name: "Tony", Age: 61} )   // struct form
fmt.Println(err)                             // <nil>

err = t.AddRow("Mary", 60)                    // positional form
fmt.Println(err)                              // <nil>
```

#### Sort(columnName...)

Sorts the table's rows by one or more columns. With multiple column names,
the **last** argument is the most significant sort key -- the table is
stable-sorted starting with the least-significant key first, so that the
last-applied (most significant) sort determines the final primary order.
Prefix a column name with `~` to sort that column descending instead of the
default ascending.

```go
t, _ := tables.New("Name", "Age")
t.AddRow("Mary", 60)
t.AddRow("Tony", 61)
t.AddRow("Andy", 61)

t.Sort("Name", "Age")   // primary key: Age; secondary/tie-break: Name
```

#### Format(headings, underlines) / Align(column, alignment)

`Format` controls whether `Print()`/`String()` include the heading row and
the `===`-style underline beneath it:

```go
t, _ := tables.New("Name", "Age")
t.AddRow("Tom", 51)

t.Format(false, false)   // suppress both
t.Print()                 // Tom     51

t.Format(true, false)    // headings shown, underline suppressed
t.Print()
// Name    Age
// Tom     51
```

`Align` sets one column's text alignment. The column may be named or given
as its zero-based integer index; `alignment` is `"left"`, `"right"`, or
`"center"` (case-insensitive):

```go
t.Align("Age", "right")
```

#### Print([format]) / String([format])

`Print` renders the table to the console; `String` renders it the same way
but returns the result as a string instead. Both accept an optional format:
`"text"` (the default), `"json"`, or `"indented"` (pretty-printed JSON).

```go
t, _ := tables.New("Name", "Age")
t.AddRow("Tom", 51)

s, err := t.String("text")
fmt.Println(err)
fmt.Print(s)
// Name    Age
// ====    ===
// Tom      51

j, _ := t.String("json")
fmt.Println(j)   // [{"Name":"Tom","Age":51}]
```

#### Pagination(height, width)

Sets the terminal dimensions used when folding wide tables across multiple
header blocks. `Pagination(0, 0)` (the default for a newly created table)
disables pagination entirely.

#### Len() / Width()

```go
t, _ := tables.New(":Identity", "Age:", "Address")
t.AddRow( {Identity: "Tony", Age: 61, Address: "Main St"} )
t.AddRow( {Identity: "Mary", Age: 60, Address: "Elm St"} )

n, _ := t.Len()     // 2 -- number of rows
w, _ := t.Width()   // 3 -- number of columns, same as len(t.Headings)
```

#### Find(func(column...) bool)

`Find` accepts a closure that is called once per row, with that row's values
passed as individual string arguments (one per column, in column order). It
returns the indexes of every row for which the closure returned `true`.

```go
t, _ := tables.New(":Identity", "Age:", "Address")

t.AddRow( {Identity: "Tony", Age: 65, Address: "Main St"} )
t.AddRow( {Identity: "Mary", Age: 60, Address: "Elm St"} )
t.AddRow( {Identity: "Andy", Age: 68, Address: "Elm St"} )
t.AddRow( {Identity: "Sue", Age: 53, Address: "Baker St"} )

retirees, err := t.Find(func(id string, age string, address string) bool {
    if i, err := strconv.Atoi(age); err == nil {
        return i >= 65
    }

    return false
})

fmt.Println(retirees, err)   // [0, 2] <nil>
```

The closure converts each row's `age` column from a string to an integer and
compares it to 65; `retirees` ends up holding the indexes of the rows (`0`
and `2`) where that comparison is true.

#### Get(rowIndex, columnName) / GetRow(rowIndex)

`Get` retrieves a single cell; `GetRow` retrieves an entire row as an array
of strings. Both use a zero-based row index and return the same string
values that were passed to `AddRow()`, without any formatting or alignment
applied. The column name in `Get` is case-insensitive. An invalid row index
or unknown column name returns an error rather than panicking.

```go
t, _ := tables.New("Name", "Age", "Title")
t.AddRow("Tom", 55, "CTO")
t.AddRow("Mary", 51, "President")

v, err := t.Get(1, "title")
fmt.Println(v, err)          // President <nil>

row, err := t.GetRow(0)
fmt.Println(row, err)        // ["Tom", "55", "CTO"] <nil>

_, err = t.GetRow(99)
fmt.Println(err)             // in GetRow, invalid range: 99
```

---

### time Package<a name="time"></a>

The `time` package works with time and date values and durations, mirroring Go's own
`time` package fairly closely, with two Ego-specific additions: `ParseDuration()` accepts
a `"d"` suffix for days (e.g. `"2d"`, `"1d6h30m"`), and `time.Time` has an extra
`SleepUntil()` method with no Go equivalent. Every fallible function returns
`(value, error)`, like everywhere else in the runtime.

#### Package-level functions

| Function | Description |
| :------- | :----------- |
| `Now() Time` | The current time. |
| `Parse(layout, value string) (Time, error)` | Parses `value` using a Go reference-time `layout` string. |
| `ParseAny(text string) (Time, error)` | An Ego addition: parses `text`, guessing the layout automatically. |
| `ParseDuration(text string) (Duration, error)` | Parses a duration string; accepts Go's own syntax plus an Ego `"d"` (days) suffix. |
| `Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time` | Constructs a time from individual components. |
| `Unix(sec, nsec int64) Time` | Constructs a time from a Unix timestamp. |
| `Since(t Time) Duration` | The duration elapsed since `t`. |
| `Sleep(d Duration)` | Blocks the current goroutine for `d`. |
| `FixedZone(name string, offsetSeconds int) *Location` | A `*Location` with a fixed offset from UTC. |
| `LoadLocation(name string) (*Location, error)` | Looks up an IANA time zone by name (e.g. `"America/New_York"`). |

```go
now := time.Now()
time.Sleep(time.ParseDuration("10ms"))
elapsed := time.Since(now)
fmt.Println(elapsed.Milliseconds() >= 10)   // true

d, err := time.ParseDuration("1d6h30m")
fmt.Println(d, err)      // 30h30m0s <nil>
fmt.Println(d.Hours())   // 30.5
```

`time.Month` has no named constants (no `time.January`, etc.) -- construct one with a
cast, `time.Month(n)`, using `1` for January through `12` for December. Likewise there
is no pre-built `time.UTC` location value -- use `time.FixedZone("UTC", 0)`, or the
zero-value `*Location` produced by leaving `Date()`'s last argument as `nil`:

```go
loc := time.FixedZone("EST", -5*3600)
t := time.Date(2024, time.Month(7), 4, 8, 30, 0, 0, loc)
fmt.Println(t.String())                       // 2024-07-04 08:30:00 -0500 EST
fmt.Println(t.Format("2006-01-02 15:04:05"))  // 2024-07-04 08:30:00

u := time.Unix(int64(1705315800), int64(0))
fmt.Println(u.Format("2006-01-02"))   // 2024-01-15

loc2, err := time.LoadLocation("America/New_York")
fmt.Println(loc2, err)   // America/New_York <nil>
```

#### Parsing

`Parse(layout, value)` follows Go's reference-time convention: the layout string is the
specific moment `Mon Jan 2 15:04:05 MST 2006`, written using the actual field values
that moment would have, and any character that isn't part of a recognized field
(slashes, commas, etc.) must appear literally in `value` in the same position.

```go
t, err := time.Parse("1/2/2006 15:04", "12/7/1960 15:30")
fmt.Println(t.String(), err)   // 1960-12-07 15:30:00 +0000 UTC <nil>

_, err = time.Parse("1/2/2006", "not a date")
fmt.Println(err != nil)   // true
```

`ParseAny(text)` is an Ego addition that guesses the layout for you -- convenient when
the input format isn't fixed or known in advance:

```go
t, err := time.ParseAny("December 7, 1959 10:35am EST")
fmt.Println(t.String(), err)   // 1959-12-07 10:35:00 -0500 EST <nil>

t2, err := time.ParseAny("2024-01-15")
fmt.Println(t2.Hour(), err)    // 0 <nil>
```

`ParseDuration(text)` accepts Go's own syntax (`"h"`, `"m"`, `"s"`, `"ms"`, `"us"`/`"µs"`,
`"ns"`, fractional values, combinations like `"1h30m"`) plus an Ego-only `"d"` suffix for
days (`1d` = 24 hours); units may be separated by spaces:

```go
d, err := time.ParseDuration("2d")
fmt.Println(d.Hours(), err)   // 48 <nil>

d2, err := time.ParseDuration("1d 6h 30m")
fmt.Println(d2.Hours(), err)  // 30.5 <nil>
```

#### The `time.Time` structure

| Method | Description |
| :------- | :---------- |
| `Add(d Duration) Time` | `t` shifted by `d`. |
| `Sub(u Time) Duration` | The duration between `t` and `u`. |
| `Before(u Time) bool` / `After(u Time) bool` / `Equal(u Time) bool` | Comparisons. |
| `Format(layout string) string` | Formats `t` using a Go reference-time layout string. |
| `String() string` | Formats `t` using Go's default layout (`2006-01-02 15:04:05.999999999 -0700 MST`, with trailing zero fractional digits omitted). |
| `Hour() int` | The hour, in `[0, 23]`. |
| `Clock() (hour, min, sec int)` | Hour, minute, and second together. |
| `Month() Month` | The month, as a `time.Month` value (its own `String()` gives the English name). |
| `SleepUntil() error` | An Ego addition: blocks until `t` is reached; a no-op (not an error) if `t` is already in the past. |

```go
t1 := time.ParseAny("Dec 1, 1960")
t2 := time.ParseAny("Dec 2, 1960")

fmt.Println(t1.Before(t2), t2.After(t1), t1.Equal(t1))   // true true true

diff := t2.Sub(t1)
fmt.Println(diff.Hours())   // 24

d, _ := time.ParseDuration("1h30m")
t3 := t1.Add(d)
fmt.Println(t3.Sub(t1).Hours())   // 1.5

fmt.Println(t1.Hour(), t1.Month().String())   // 0 December
h, m, s := t2.Clock()
fmt.Println(h, m, s)   // 0 0 0
```

`SleepUntil()` blocks the calling goroutine until the receiver's time arrives:

```go
target := time.Now().Add(time.ParseDuration("300ms"))
err := target.SleepUntil()   // returns ~300ms later
fmt.Println(err)             // <nil>

past := time.Now().Add(-time.ParseDuration("300ms"))
err = past.SleepUntil()      // returns immediately -- no-op, not an error
fmt.Println(err)             // <nil>
```

#### The `time.Duration` structure

| Method | Description |
| :------- | :---------- |
| `String([extended bool]) string` | Formats the duration. See below for the `extended` argument. |
| `Hours() float64` / `Minutes() float64` / `Seconds() float64` | The duration expressed in that unit. |
| `Milliseconds() int64` / `Microseconds() int64` / `Nanoseconds() int64` | The duration expressed as an integer count of that unit. |

`String()` with no argument (or `false`) matches Go's own compact format
(`"1h30m0s"`). Passing `true` selects an Ego-only extended format: units separated by
spaces, with durations of 24 hours or more broken into days. Durations under one second
always use the compact format regardless of `extended`:

```go
d, _ := time.ParseDuration("772h35m12s")
fmt.Println(d.String())       // 772h35m12s
fmt.Println(d.String(true))   // 32d 4h 35m 12s

d2, _ := time.ParseDuration("300ms")
fmt.Println(d2.String(true))  // 300ms -- sub-second always uses the compact format

fmt.Println(d.Hours(), d.Milliseconds())
```

#### Duration and layout constants

`time.Nanosecond`, `Microsecond`, `Millisecond`, `Second`, `Minute`, and `Hour` are
plain integer constants (each is that many nanoseconds), not `Duration`-typed values --
cast the result to `time.Duration` to use `Duration`'s methods:

```go
d := time.Duration(90 * time.Minute)
fmt.Println(d.String())   // 1h30m0s
```

`time.ANSIC`, `UnixDate`, `RubyDate`, `RFC822`, `RFC822Z`, `RFC850`, `RFC1123`,
`RFC1123Z`, `RFC3339`, `RFC3339Nano`, `Kitchen`, `Stamp`, `StampMilli`, `StampMicro`, and
`StampNano` are ready-made layout strings for `Format()`/`Parse()`, matching Go's own
constants of the same names:

```go
t := time.ParseAny("December 7, 1959")
fmt.Println(t.Format(time.RFC1123))   // Mon, 07 Dec 1959 00:00:00 UTC
```

---

### util Package<a name="util"></a>

The `util` package contains miscellaneous utility functions that may be convenient
for developers writing _Ego_ programs. It is an Ego-specific package with no
corresponding package in Go; every fallible function returns `(value, error)`.

| Function | Description |
| :------- | :---------- |
| `SetLogger(name string, active bool) (bool, error)` | Enables or disables a logger; returns its previous state. |
| `Memory() MemoryStatus` | Current and lifetime memory consumption of the running program. |
| `Mode() string` | The execution mode the program is running under. |
| `Symbols([scope int [, format string [, allSymbols bool]]]) string` | A formatted report of the active symbol table chain. |
| `SymbolTables() []SymbolTable` | Metadata (name, depth, size, ...) about each symbol table in the active scope chain. |
| `Package(name string) (map[string]map[string]string, error)` | Lists the types, constants, variables, and functions exported by a package. |
| `Packages() []string` | The names of every package known to the running program. |
| `Log(count int [, session int]) ([]string, error)` | The last `count` lines of the log buffer, optionally filtered to one session. |

#### util.SetLogger()

`SetLogger()` enables or disables a named logger at runtime, returning the logger's
previous enabled state. This can be used to turn on tracing while running interactively,
for example. Logger names are not case sensitive; an unrecognized name is an error.

```go
old, err := util.SetLogger("trace", true)
fmt.Println(old, err)   // false <nil>  (assuming it started disabled)

_, err = util.SetLogger("no-such-logger", true)
fmt.Println(err != nil)   // true
```

#### util.Memory()

`Memory()` returns a `MemoryStatus` structure summarizing current memory consumption,
total consumption for the life of the program, system memory used on behalf of the
_Ego_ process, and a count of how many times the garbage collector has run.

```go
fmt.Println(util.Memory())
// MemoryStatus{ Current: 36.22, GC: 4, System: 114.32, Time: "Mon Jul 13 2026 17:53:24 EDT", Total: 102.25 }
```

`Current` and `System` are expressed in megabytes: `Current` is memory currently
allocated by the Ego process, and `System` is memory obtained from the operating
system on the process's behalf (always somewhat larger than `Current`, since it
includes reserved-but-unused space). `Total` is the cumulative amount ever allocated
over the life of the program -- it only grows, unlike `Current`, which drops each
time the garbage collector reclaims unused memory. `GC` counts how many garbage
collection cycles have completed so far.

#### util.Mode()

`Mode()` reports the mode the current program is running under:

&nbsp;

| Mode | Description |
| :---------- | :------------------------------------------------------------------------ |
| interactive | The `ego` program was run with no program name, accepting console input |
| server | The program is running under control of an _Ego_ rest server as a service |
| test | The program is running using the `ego test` integration test command |
| run | The program is running using `ego run` with an input file or pipe |

&nbsp;

```go
fmt.Println(util.Mode())   // run
```

#### util.Symbols([scope [, format [, allSymbols]]])

`Symbols()` generates a report on the current state of the active symbol table chain,
starting with the scope the call itself runs in (scope 0, which -- since it was created
just for this call -- is always empty) and continuing outward through every enclosing
scope: statement blocks, functions, imported-package tables, and finally the root table.
All three arguments are optional:

- `scope`: if given (`>= 0`), restricts the report to just that one scope level instead
  of the full chain. Requesting a scope level beyond the root table is an error.
- `format`: `"json"` or `"indented"` selects JSON output (the latter pretty-printed)
  instead of the default human-readable table.
- `allSymbols`: when `true`, also includes built-in functions and built-in packages,
  which are omitted by default.

```go
x := 5
y := "hello"
fmt.Println(util.Symbols())
```

```text
Scope    Table                 Boundary    Symbol    Type              Readonly    Value
=====    ==================    ========    ======    ==============    ========    ==================
  0       builtin ... Symbols  false       <no symbols>
  1      *file <stdin>         false       fmt       *interface{}      false       Package<"fmt", ...>
                                            ...
                                            x         int               false       5
                                            y         string            false       "hello"
  2      *root                 true        _version  string            true        "1.8-1896"
                                            ...
```

Requesting just the caller's own scope (level 1 here), as indented JSON:

```go
fmt.Println(util.Symbols(1, "indented", false))
```

```text
[
   ...
   {
      "Symbol":"x",
      "Type":"int",
      "Readonly":false,
      "Value":5
   },
   {
      "Symbol":"y",
      "Type":"string",
      "Readonly":false,
      "Value":"\"hello\""
   }
]
```

#### util.SymbolTables()

`SymbolTables()` returns an array of `SymbolTable` structures, one per symbol table in
the active scope chain, giving each table's name, nesting depth, unique id, size, and
whether it is the root table or a shared table:

```go
tbls := util.SymbolTables()
fmt.Println(len(tbls) > 0)   // true
fmt.Println(tbls[0].name)    // file <stdin>
```

#### util.Package(name)

`Package(name)` looks up a package by name (or import path) and returns a map
describing every exported type, constant, variable, and function it defines. The
result map has up to four keys -- `"types"`, `"constants"`, `"variables"`, and
`"functions"` -- each present only if the package defines at least one item of that
kind; each of those, in turn, maps an item's name to a human-readable description of
its declaration or value.

```go
m, err := util.Package("math")
fmt.Println(err)                       // <nil>
fmt.Println(m["functions"]["Sqrt"])     // Sqrt(f float64) float64
fmt.Println(m["constants"]["Pi"])       // 3.141592654

_, err = util.Package("no-such-package")
fmt.Println(err)   // invalid or missing package name: no-such-package
```

#### util.Packages()

`Packages()` returns a sorted array of the names of every package known to the
running program (built-in packages plus anything imported):

```go
names := util.Packages()
fmt.Println(len(names) >= 20)   // true
```

#### util.Log(count [, session])

`Log(count)` returns the last `count` lines from the log buffer as a string array. The
optional `session` argument restricts the result to lines logged during a specific
session id; pass `0` (or omit it) for lines from all sessions. If the log buffer is
empty (or logging isn't configured to retain lines), an empty array is returned rather
than an error.

```go
lines, err := util.Log(10)
fmt.Println(err)             // <nil>
fmt.Println(len(lines) >= 0) // true
```

---

### uuid Package<a name="uuid"></a>

The `uuid` package provides support for universal unique identifiers, based on
[github.com/google/uuid](https://pkg.go.dev/github.com/google/uuid). This is an
industry-standard way of creating an identifier that is (for all practical purposes)
guaranteed to be unique, even among different instances of `ego` running on different
computers. Ego's package covers `New`, `Nil`, and `Parse`, plus the `UUID` type and its
`String`/`Gibberish` methods; it does not include the Go package's `Must*` variants
(`MustParse`, etc.) that panic instead of returning an error.

A `uuid.UUID` value is its own distinct type, not a string -- printing one (`fmt.Println`)
or comparing it with `==` works directly, but using it as a string (concatenation, passing
to a function expecting a string) requires an explicit call to `.String()`. Its canonical
text form is the standard 8-4-4-4-12 group of lowercase hexadecimal digits separated by
hyphens, e.g. `"af315ffd-6c57-46b9-af62-4aac8ba5a212"`.

| Function | Description |
| :------- | :---------- |
| `New() UUID` | Generates a new, randomly-generated unique identifier. |
| `Nil() UUID` | The all-zeros UUID; never produced by `New()`. |
| `Parse(text string) (UUID, error)` | Parses and validates a UUID string. |

| Method | Description |
| :------- | :---------- |
| `String() string` | The canonical hyphenated hexadecimal text form. |
| `Gibberish() string` | A lower-case alphanumeric encoding of the UUID, omitting `o`, `l`, and `i` to avoid confusion with `0` and `1`. |

#### uuid.New()

`New()` generates a new, randomly-generated `UUID` value:

```go
id := uuid.New()
fmt.Println(id)            // af315ffd-6c57-46b9-af62-4aac8ba5a212  (a new value each run)
fmt.Println(id.String())   // same text, via an explicit call
```

#### uuid.Nil()

`Nil()` returns the _zero value_ for a UUID -- one that consists entirely of zeros. This
value is never produced by `New()` and never matches another UUID's value. It's also the
zero value of a `var`-declared `UUID`, so the two are interchangeable:

```go
var x uuid.UUID
fmt.Println(x == uuid.Nil())         // true
fmt.Println(x.String())              // 00000000-0000-0000-0000-000000000000

id := uuid.New()
if id == uuid.Nil() {
    fmt.Println("id value was not set")
}
```

#### uuid.Parse(text)

`Parse()` parses and validates a UUID string value -- useful for string values received
via REST API calls, etc. -- returning the parsed `UUID` and an error indicating whether
the parse succeeded.

```go
id, err := uuid.Parse("af315ffd-6c57-46b9-af62-4aac8ba5a212")
fmt.Println(id, err)   // af315ffd-6c57-46b9-af62-4aac8ba5a212 <nil>

_, err = uuid.Parse("not-a-uuid")
fmt.Println(err)       // invalid UUID length: 10
```

If `text` is a valid UUID specification, `id` holds the parsed value (case-normalized)
and `err` is `nil`. If `text` is invalid, `id` is `nil` and `err` describes the failure.
Round-tripping through `String()`/`Parse()` always reproduces an equal value:

```go
u1 := uuid.New()
u2, _ := uuid.Parse(u1.String())
fmt.Println(u1 == u2)   // true
```

#### uuid.UUID.Gibberish()

`Gibberish()` is an Ego-specific extension with no direct equivalent in the Go package: it
renders the UUID as a lower-case alphanumeric string that omits the letters `o`, `l`, and
`i` (to avoid confusion with the digits `0` and `1`) -- useful for generating identifiers
meant to be read aloud or typed by hand:

```go
id, _ := uuid.Parse("ab34d542-a437-408a-b0ca-38ea5d78696f")
fmt.Println(id.Gibberish())   // rm4szqj72tubkesqdukixgpyk
```

&nbsp;
&nbsp;

## User Packages

You can create your own packages which contain type definitions and
functions that are used via the package prefix you specify. Consider
the following example files.

The first file is "employee.ego" and describes a package. It starts
with a `package` statement as the first statement item, and then
defines a type and a function that accepts a value of that type as
the function receiver.

```go
package employee

type Employee struct {
    id int
    name string
}

func (e *Employee) SetName( s string ) {
    e.name = s
}
```

The second file is "test.ego" and is the program that will use this package.
It starts with an `import` statement, which causes the compilation to include
the package definition within the named file "employee". You can specify the
file extension of ".ego" but it is not necessary.

```go
import "employee"

e := employee.Employee{id:55}

e.SetName("Frodo")

fmt.Println("Value is ", e)
```

This program uses the package definitions. It creates an instance of an
`Employee` from the `employee` package, and initializes one of the fields
of the type. It then uses the object to invoke a function for that type,
the `SetName` package. Note that when this is called, you do not specify
the package name; instead you specify the object that was created using
the package's definition. In this example, it should print the structure
contents showing the `id` of 55 and the `name` of "Frodo."

### package \<name\>

Use the `package` statement to define a set of related functions in
a package in the current source file. A give source file can only
contain one package statement and it must be the first statement.

```go
package factor
```

This defines that all the functions and constants in this module will
be defined in the `factor` package, and must be referenced with the
`factor` prefix, as in

```go
y := factor.IntFact(55)
```

This calls the function `IntFact()` defined in the `factor` package.

### init() Function

A package can define a function named `init` that takes no arguments and
returns no values. If present, it is run automatically, exactly once, the
first time the package is imported — you never call it yourself. This
mirrors Go's package `init()` behavior, and is useful for one-time setup
such as populating a lookup table or setting a package variable to a
computed default.

```go
package factor

var table map[string]int

func init() {
    table = map[string]int{}
    table["one"] = 1
    table["two"] = 2
}
```

Any program that does `import "factor"` will have `init()` run before the
`import` statement completes, so `table` is guaranteed to already be
populated the first time any code references it.

A few things to keep in mind:

- `init()` runs only the first time the package is imported anywhere in the
  running program. If the same package is imported again — by another file,
  under an alias, or from a different part of the program — the cached
  package is reused and `init()` does not run again.
- Because `init` is a lowercase (unexported) name, it cannot be called
  directly by other code, even with the package prefix (`factor.init()` is
  not a valid reference). It exists solely as an automatic one-time
  initializer.
- If `init()` itself generates a runtime error (for example, a division by
  zero), the `import` statement that triggered it fails with that error.
- A package is not required to have an `init()` function; most packages
  won't need one.

## Directives <a name="directives"></a>

Directives are special _Ego_ statements that perform special functions
outside the normal language syntax, often to influence the runtime
environment of the program or give instructions to the compiler itself.

### @capture \<var\> := | = { code } <a name="at-capture"></a>

The `@capture` directive redirects everything a block of code prints (via
`fmt.Println`, `fmt.Printf`, `fmt.Print`, or the `print` language extension)
into a string variable, instead of letting it appear on the console. This
is mainly useful inside `@test`/`@assert` blocks, so a test can check
exactly what a piece of code prints, not just what it returns — and so
routine test output doesn't clutter the console while `ego test` is running.

```go
@capture output := {
    fmt.Println("Hello")
    fmt.Printf("%d\n", 53)
}
fmt.Println(output)
```

Running this prints `Hello` and `53` exactly once, because the two prints
inside the `@capture` block are captured into `output` rather than shown
directly; the final `fmt.Println(output)` is what actually displays them.
After the block finishes, `output` holds the string `"Hello\n53\n"`.

#### `:=` versus `=`

Like an ordinary Ego assignment, `@capture` supports two forms:

| Form | Meaning |
| ---- | ------- |
| `@capture x := { ... }` | Declares a brand new variable `x` in the current scope, the same as an ordinary `x := 5`. It is an error to use `:=` if `x` already exists in the current scope. |
| `@capture x = { ... }` | Assigns to an existing variable `x`. Unlike ordinary `x = 5` (which only fails at _runtime_ if `x` was never declared), `@capture x = { ... }` checks at _compile time_ that `x` already exists, giving a clearer error for a typo'd variable name. |

#### Discarding the output with `_`

Sometimes a block is only wrapped in `@capture` to keep it from cluttering the
console, and nothing needs to inspect the printed text itself — for example, a
test that already checks `fmt.Printf`'s returned byte count and error value has
no further use for the text. Use Ego's ordinary "discard" name in place of a
real variable:

```go
@capture _ := {
    n, err := fmt.Printf("hello %d\n", 42)
    // n == 9, err == nil
}
```

`_` can be used with either `:=` or `=`, exactly as with an ordinary
assignment — both behave identically, and (unlike a real variable) `_` can be
reused as many times as needed in the same scope without an "already
declared" error, since Ego never actually declares or stores anything named
`_` anywhere in the language. Discarding on success doesn't mean "never show
me this," though: if the block raises an error, the captured text is still
printed to the console with the usual `"@capture _:"` heading — only the
success-path bookkeeping is skipped.

#### Error handling

A `@capture` block behaves like an implicit `try` around its statements. If
an error occurs partway through the block:

- Capturing is always turned off cleanly before the error is allowed to
  continue — later code (including the rest of an enclosing `@test` body)
  is never left accidentally writing into an abandoned buffer.
- `<var>` still receives whatever text _was_ captured before the error
  occurred, so a partial result is never silently discarded.
- The captured text is printed to the console (or, if running inside
  `ego test`, into that test's own output) with a heading naming the
  variable, e.g.:

  ```text
  @capture output:
  Hello
  ```

  so that if a test fails or a program crashes unexpectedly, whatever the
  block had already printed remains visible instead of vanishing along with
  the error.
- The error itself is then re-raised (rethrown), exactly as if the code
  inside the block had never been wrapped in `@capture` at all. A real,
  user-written `try`/`catch` around the `@capture` block (or further out)
  can still catch it normally; if nothing catches it, it terminates the
  program the same way any other uncaught error would.

```go
var output string
caught := false

try {
    @capture output = {
        fmt.Println("before the error")
        _ = 5 / 0
    }
} catch(e) {
    caught = true
    // output == "before the error\n" -- the partial text is still there.
}
```

**Known limitation:** `@fail` and an unrecovered `panic()` both intentionally
bypass `try`/`catch` entirely elsewhere in the language (there is no way to
`catch` either one — by design, both are meant to be unconditionally fatal).
Since `@capture`'s error handling is built on the same `try`/`catch`
mechanism, it inherits this limitation: if `@fail` runs, or a `panic()`
inside the block is never recovered, the cleanup/flush behavior described
above does not run, and whatever had been captured so far is lost. In
practice this is rarely noticeable, because the whole program is already
terminating in both of those cases.

#### Implementation note

`@capture` does not introduce a new output mechanism — it reuses the same
`io.Writer`-based redirection that both `fmt.Println` and the `print`
language extension already write through, temporarily swapping in a
string buffer and then restoring exactly what was there before (which
could be the real console, an enclosing `@capture` block's own buffer, or
`ego test`'s own per-test output-gathering buffer). This is why `@capture`
blocks can be nested, and can be used freely inside `@test` blocks without
interfering with `ego test`'s own reporting of each test's output.

### @compile [block] { code } <a name="at=compile"></a>

The `@compile` directive allows a test program to trap compiler errors in
a block of code, and evaluate the compilation error. The `@compile` directive
acts like a `try`/`catch` block.

```go
@compile block {
    x = bob
} catch(e) {
    fmt.Println("Compile error, ", e)
}
```

In this example, the text in the `@compile` braces will be compiled, but if
there is a compile error, it signals that error which can be caught as the
variable `e`. This could contain an error if `bbo` is not a known symbol,
for example. If `bob` does exist, then the compilation may not have an error,
and the code will run as if it was compiled inline.

If you omit the `catch` statement, if the compilation generated an error,
that error is signalled instead.

```go
@compile blocok {
    pring "Hello"
}
```

When executed, this will generate an error indicating that `pring` is not
a valid verb. The catch block is required to prevent this from stopping
the program. You can generate an empty catch block if your intent is to test
invalid Ego code without stopping teh program.

```go
@compile blocok {
    pring "Hello"
} catch {}
```

This use of `@compile` will silently fail and exexcution continues. In all
the above examples, the keyword `block` is one of the options that can be
set on the compilation unit, that do not affect the rest of the program's
compilation.

| Keyword | Values | Description |
| ------- | ------ | ----------- |
| block | | If present, the code in braces is a statement block. Otherwise it must be a full program. There is no option value, the option is true if `block` is present. |
| disasm | | If present, prints a disassembly of the bytecode generated for this block once it compiles successfully. There is no option value, the option is true if present. See below. |
| eof | "marker" | Use a text marker instead of `{ }` braces to delimit the code to compile. See below. |
| optimize | 0\|1\|2 | Specify the optimization level for this compilation. |
| typeshadowing | true\|false\| | If true, builtin type names can be used as var names. IF false, generates an Error. Go-compatability requires using true. |
| unknown | true\|false | If true, unknown symbol references generate a compile error. If false, the unknown symbol will only cause an error if the compiled function code is executed. |
| unused | true\|false | If true, unused variables generate a compile error. If false, no error is reported. |

If the keyword `block` is not present, then the code between the braces must
be a complete program. That is, it can support global type specifications,
function declarations, etc. In fact, most statements like an assignment
operation can only be used inside a function definition.

If `unknown` or `unused` are set, they override the default setting for the
Ego profile setting. This allows a compilation test to validate the operation
of the compiler feature irrespective of the user's default configuration value,
which is not changed by this operation.

Similarly, the `optimize` setting allows the test to override the compiler
optimization setting for just this block of code.

This extension is used mostly in writing unit tests for the compiler
itself.

#### Viewing generated bytecode with `disasm` (or `bytecode`)

While debugging the compiler itself, or a test that isn't producing the
result you expect, it can help to see the actual bytecode instructions
generated for a block of code. Adding the `disasm` keyword (or its alias,
`bytecode`) to `@compile` prints a disassembly listing of the block once it
finishes compiling successfully:

```go
@compile block disasm {
    x := 1
    fmt.Println(x)
}
```

This produces output similar to the following (each line is actually
prefixed with a timestamp, sequence number, and the `BYTECODE:` logger tag,
omitted below for readability):

```text
*** Disassembly of @compile
   0: AtLine       0
   1: Push         Marker<let>
   2: Push         ^1
   3: SymbolCreate "x"
   4: Store        "x"
   5: DropToMarker Marker<let>
   6: AtLine       8
   7: Push         Marker<call>
   8: Load         "fmt"
   9: SetThis
  10: Member       "Println"
  11: Load         "x"
  12: Call         1
  13: DropToMarker Marker<call>
Disassembled 14 instructions
```

This is independent of the `--log bytecode` command-line logging option: the
`bytecode`/`disasm` keyword is a one-off request for this specific
`@compile` block, and does not turn on bytecode logging for the rest of the
program (nor is it affected by whether `--log bytecode` was separately
specified on the command line).

#### Delimiting the code with `eof="marker"` instead of braces

The `{ code }` form above finds the end of the code to compile by counting
`{` and `}` tokens as it reads them, stopping once the count returns to
where it started. That works well as long as the code being tested has
correctly balanced braces — but it makes it awkward to write a test whose
whole point is that the braces are **not** balanced, such as checking that
the compiler reports a sensible error for a missing `}`. A stray or missing
brace inside the test code throws off the count, so the `@compile` directive
can grab the wrong tokens, or leave tokens behind that confuse the
_enclosing_ program with an unrelated error.

The `eof="marker"` option avoids this entirely by delimiting the code with a
plain text marker instead of braces:

```go
@compile eof="$EOF"
    fmt.Println(1,,2)
$EOF
catch(e) {
    fmt.Println("Compile error, ", e)
}
```

Here, there are no braces around the code at all. Instead, the compiler
scans the token stream that follows the `@compile eof="$EOF"` line, gluing
together the spelling of each token it reads, until that concatenated text
exactly matches the marker string (`$EOF` in this example). Everything read
before the marker is the code to compile; the marker's own tokens are
discarded. Because braces are never treated specially in this mode, code
with intentionally mismatched braces can be tested cleanly:

```go
@compile block eof="$EOF"
    if true {
        fmt.Println("this block is never closed")
$EOF
catch(e) {
    // e.Code() is "block.end" - "missing '}'"
}
```

The marker string can be written as a single token (for example `"END"`) or
split across several tokens by the tokenizer (for example `"$EOF"`, which is
actually read as the two tokens `$` and `EOF`) — the comparison is always
done against the _concatenated spelling_ of the tokens read so far, not
against a single token, so you can pick whatever marker text is convenient
and unlikely to appear naturally in the code you are testing. The marker
value must be a non-empty quoted string.

The `eof=` option can be freely combined with `block`, `unused`, `unknown`,
and `optimize`, exactly as they combine with the brace-delimited form:

```go
@compile block unused=false eof="###"
    func scratch() {
        neverReferenced := 1
    }
###
catch(e) {
    fmt.Println("Compile error, ", e)
}
```

`eof=` is an addition to `@compile`, not a replacement — the `{ code }`
brace-delimited form shown at the top of this section continues to work
exactly as it always has.

### @extensions true|false|default

Language extensions are off by default when you first start running
_Ego_. You can override the default value by setting it in the configuration
using the command line:

```sh
    ego config set ego.compiler.extensions=true
```

When extensions are enabled, additional language features are available.
These include:

- The `print` statement as a shorter form of `fmt.Println()`
- The `panic` statement to signal a runtime error
- The `try` and `catch` statements for error catching
- The `throw` statement to raise an already-constructed, non-nil error value
  as a catchable runtime error
- The optional operator `? value : ifErrorValue`
- Use of `len()` with any data type
- Addition of the `index()` function for searching any data type
- Support for variable-length argument lists in functions (as distinct
  from functions with variadic `...` argument lists)

You can also temporarily set this value within any function by using
the `@extensions` directive, followed by one of `true`, `false`, or
`default`. The `default` value sets the setting back to whatever it
is in the default configuration.

Note that if the directive is used within a function, it only remains
in effect for that function. If the directive is used at the start
of a source file, it remains in effect for the entire source file.

### @global

You can store a value in the Root symbol table (the table that is the
ultimate parent of all other symbols). You cannot modify an existing
readonly value, but you can create new readonly values, or values that
can be changed by the user.

```go
@global base "http://localhost:8080"
```

This creates a variable named `base` that is in the root symbol table,
with the value of the given expression. If you do not specify an expression,
the variable is created as an empty-string.

### @localization <a name="at-localization"></a>

The `@localization` directive defines localized string properties for any
supported language in the current Ego program. The directive stores data in
the localization properties dictionary, which can be accessed using the
i18n.T() function. The localization is defined using structure notation,
with a field for each language. Within each language is are fields for
each message property. The property name is the field name (which can be
in double quotes if it is not a valid identifier) and the value is the
localized string.

```go
@localization {
    "en": {
        "hello.msg": "hello, {{Name}}",
        "goodby.msg": "goodbye"
    },
    "fr": {
        "hello.msg": "bonjour, {{Name}}",
        "goodbye.msg": "au revoir"
    },
    "es": {
        "hello.msg": "hola, {{Name}}",
        "goodbye.msg":"adios"
    }
}

func main{
    m := i18n.T("hello.msg", {Name: "Tom"})
    
    fmt.Println(m)
}
```

There can be only one `@localization` specification in a given program.
It can appear before or after the functions in the program (it is
processed during compilation).

Use the `i18n.T()` function to get the localized string value. In the
above example, the optional second argument is used, which contains a
parameter map for each item called out in the message text. Note that the
message text is compiled and executed using the substitution rules for
Ego messages, so you can reference the named values but also specify
additional formatting rules.

An optional third argument indicates the language code ("en", "fr", "es",
etc.) to use. If omitted, the current session's language is used. In the
case of a web service, the service may wish to ascertain the caller's language
to provide language-specific web results.

#### Substitutions

Some messages do not need substitution values. That is, the message text is
complete as is. However, many other messages have additional data stored
in the message text. Consider a message whose job is to report the length of
a buffer generated. Here is an example text:

```text
Length of reply: {{length}}
```

In this example, there is a single substitution value for `length`. The
substitution operator is defined by text with double braces (`{{` and `}}`).
When calling the localization function, an additional parameter is supplied
which is a map that defines the substitution values to be put in the text.
Consider the following snippet of Go code, which presumes the length value is stored
in a variable called `numBytes`:

```go
text := i18n.T("msg.data.length", map[string]any{
    "length": numBytes,
})
```

In this case, the number of bytes is passed into the map with a key value of "length". When the
text is being formatted by the `i18n.T()` function, when the `{{length}}` text is found in the
localization, it directs the formatter to look in the map and place the value assigned to "length"
in the message. If the value of `numBytes` is 357, the message text resulting would be

```text
Length of reply: 357
```

By default, whatever value is found in the map is formatted using the default String() formatter
or default output type for that data value. The substitution text can specify a different format
or other information to control how the information is written. For example, if the length must
always be a five-digit string with leading zeros, you can specify the format in the localization
text as follows:

```text
Length of reply: {{length|%05d}}
```

In this case, additional formatting information is given in the substitution operator by putting
a "|" character followed by the additional formatting operation. There can be multiple operators
given, all separated by the "|" character. In this case, the item is considered a Go format
specification because it starts with the "%" character. The string `%05d` indicates that the value
is meant to be formatted as an integer value with five spaces and leading zeros. In this case the
resulting text would be:

```text
Length of reply: 00357
```

Below is a table of the formatting operators that can be specified. If multiple format operations
are given in a substitution operator, they are processed in order specified.

| Format Operator | Description |
| ---------------- | ------------ |
| lines | The item is an array, make a separate line for each array element |
| list | The item is an array, output each item separated by "," |
| size n | If the substitution is longer than `n` characters, truncate with `...` ellipses |
| pad "a" | Use the value to write copies of the string "a" to the output |
| left n | Left justify the value in a field n characters wide |
| right n. | Right justify the value in a field n characters wide |
| center n. | Center justify the value in a field n characters wide |
| empty "text" | If the value is zero, an empty string, or an empty array, output "text" instead |
| nonempty "Text" | If the value is non-zero, non-empty string, or non-empty array, output "Text" instead |
| zero "text" | If the value is numerically zero, output "text" instead of the value |
| one "text" | If the value is numerically one, output "text" instead of the value |
| many "text" | If the value is numerically greater than one, output "text" instead of the value |
| card "a","b" | If the value is numerically one, output "a" else output "b" |

These can be combined as needed, and a single value from the map of values can be used multiple
times in substitution operators. Consider the following message:

```text
There are {{count}} rows
```

In many languages (English included) both the verb and the noun are affected by the cardinality of
the value of count. Additionally, we might want to specify "no rows" when the count is zero. This
can all be done in the localization substitution defines. If the message was defined as:

```text
There {{count|card is,are}} {{count|empty "no"}} {{count||card row,rows}}.
```

This uses the count value to control the verb "to be", whether a numeric value or "no" for an empty
value, and the cardinality of the row noun. Note that in the example the `card` format operator
replaces the value with the string, while the `empty` format operator formats the value of `count`
normally using the default integer output, unless the value is empty/zero in which case the string
"no" will be used instead. That is, some operators affect the formatting of the value and other
operators use the value to made decisions about what to output instead of the value itself.

```text
There are no rows.   # For count of zero
There is 1 row.      # For count of 1
There are 32 rows.   # For count of 32
```

### @optimizer on|off<a name="at-optimizer"></a>

You can turn the compiler optimizer on and off during a compilation using the `@optimizer`
directive. This must be followed by keywords "on" or "off". The effect is set immediately,
when the next test/function/compilation unit completes, whether the optimizer runs or not
will be controlled by this setting.

Note that the optimizer is helpful for programs that run a lot of iterations of loops, or
have code that is called frequently. For shorter programs that run once, the optimizer can
incur significant overhead. with little-to-no-benefit.

By default, the optimizer is always disabled when running the `ego test` command.

### @template <a name="at-template"></a>

You can store away a named Go template as inline code. The template
can reference any other templates defined.

```go
@template hello "Greetings, {{.Name}}"
```

The resulting templates are available to the template() function,
 whose first parameter is the template name and the second optional
 parameter is a record containing all the named values that might
 be substituted into the template. For example,

```go
     print strings.template(hello, { Name: "Tom"})
```

This results in the string "Greetings, Tom" being printed on the
stdout console. Note that `hello` becomes a global variable in the program, and
is a pointer to the template that was previously compiled. This
global value can only be used with template functions.

### @type strict|relaxed|dynamic <a name="at-type"></a>

You can temporarily change the language settings control when type
checking is strict, relaxed, or dynamic.

The rules below cover variable **assignment**. Combining values in an
expression, passing function arguments, and returning values from a function
each follow their own related-but-not-identical rules under these same three
modes; see [Type Conversions](#typeConversion) for the complete picture.

When in strict mode, assignment follows Go's own assignability rules:

- All values in an array constant must be of the same type
- You cannot create or delete structure members
- A non-constant value of a different type than the receiving variable is always rejected —
  cast it explicitly first (e.g. `w = int(f)`)
- A literal constant (e.g. `w = 5`) may still convert implicitly, but only when doing so
  loses no information — `var f float64 = 1.0; f = 5` and `var w int = 5; w = 3` both
  succeed, but `w = 3.7` is rejected because truncating it would lose the fractional part,
  even though `3.7` is itself a constant

When in relaxed mode,

- A value is always converted before being stored to match the type of the receiving
  variable, whether it comes from a literal or a variable — the receiving variable's type
  never changes as a result of an assignment. This is effectively an automatic version of
  the explicit cast strict mode requires you to write by hand.
- An error is only raised if the value genuinely cannot be converted at all (e.g. assigning
  the string `"abc"` to an `int` variable)
- In expressions, data types will automatically be promoted to the most complex type in the expression

When in dynamic mode,

- Any value will be converted to the required type for any operation, automatically
- Storing a value of a different type into an existing variable changes that variable's
  type to match, rather than converting the value — this is the only mode where a
  variable's type can change after it is first assigned

This mode is effective only within the current statement block
(demarcated by "{" and "}" characters). When the block finishes,
type enforcement returns to the state of the previous block. This
value is controlled by the types preferences item or
command-line option.
