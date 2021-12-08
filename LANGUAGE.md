
# Table of Contents

1. [Introduction](#intro)

1. [Data Types](#datatypes)
    1. [Base Types](#basetypes)
    2. [Arrays](#arrays)
    3. [Structures](#structures)
    4. [Maps](#maps)
    5. [Pointers](#pointers)
    6. [User Types](#usertypes)

1. [Symbols and Expressions](#symbolsexpressions)
    1. [Symbols and Scope](#symbolsscope)
    1. [Constants](#const)
    1. [Operators](#operators)
    1. [Type Conversion](#typeconversion)
    1. [Builtin Functions](#builtinfunctions)

1. [Conditional and Iterative Execution](#flow-control)
    1. [If/Else Conditional](#if)
    2. [For &lt;condition&gt;](#for-conditional)
    3. [For &lt;index&gt;](#for-index)
    4. [For &lt;range&gt;](#for-range)
    5. [Break and Continue](#break-continue)

1. [User Functions](#user-functions)
    1. [The `func` Statement](#function-statement)
    2. [The `return` Statement](#return-statement)
    3. [The `defer` Statement](#defer-statement)
    4. [Function Variables](#function-variables)
    5. [Function Receivers](#function-receivers)

1. [Error Handling](#errors)
    1. [Try and Catch](#try-catch)
    2. [Signalling Errors](#signalling)

1. [Threads](#threads)
    1. [Go Routines](#goroutine)
    2. [Channels](#channels)

1. [Packages](#packages)
   1. [The `import` statement](#import)
   1. [`cipher` package](#cipher)
   1. [`db` package](#db)
   1. [`fmt` package](#fmt)
   1. [`io` package](#io)
   1. [`json` package](#json)
   1. [`math` package](#math)
   1. [`os` package](#os)
   1. [`rest` package](#rest)
   1. [`sort` package](#sort)
   1. [`strings` package](#strings)
   1. [`sync` package](#strings)
   1. [`tables` package](#tables)
   1. [`util` package](#util)
   1. [`uuid` package](#uuid)

1. [User Packages](#packages)
   1. [The `package` statement](#package)

1. [Directives](#directives)
   1. [@error](#at-error)
   2. [@global](#at-global)
   3. [@localization](#at-localization)
   3. [@template](#at-template)
   4. [@type](#at-type)

1. [Testing](#testing)
   1. [The `test` command](#at-test)
   2. [The `@test` directive](#at-test)
   3. [The `@assert` directive](#at-assert)
   4. [The `@fail` directive](#at-fail)
   5. [The `@pass` directive](#at-pass)

&nbsp;
&nbsp;

# Introduction to _Ego_ Language <a name="intro"></a>

Version 1.1

This document describes the language _Ego_, which is a scripting
language and tool set patterned off of the _Go_ programming language.
The _Ego_ language name is a portmanteaux for _Emulated Go_. The data
types and language statements are very similar to _Go_ with a few
notable exceptions:

* The _Ego_ type system is simpler than Go, and does not yet offer
  the idea of typed interfaces.
* If enabled by settings, _Ego_ offers language extensions such as
  a try/catch model for intercepting runtime errors and "optional"
  values similar to Swift.
* The language can be run with either dynamic or static typing. The
  default is dynamic, so variable type binding occurs at the moment
  of use as opposed to part of the compilation process.
* The available set of packages that support runtime functionality
  is significantly limited.

The _Ego_ language is run using the `ego` command-line interface. This
provides the ability to run a program from an external text file, to
interactively enter _Ego_ programming statements, or to run _Ego_
programs as web services. This functionality is documented elsewhere;
this guide focusses on writing _Ego_ programs regardless of the runtime
environment.

The _Ego_ language is Copyright 2020, 2021 by Tom Cole, and is freely
available for any use, public or private, including commercial software.
The only requirement is that any software that incorporates or makes use
of _Ego_ or the packages written by Tom Cole to support must include a
statement attributing authorship to _Ego_ and it's runtime environment
to Tom Cole.

&nbsp;
&nbsp;

# Data Types<a name="datatypes"></a>

The _Ego_ language supports a number of base types which express a
single value of some type (string, integer, boolean, etc.). These base
types can be members of complex types consisting of arrays (ordered
lists), maps (dynamic types key/value pairs) and structs (field-name/value
pairs). Additionally, the user can
create types based on the base or complex types, such as a type
describing a structure that records information about an employee;
this type can be used to create instances of the structure, etc.

## Base Types<a name="basetypes"></a>

A value can be a base type; when it is a base type is contains only
one value at a time, and that value has a specific type.  These are
listed here.

&nbsp;

|   Type     | Example  |    Range              | Description  |
| ---------- | -------- | --------------------- | -------------|
| `nil`      | nil      | nil                   | The `nil` value indicates no value or type specified |
| `bool`     | true     | true, false           | A Boolean value that is either true or false |
| `byte`     | 5        | 0-255                 | An 8-bit unsigned integer |
| `int32`    | 1024     | -32768 to 32767       | A signed 32-bit integer |
| `int`      | 1024     | -32768 to 32767       | A signed 32-bit integer |
| `int64`    | 1573     | -2^63 to 2^63 -1      | A 64-bit integer value |
| `float32`  | -3.14    | -1.79e+38 to 1.79e+38 | A 32-bit floating point value |
| `float64`  | -153.35  | -1.79e+308 to 1.79e+308 | A 64-bit floating point value |
| `string`   | "Andrew" | any                   | A string value, consisting of a varying number of Unicode characters |
| `chan`     |  chan    | any                   | A channel, used to communicate values between threads |

_Note that the numeric range values shown are approximate._ Also,
_Ego_ supports a single integer type of `int` which is a 64-bit integer.

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

A `chan` value has no constant expression; it is a type that can be
used to create a variable used to communicate between threads. See
the section below on threads for more information.

## Arrays<a name="arrays"></a>

An array is an ordered list of values. That is, it is a set where each
value has a numerical position referred to as it's index. The first
value in an array has an index value of 0; the second value in the
array has an index of 1, and so on. An array has a fixed size; once it
is created, you cannot add to the array directly.

Array constants can be expressed using square brackets, which contain a
list of values separated by commas. The values may be any valid value
(base, complex, or user types).  The values do not have to be of the
same type. For example,

    [ 101, 335, 153, 19, -55, 0 ]
    [ 123, "Fred", true, 55.738]

The first example is an array of integers. The value at position 0 is
`101`. The value at position 1 is `335`, and so on.  The second
example is a heterogenous array, where each value is of varying types.
For example, the value at position 0 is the integer `123` and the
value at position 1 is the string `"Fred"`.

These kinds of arrays are _anonymous_ arrays, in that they have no
specific type for the values. You can also specify a type for the 
array using a typed array constant. For example,

    a := []int{101, 102, 103}

In this example, an array is created that can only contain `int` values.
If you specify a value in the array initialization list that is not an
`int`, it is converted to an `int` before in is stored. You an then
only store `int` values in the array going forward,

    a[1] = 1325    // Succeeds
    a[1] = 1325.0  // Failed, must be of type int

## Structures<a name="structures"></a>

A structure (called `struct` in the _Ego_ language) is a set of
key/value pairs. The key is an _Ego_ symbol, and the value is any
supported value type. Each key must be unique. The values can be
read or written in the struct based on the key name. Once a struct
is created, it cannot have new keys added to it directly. A struct
constant is indicated by braces, as in:

    {  Name: "Tom", Age: 53 }

This struct has two members, `Name` and `Age`. Note that the
member names (the keys of the key/value pair) are case-sensitive.
The struct member `Name` is a string value, and the struct member
`Age` is an int value.

This type of struct is known as an _anonymous_ struct in that it
does not have a specific type, and in fact the fields are all
declared as type interface{} so they can hold any arbitrary values
unless static type checking is enabled.

You cannot add new fields to this struct if you create a struct
constant with fields already. That is, you cannot

    a := { Name: "Bob" }
    a.Age = 43

The second line will generate an error because Age is not a member
of the structure. There is one special case of an _anonymous_
struct that can have fields added (or removed) dynamically. This is
an empty _anonymous_ struct,

    a := {}
    a.Name = "Fred"
    a.Gender = "M"

The empty anonymous structure can have fields added to it just by
naming them, and they are created as needed.


## Maps<a name="maps"></a>

A `map` in the _Ego_ language functions the same as it does in Go. A
map is declared as having a key type and a value type, and a hashmap
is constructed based on that information. You can set a value in the
map and you can fetch a value from the map.

You can create create a map by setting a value to an empty map constant. 
For example,

    staff := map[int]string{}

This creates a map (stored in `staff`) that has an integer value as
the key, and stores a string value for each unique key. A map can
contain only one key of a given value; setting the key value a second
time just replaces the value of the map for that key.

You can also initialize the map values using `{}` notation, as in:

    staff := map[int]string{101:"Jeff", 102:"Susan"}

    staff[103] = "Buddy"
    staff[104] = "Donna"

This adds members to the map. Note that the key  _must_ be an integer
value, and the value _must_ be a string value because that's how the
map was declared. Unlike a variable, a map always has a static definition
once it is created and cannot contain values of a different type.
Attempting to store a boolean in the map results in a runtime error,
for example.

    id := 102
    name := staff[id]

This uses an integer variable to retrieve a value from the map. In this
case, the value of `name` will be set to "Susan". If there is nothing
in the map with the given key, the value of the expression is `nil`.

You can test for the existence of an item when you attempt to read it
from the map. In this notation, the second value returned in the assignment
is a boolean that indicates if the item was found or not.

    emp, found := staff[105]

In this example, `found` will be true if there was a value in the map for
the key value `105`, and the value of the map item (a string in this case)
will be stoerd in `emp`. If there was no value in the map for the given key,
`found` will be set to false, and `emp` will be nil. (Note that this is
slightly different than traditional Go, where the result would be the zero
value for the type, i.e. an empty string in this case).

## Pointers<a name="pointers"></a>

The _Ego_ language adopts the Go standards for pointers. Pointers exist
solely to identify the address of another object. This address can be
passed across function boundaries to allow the function to modify the
value of a parameter.

No pointer arithmetic is permitted; a pointer can only be set as the
address of another variable.

    var x *int                      (1)

    y := 42
    x := &y                         (2)

    fmt.Println(*x)                 (3)

In this example,

1. A variable `x` is created as a pointer to an integer
value. At the time of this statement, the value of x is `nil` and
it cannot be dereferenced without an error.

2. The value of `x` is now set to a non-nil value; it becomes the
address of the variable `y`.  From this point forward (until the
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

In this somewhat contrived example, the function `hasPositive` does not
return an integer, it returns a pointer to an integer. The logic of the
function is such that if a positive value was given, it is returned,
else a nil value is returned as the pointer value indicating _no value_
returned from the function.

As a final example, you can use pointers to allow a function to modify
a value.

    func setter( destination *int, source int) {
        *destination = source
    }

    x := 55
    setter(&x, 42)
    fmt.Println(x)

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

## User Types<a name="usertypes"></a>

The _Ego_ language includes the ability to create use-defined types.
These are limited to `struct` definitions. They allow the program to
define a short-hand for a specific type, and then reference that type
when creating a new variable of that type. The `type` statement is used
to define the type. Here is an example:

    type Employee struct {
       Name    string
       Age     int
    }

This creates a new type called `Employee` which is a struct with
two members, `Name` and `Age`. A variable created with this type
will always be a struct, and will always contain these two members.
You can then create a variable of this type using

   e := Employee{}

The `{}` indicates this is a type, and a new structure (of type
`Employee`) is created and stored in the variable `e`.  You can
initialize fields in the struct when you create it if you wish,

   a := Employee{ Name: "Robin" }

In this example, a new Employee is created and the `Name` field is
initialized to the string "Robin". The value `a` also contains a
field `Age` because that was declared in the type, but at this
point it contains the zero-value for it's type (in this case, an
integer zero). You can only initialize fields in a type that were
declared in the original type.

&nbsp;
&nbsp;

# Variables and Expressions<a name="symbolsexpressions"></a>

This section covers variables (named storage for values) and
expressions (sequences of variables, values, and operators that
result in a computed value).

## Symbols and Scope<a name="symbolsscope"></a>

A variable is a named storage location, identified by a _symbol_.
The _Ego_ language is, by default, a case-sensitive language, such
 that the variables `Age` and `age` are two different values. A
 symbol names can consist of letters, numbers, or the underscore
 ("&lowbar;") character. The first character must be either an underscore
 or a alphabetic character. Here are some examples of valid and
 invalid names:

&nbsp;

| Name      | Description |
| --------- | ----------- |
| a123      | Valid name |
| user_name | Valid name |
| A123      | Valid name, different than `a123`|
| _egg      | Valid name, but is a read-only variable |
| 15States  | Invalid name, does not start with an alphabetic character |
| $name     | Invalid name, dollar-sign is not a valid symbol character |

&nbsp;

There is a reserved name that is just an underscore, "&lowbar;". This name
means _value we will ignore._ So anytime you need to reference a variable
to conform to the syntax of the language, but you do not want or need
the value for your particular program, you can specify "&lowbar;" which is a
short-hand value for "discard this value".

A symbol name that starts with an underscore character is a read-only
variable. That is, it's value can be set once when it is created, but
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

      name := "Bob"
      name = "Mary"

In this example, the first statement uses the `:=` operator, which
causes the symbol `name` to be created, and then the string value "Bob"
is stored in the variable. If the variable already exists, this is an
error and you should use the `=` instead to update an existing value.
The `=` operator looks for the named value in the current scope, and
if it finds it, the value "Mary" is assigned to it. If the variable
does not exist at this scope, but does in an outer scope level, then
the variable at the outer scope is updated.

You can also create a variable using the `var` statement, which is
followed by a comma-separated list of names and finally a type value.
The variables are all created and set to the given type, with an
appropriate "zero value" for that type.

      var first, last string
      var e1 Employee{}

The second example creates a variable based on a user-defined type
`Employee`.  The {} characters causes an instance of that type to be
created and stored in the named variable `e1` in this example. The {}
characters can contain field initializations for the type, such as

      var e2 Employee{ Name: "Bob", Age: 55}

The type of `e2` is `Employee` and it contains initialized values for
the permitted fields for the type. If the initializer does not specify
a value for all fields, the fields not explicitly named are set to
zero values for their types.

## Constants <a name="const"></a>

The `const` statement can define constant values in the current scope.
These values are always readonly values and you cannot use a constant
name as a variable name. You can specify a single constant or a group
of them; to specify more than one in a single statement enclose the
list in parenthesis:

    const answer = 42

    const (
        first = "a"
        last = "z"
    )

This defines three constant values. Note that the value is set using an
`=` character since a symbols is not actually being created.

## Operators<a name="operators"></a>

Operators is the term for language elements that allow you to perform
mathematical or other other operations using constant values as well as
variable values, to produce a new computed value. Some operators can
operate on a wide range of different value types, and some operators
have more limited functionality.

There are _dereference_ operators that are used to access members of
a struct, values of a type, or index into an array.

&nbsp;

| Operator | Example  | Description |
| -------- | -------- | ----------- |
| .        | emp.age  | Find the member named `age` in the struct named `emp` |
| []       | items[5] | Find the value at index 5 of the array named `items` |
| {}       | emp{}    | Create an instance of a struct of the type `emp` |

The `[]` operator can also be used to access a map, by supplying the key value
in the brackets. This key value must be of the same type as the map's declared
key type. So if the map is declared as `map[string]int` then the key must be
of type `string`.

&nbsp;

There are _monadic_ operators which precede an expression and operate
on the single value given.

&nbsp;

| Operator | Example | Description |
| -------- | ------- | ----------- |
|  -       | -temp   | Calculate the negative of the value in `temp` |
| !        | !active | Calculate the boolean NOT of the value in `active` |

&nbsp;

There are _diadic_ operators that work on two values, one of which
precedes the operator and one of which follows the operator.

&nbsp;

| Operator | Example | Description |
| -------- | ------- | ----------- |
|  +       | a+b     | Calculate the sum of numeric values, the AND of two boolean values, or concatenate strings |
| -        | a-b     | Calculate the difference of the integer or floating-point values |
| *        | a*b     | Calculate the product of the numeric value, or the OR of two boolean values |
| /        | a/b     | Calculate the division of the numeric values |
| ^        | 2^n     | Calculate `2` to the power `n` |

&nbsp;

For division, integer values will result in the integer value of
the division, so `10/3` will result in `3` as the expression value.
A floating point value retains the fractional value of the conversion,
so `10.0/3.0` results in `3.333333333` as the result.

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

| Operator | Example    | Description |
| -------- | ---------- | ----------- |
|  ==      | a == b     | True if `a` is equal to `b` |
|  !=      | a != b     | True if `a` is not equal to `b` |
|  &gt;    | a &gt; b   | True if `a` is less than `b` |
|  &gt;=   | a &gt;= b  | True if `a` is less than or equal to `b` |
|  &lt;    | a &lt; b   | True if `a` is greater than `b` |
|  &lt;=   | a &lt;= b  | True if `a` is greater than or equal to `b` |

&nbsp;

## Type Conversions<a name="typeconversion"></a>

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

## Builtin Functions<a name="builtinfunctions"></a>

The _Ego_ language includes a library of built-in functions which can
also be used as elements of an expression, including having the function
value be assigned to a variable. A function consists of a name, followed
by a list of zero or more values in parenthesis, separated by commas. If
there are no values, you still must specify the parenthesis. The function
may accept a fixed or variable number of arguments, and typically returns
a single value.

&nbsp;

| Function | Example               | Description |
| -------- | --------------------- | ----------- |
| append() | append(list, 5, 6, 7) | Append the items together into an array. |
| close()  | close(sender)         | Close a channel. See the information on [Threads](#threads) for more info. |
| delete() | delete(emp, "Name")   | Remove the named field from a map, or a struct member |
| error()  | error("panic") | Generate a runtime error named "panic". |
| eval()   | eval("3 + 5")  | Evaluate the expression in the string value, and return the result, `8` |
| index()  | index(items, 55) | Return the array index of `items` that contains the value `55` |
| len()    | len(items)     | If the argument is a string, return its length in characters. If it is an array, return the number of items in the array |
| make()   | make([]int, 5) | Create an array of int values with `5` elements in the array |
| members() | members(emp)  | Return an array of strings containing the struct member names of the argument |
| type()   | type(emp)      | Return a string with the type of the argument. If emp is a struct, the result will be `"struct"` |
&nbsp;
&nbsp;

## Casting

This refers to functions used to explicitly change the type of a value, or
convert it to a comparable value where possible.  This can be done for base
type values (int, bool, string) as well as for arrays.

For base types, the following are available:

&nbsp;

| Function | Example               | Description |
| -------- | --------------------- | ----------- |
| bool()   | bool(55)              | Convert the value to a boolean, where zero values are false and non-zero values are true |
| float32()  | float32(33)      | Convert the value to a float32, in this case `33.0` |
| float64()  | float64(33)      | Convert the value to a float64, in this case `33.0` |
| int()    | int(78.3)      | Convert the value to an integer, in this case `78` |
| string() | string(true)   | Convert the argument to a string value, in this case `true` |

&nbsp;

A special note about `string()`; it has a feature where if the value passed in is an array of
integer value, each one is treated as a Unicode rune value and the resulting string is
the return value.  Any other type is just converted to its default formatted value.

You can also perform conversions on arrays, to a limited degree. This is done with
the function:

&nbsp;

| Function | Example               | Description |
| -------- | --------------------- | ----------- |
| []bool() | []bool([1, 5, 0])| Convert the array to a []bool array. |
| []int()  | []int([1, 5.5, 0])| Convert the array to a []int array. If the parameter is a string, then the string is converted to an array of ints representing each rune in the string. |
| []interface{}() | []interface{}([true, 5, "string"])| Convert the array to a []interface{} array where there are no static types for the array. |
| []float64() | []float64([1, 5, 0])| Convert the array to a []float64 array. |
| []float32() | []float32([1, 5, 0])| Convert the array to a []float32 array. |
| []string() | []string([1, 5, 0])| Convert the array to a []string array. |

&nbsp;

In all cases, the result is a typed array of the given cast type. Each
element of the array is converted to the target type and stored in the
array. So []bool() on an array of integers results in an array of bool
values, where zeros become false and any other value becomes true. The
special type name interface{} means _no specified type_ and is used
for arrays with heterogenous values.

Note the special case of []int("string"). If the parameter is not an
array, but instead is a string value, the resulting []int array contains
each rune from the original string.

### make

The `make` pseudo-function is used to allocate an array, or a channel with
the capacity to hold multiple messages. This is called a pseudo-function
because part of the parameter processing is handled by the compiler to
identify the type of the array or channel to create.

The first argument must be a data type specification, and the second argument
is the size of the item (array elements or channel messages)

    a := make([]int, 5)
    b := make(chan, 10)

The first example creates an array of 5 elements, each of which is of type `int`,
and initialized to the _zero value_ for the given type. This could have been
done by using `a := [0,0,0,0,0]` as a statement, but by using the make() function
you can specify the number of elements dynamically at runtime.

The second example creates a channel object capable of holding up to 10 messages.
Creating a channel like this is required if the channel is shared among many
threads. If a channel variable is declare by default, it holds a single message.
This means that before a thread can send a value, another thread must read the
value; if there are multiple threads waiting to send they are effectively run
one-at-a-time. By creating a channel that can hold multiple messages, up to 10
(in the above example) threads could send a message to the channel before the
first message was read.

&nbsp;
&nbsp;

# Conditional and Iterative Execution <a name="flow-control"></a>

We have discussed how variables are created, and how expressions are
used to calculate values based on variables, constant values, and
functions. However, most interesting programs require some decision
making to control the flow of execution based on the values of
variables. This section will describe how to make _either/or_ decisions
in the code, and how to execute a block of code repeatedly until a
condition is met.

## If-Else <a name="if"></a>

The general nature of a conditional `if` statement is

     if <condition> {
         <statements>
     } else { 
         <statements>
     }

The `else` clause is optional, as described below. Even when there 
is only a single statement in the block, a basic block is used for
readability.

Consider the following example code:

    salary := hours * wage                  (1)
    if salary < 100.0 {                     (2)
        fmt.Println("Not paid enough!")     (3)
    }                                       (4)
    total = total + salary                  (5)


This introduces a number of new elements to a program, so let's go
over them line-by-line. The numbers is parenthesis are not part of
the program, but are used to identify each line of the code.

1. This first line calculates a new value by multiplying the `hours`
   times the `salary`, and store it in a new value called `salary`.
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

3. If salary is less than 100.0, then the fmt.Println() operation is
   performed. Don't worry that we haven't talked about this yet; its
   covered below in the section on the `fmt` package, but it is enough
   to know that this produces a line of output with the string
   `Not paid enough!`. If, however, the value of `salary` is not less
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
that has the statements to execute if teh condition is false. That
is, the result of the expression will result in one or the other of
two basic blocks being executed.

    salary := hours * wage
    if salary < 100 {
        scale = "small"
    } else {
        scale = "large"
    }

In this example, after calculating a value for `salary`, it is
compared to see if it is less than 100. If so, then the value
`scale` is set to the value `"small"`. But if the value of `salary`
is not less than 100, the value of `scale` is set to `"large"`.
Regardless of which basic block was executed, after the block
executes, the program resumes with the next statement after the
 `if` statements.

## For _condition_ <a name="for-conditional"></a>

The simplest form of iterative execution (also referred to as a
"loop") is the `for` statement, followed by a condition, and a
basic block that is executed as long as the condition is true.

     for <condition> {
        <statements>
     }

Here is an example

    value := 0                  (1)
    for value < 5 {             (2)
        fmt.Println(value)      (3)
        value = value + 1       (4)
    }                           (5)

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

## For _index_ <a name="for-index"></a>

You can create a `for` loop that explicitly specifies an expression
that defines the starting value, ending condition, and how the value
changes with each iteration of a loop. For example,

     for i := 0; i < 10; i = i + 1 {
         fmt.Println(i)
     }

This loop will print the values `0` through `9` to the standard
console. The index variable `i` is first initialized with the value
`0` from the first `for` statement clause. The second statement
clause describes the condition that must be true for the loop body
to be executed. This is evaluated before the loop body is run each
time. The third clause is the statement that modifies the index
value _after_ the body of the loop is run but _before_ the  next
evaluation of the clause that determines if the loop continues.

The variable `i` in the above example is scoped to the `for`
statement and it's loop body. That is, after this loop runs, the
variable `i` will no longer exist because it was created (using
the `:=` operator) in the scope of the loop. You can use a simple
 assignment (`=`) of an existing variable if you want the updated
 index value available after the loop body ends.

    var i int
    for i = 0; i < 10; i = i + 1 {
        fmt.Println(i)
    }
    fmt.Println("The final value of i is ", i)

This example uses a variable that already exists outside the scope
of the `for` loop, so the value continues to exist after the loop
runs. This allows the final statement to print the value of the
index variable that terminated the loop.

## For _range_ <a name="for-range"></a>

You can create a loop that indexes over all the values in an array,
in sequential order. The index value is the value of the array
element. For example,

     ids := [ 101, 143, 202, 17]
     for i, j := range ids {
        fmt.Println("Array member ", i, " is ", j)
     }

This example will print a line for each value in the array, in the
order they appear in the array.  During each iteration of the loop,
the variable `i` will contain the numerical array index  and the
variable `v` will contain the actual values from the array for each
iteration of the loop body. During execution of the loop body, the
value of `i` (the _index_ variable)` contains the next value of the
array for each iteration of the loop.  You can also specify a
second value, in which case the loop defines an index number as well
as index value, as in:

    for _, v := range ids {
        fmt.Println(v)
    }

In this example, for each iteration of the loop, the variable `v`
will contain the actual values from the array for each iteration of
the loop body. By using the reserved name `_` for the index variable,
the index value for each loop is not available.  Similarly, you can
use the range to get all the index values of an array:

    for i := range ids {
        fmt.Println(v)
    }

In this case, if the array `ids` has 5 values, then this will print
the numbers 1 through 5. The value of the array can be accessed inside
the body of the loop as `ids[i]`.

Similarly, you can use the `range` construct to step through the values
of a map data type. For example,

    inventory := map[string]int{}
    inventory["wrenches"] = 5
    inventory["pliers"] = 12
    inventory["hammers"] = 2

    for product, count := range inventory {
        fmt.Println("There are ", count, " ", product, " in stock.")
    }

When the loop runs, the value of `product` is set to each key in the
map, and `count` is set to the value associated with that key. These
variables exist only within the body of the loop. Note that if you
omit either one and use the `_` variable instead, that item (key or
value) is not read from the map. You can use this to generate a list
of the keys, for example:

    names := []string{}
    for name := range inventory {
        names = names + name
    }
    fmt.Println("The products are all named", names)

This creates an array of string values, and stores the name of each
key in the list by appending them.

## `break` and `continue` <a name="break-continue"></a>

Sometimes when running an loop, you may wish to change the flow of
execution in the loop based on conditions unrelated to the index
variable. For example, consider:

    for i := 1; i < 10; i = i + 1 {
        if i == 5 {                      (1)
            continue                     (2)
        }
        if i == 7 {                      (3)
            break                        (4)
        }
        fmt.Println("The value is ", i)  (5)
    }

This loop will print the values 1, 2, 3, 4, and 6. Here's what
each statement is doing:

1. During each execution of the loop body, the index value is
compared to 5. If it is equal to 5 (using the `==` operator),
the conditional statement is executed.
2. The `continue` statement causes control to branch back to the
top of the loop. The index value is incremented, and then tested
again to see if the loop should run again. The `continue` statement
means "stop executing the rest of this loop body, but continue
looping".
3. Similarly, the index is compared to 7, and if it equal to 7 then
the conditional statement is executed.
4. The `break` statement exits the loop entirely. It means "without
changing any of the values, behave as if the loop condition had been met and resume execution after the loop body.

&nbsp;
&nbsp;

# User Function Definitions <a name="user-functions"></a>

In addition to the [Builtin Functions](#builtinfunctions) listed
previously, the user program can create functions that can be
executed from the _Ego_ program. Just like variables, functions
have scope, and can only be accessed from within the program in
which they are declared. Most functions are defined in the program
file before the body of the program.

## The `func` Statement <a name="function-statement"></a>

Use the `func` statement to declare a function. The function must
have a name, optionally a list of parameter values that are passed
to the function, and a return type that indicates what the function
is expected to return. This is followed by the function body described
as a basic block. When the function is called, this block 
is executed, with the function arguments all available as local
variables.  For example,

    func addValues( v1 float64, v2 float64) float64 {
        x := v1 + v2
        return x
    }

    // Calculate what a variable value (10.5) added to 3.8 is
    a := 10.5
    x := addValues(a, 3.8)
    fmt.Println("The sum is ", x)

In this example, the function `addValues` is created. It accepts two
parameters; each is of type float64 in this example. The parameter
values actually passed in by the caller will be stored in local
variables v1 and v2. The function definition also indicates that
the result of the function will also be a float64 value.

Parameter types and return type cause type _coercion_ to occur,
where values are converted to the required type if they are not
already the right value type. For example,

    y := addValues("15", 2)

Would result in `y` containing the floating point value 17.0.
This is because the string value "15" would be converted to a
float64 value, and the integer value 2 would be converted to a
float64 value before the body of the function is invoked. So
`type(v1)` in the function will return "float64" as the result,
regardless of the type of the value passed in when the function
was called.  

The `func` statement allows for a special data type `interface{}`
 which really means "any type is allowed" and no conversion occurs.
  If the function body needs to know the actual type of the value
   passed, the `type()` function would be used.

A function that does not return a value at all should omit the
return type declaration.

## The `return` Statement  <a name="return-statement"></a>

When a function is ready to return a value the `return` statement
is used. This identifies an expression that defines what is to be
returned. The `return` statement results in this expression being
_coerced_ to the data type named in the `func` statement as the
return value.  If the example above had a `string` result type,

    func addValues( v1 float64, v2 float64) string {
        x := v1 + v2
        return x
    }

    y := addValues(true, 5)

The resulting value  for `y` would be the string "6". This is
because not only will the boolean `true` value and the integer
 5 be converted to floating point values, bue the result will
 be converted to a string value when the function exits.

Note that if the `func` statement does not specify a type for
the result, the function is assumed not to return a result at
 all. In this case, the `return` statement cannot specify a
 function result, and if the `return` statement is the last
 statement then it is optional. For example,

    func show( first string, last string) {
        name := first + " " + last
        fmt.Println("The name is ", name)
    }

    show("Bob", "Smith")
  
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

## The `defer` Statement  <a name="defer-statement"></a>

Sometimes a function may have multiple places where it returns
from, but always wants to execute the same block of code to
clean up a function (for example, closing a file that had been
 opened). For example,

    func getName() bool {
        f := io.Open("name")
        defer io.Close(f)

        s := io.ReadLine(f)
        if s == "" {
            return false
        }
        return true
    }

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

## Function Variable Values  <a name="function-variables"></a>

Functions can be values themselves. For example, consider:

    p := fmt.Println

This statement gets the value of the function `fmt.Println` and
stores it in the variable p. From this point on, if you wanted
to call the package function that prints items to the console,
instead o fusing `fmt.Println` you can use the variable `p` to
invoke the function:

    p("The answer is", x)

This means that you can pass a function as a parameter to another
function. Consider,

    func show( fn interface{}, name string) {
        fn("The name is ", name)
    }
    p := fmt.Println
    show(p, "tom")

In this example, the `show` function has a first parameter that
is a function, and a second parameter that is a string.  In the
body of the function, the variable `fn` is used to call the
`fmt.Println` function. You might use this if you wanted to send
output to either the console (using `fmt.Println`) or a file
 (using `io.WriteString`). The calling code could make the choice
  of which function is appropriate, and pass that directly into
  the `show` function which makes the call.

You can even create a function literal value, which defines the
 body of the function, and either store it in a variable or pass
 it as a parameter. For example,

    p := func(first string, last string) string {
             return first + " " + last
         }

    name := p("Bob", "Smith")

Note that when defined as a function literal, the `func` keyword
is not followed by a function name, but instead contains the
parameter list, return value type, and function body directly.  
There is no meaningful difference between the above and declaring
 `func p(first string...` except that the variable `p` has scope
  that might be limited to the current _basic block_ of code.  
  You can even define a function as a parameter to another
  function directly, as in:

    func compare( fn interface{}, v1 interface{}, v2 interface) bool {
        return fn(v1, v2)
    }

    x := compare( func(a1 bool, a2 bool) bool { return a1 == a2 }, true, false)

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

## Function Receivers  <a name="function-receivers"></a>

A function can be written such that it can only be used when
referenced via a variable of a specific type. This type is created
by the user, and then the functions that  are written to operate on
that type use any variable of the type as a _receiver_, which means
that variable in the function gets the value of the item in the
function invocation without it being an explicit parameter. This
also allows multiple functions of the same name to exist which
just reference different types.  For example,

    type Employee struct {                        (1)
        first string
        last  string
        age   int
    }

    func (e Employee) Name() string {             (2)
        return e.first + " " e.last
    }

    var foo Employee{                             (3)
        first: "Bob", 
        last: "Smith"}
        
    fmt.Println("The name is ", foo.Name())       (4)

Let's take a look more closely at this example to see what's going
on.

1. This defines the type, called `Employee` which has a number of
   fields associated with it.
2. This declares a function that has a variable `e` of type
   `Employee` as it's receiver. The
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

    func (e *Employee) SetName(f string, l string)  { 
        e.first = f
        e.last = l
    }

This function can be used to set the names in the receiver
variable. If the function was declared without the "*" marker,
the receiver would actually only be a copy of the instance. So
the function `Name()` above can read any of the values from the
receiver, if it sets them it is changing a copy of the instance
that only exists while the function is executing, and the
values in the `foo` instance are not changed.

By using the "*" as in the `SetName` example, the receiver `e`
isn't a copy of the instance, it is the actual instance. So
changes made to the receiver `e` will actually be made in the
instance variable `foo`. An easy way to remember this is that,
without the "*", the receiver does not change the instance that
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

The type `Employee` contains within it an item `Info` which is
of another type, `EmpInfo`. Note that when initializing the
fields of the newly-created instance variable `e`, you must
identify the type name for the `Info` field explicitly, since
it acts as an instance generator itself for an instance of
`EmpInfo` which is then stored in the field `Info` in the
structure.

&nbsp;
&nbsp;

# Error Handling <a name="errors"></a>

There are two kinds of errors that can be managed in an
_Ego_ program.

The first are user- or runtime-generated errors, which
are actually values of a data type called `error`. You can
create a new error variable using the `error()` function,
as in:

    if v == 0 {
        return error("invalid zero value")
    }

This code, executed in a function, would return a value
of type `error` that, when printed, indicates the text
string given. An `error` can also have value `nil` which
means no error is stored in the value. Some runtime
functions will return an error value, and your code can
check to see if the result is nil versus being an actual
error.

The second kind are panic error, which are errors generated
by _Ego_ itself while running your program. For example, an
attempt to divide by zero generates a panic error. By default,
this causes the program to stop running and an error message
to be printed out.

## `try` and `catch` <a name="try-catch"></a>

You can use the `try` statement to run a block of code (in the same
scope as the enclosing statement) and catch any panic errors that
occur during the execution of that block. The error causes the code
to execute the code in the `catch` block of the statement.
If there are no errors,  execution continues after the catch block.
For example:

    x := 0
    try {
        x = pay / hours
    } catch {
        print "Hours were zero!"
    }
    print "The result is ", x

If the value of `hours` is non-zero, the assignment statement will assign
the dividend to `x`. However, if hours is zero it will trigger a panic
divide-by-zero error. When this happens, the remainder of the statements
(if any) in the `try` block are skipped, and the `catch` block is executed.

You can optionally specify the name of a variable that will be created within
the catch block that contains the actual error encountered. Do this by
adding the name in parenthesis before the catch block. This can be used
in the `catch` block if it needs handle more than one possible error. For
example:

    x := 0
    try {
        x = 125 / x
    } catch (e) {
        fmt.Println("unexpected error, ", e)
    }

This can be used in the `catch` block if it needs handle more than one possible error, for example.

## Conditional expression error handling

If you need to catch a possible error in an expression, you can
use a short-form of the `try` and `catch` that works within an
expression.  Consider the following example:

    emp := { name: "Donna", age: 32 }
    
    hours := 40
    pay := emp.wage * hours

This code will generate an error on the statement that attempts
to reference the structure member `wage`, which does not exist.
If you think the field might not exist, or you are doing an
operation that might result in an error (division by zero, perhaps)
that you have a useful default for, use the conditional expression
syntax:

    emp := { name: "Donna", age: 32 }
    
    hours := 40
    pay := ?emp.wage:25.0 * hours

The "?" indicates that the following expression component (up to the ":")
is wrapped in a try/catch block. If no error occurs, the expression is
used as specified. But if there is an error ("no such structure field",
for example) then the expression after the ":" is used instead. So in the
above example, because there isn't a `wage` field in this employee's
record, the program assumes a wage of $25/hour in the calculation of
the pay.

## Signalling Errors <a name="signalling"></a>

You can cause a panic error to be signalled from within your
code, which would optionally be caught by a try/catch block,
using the @error directive:

    if x == 0 {
        @error "Invalid value for x"
    }

When this code runs, if the value of `x` is zero, then a panic
error is signalled with an error message of "Invalid value
for x". This error is indistinguishable from a panic error
generated by _Ego_ itself. If there is a `catch{}` block, the
value of the `_error_` variable will be the string "Invalid
value for x".

&nbsp;
&nbsp;

# Threads <a name="threads"></a>

Like it's inspiration, _Ego_ supports the idea of "go routines" which are threads
that can be started by an _Ego_ program, and which run asynchronously. A go routine
is always a function invocation, or a function constant value. That function is
started on a parallel thread, and will execute independently of the main program.

You can use _channels_ as a communication mechanism to communicate between the
go routines and the main program.

## Go

Use the `go` statement to start a thread. Here is a very simple example:

    func beepLater(duration string) {
        time.Sleep(duration)
        fmt.Println("BEEP!")
    }

    go beepLater("1s")

This example defines a function `beepLater` which is given a duration
string expression. The function waits for that duration, and then prints
the message to the console.

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

## Synchronization

Ego provides several data types used to synchronize execution of competing
threads, and to assist in managing access to resources in a predictable
way if needed.

| Datatype | Description |
|----------|-------------|
| sync.Mutex | A simple mutual exclusion lock for serializing access to a resource |
| sync.WaitGroup | A way to launch a varying number of go routines and wait for them to complete |

See the detailed descriptions in the later sections on the `sync` package
for more information.

## Channels

We address this synchronization issue (and also allow data to be
passed _back_ from the go routine) using channels. Here's a modified
version of the program:

    func beepLater(duration string, c chan) {
        time.Sleep(duration)
        c <- "BEEP"
    }

    var xc chan
    go beepLater("1s", xc)

    m := <- xc
    fmt.Println(m)

In this example program, the main program defines a variable `cx` which
is a _channel_ variable, of type `chan`. The duration and the channel
variable are passed to the go routine. Importantly, the program then
receives data from the channel, using the `<-` notation. This causes the
main program to wait until a message is put into the channel, and that
message is stored in the variable m

Meanwhile, the go routine starts running, and performs the wait as
before. Once the wait is completed, it puts a message (really, any
value) in the channel, again using a variant of the `<-` syntax to
show writing a value into a channel.  When this write occurs, the
main program's receive operation completes and the message is
printed.

In this way, the go function performs its work, and then sends the
result back through the channel. The main program will wait for data
to be stored in the channel before proceeding. The go routine can
send more than one data item into the channel, simply by issuing
more channel write operations. The receiver can either know how
many times to read the channel, or can use a `for...range` operation
on the channel to simply keep receiving data until done.

    func beepLater(count int, c chan) {
        for i := 0; i < int; i = i + 1 {
            c <- "Item " + string(i)
        }
    }
    var xc chan
    go beepLater(5, xc)

    for msg := range xc {
        fmt.Println("Received ", msg)
    }

In this case, the caller of the goroutine includes a count of the
number of messages to send, and that function sends that many
messages. The main program uses the `range` operation on the channel,
which means "_as many as you receive_" where each message is stored
in `msg`. The loop will terminate when the goroutine stops executing.
The goroutine can also explicitly tell the main program that it is
done by using the `close()` function on the channel. When this happens,
the range loop exits. Note that both the main program and the goroutine
will continue executing to the end even after the channel is closed.

&nbsp;
&nbsp;

# Packages <a name="packages"></a>

Packages are a mechanism for grouping related functions together. These
functions are accessed using _dot notation_ to reference the package name
and then locate the function within that package to call.

Packages may be available to your program automatically if the `ego.compiler.auto-import`
preference is set to true. If not, you must import each package before you can use it.
Additionally, packages can be created by the user to extend the runtime support for
_Ego_; this is covered later.

## import <a name="import"></a>

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

The following sections will describe the _built-in_ packages that are
provided automatically as part of Ego. You can extend the packages
by writing your own, as described later in the section on User Packages.

## db <a name="db"></a>

The `db` package provides support for accessing a database. Currently,
this must be a Postgres database or a database that uses the Postgres
wire protocol for communicating. The package has a `New` function which
creates a new database client object.

With this object, you can execute a SQL query and get back either a
fully-formed array of struct types (for small result sets) or a row
scanning object that is used to step through a result set of arbitrary
size.

### db.New("connection-string-url")

There is a simplified interface to SQL databases available. By
default, the only provider supported is Postgres at this time.

The result of the `db.New()` call is a database handle, which can be
used to execute statements or return results from queries.

     d := db.New("postgres://root:secrets@localhost:5432/defaultdb?sslmode=disable")
     r, e := d.QueryResult("select * from foo")
     d.Close()

This example will open a database connection with the specified URL,
and perform a query that returns a result set. The result set is an
Ego array of arrays, containing the values from the result set. The
`QueryResult()` function call always returns all results, so this could be
quite large with a query that has no filtering. You can specify
parameters to the query as additional argument, which are then
substituted into the query, as in:

     age := 21
     r, e := d.QueryResult("select member where age >= $1", age)

The parameter value of `age` is injected into the query where the
$1 string is found.

Once a database handle is created, here are the functions you can
call using the handle:

&nbsp;

| Function | Description |
|----------|-------------|
| d.Begin() | Start a transaction on the remote serve for this connection. There can only be one active transaction at a time
| d.Commit() | Commit the active transaction
| d.Rollback() | Roll back the active transaction
| d.QueryResult(q [, args...]) | Execute a query string with optional arguments. The result is the entire query result set.
| d.Query(q, [, args...]) | Execute a query and return a row set object
| d.Execute(q [, args...]) | Execute a statement with optional arguments. The result is the number of rows affected.
| d.Close() | Terminate the connection to the database and free up resources.
| d.AsStruct(b) | If true, results are returned as array of struct instead of array of array.

&nbsp;

When you use the Query() call it returns a rowset object. This object can be used to step through the
result set a row at a time. This allows the underlying driver to manage buffers and large result sets
without filling up memory with the entire result set at once.

&nbsp;

| Function | Description |
|----------|-------------|
| r.Next() | Prepare the next row for reading. Returns false if there are no more rows
| r.Scan() | Read the next row and create either a struct or an array of the row data
| r.Close() | End reading rows and release any resources consumed by the rowset read.
&nbsp;
&nbsp;

## fmt <a name="fmt"></a>

The `fmt` package contains a function library for formatting and printing output to the
stdout console. These are generally analogous to the Go functions of the same name. Some
functions return two values (a result or length, and an error). If the caller does not
specify that the result is assigned to two variables, then the error is ignored.

Note that only a subset of the equivalent Go functions are supported in _Ego_.

### fmt.Printf()

The `Printf` function formats one or more values using a format string, and sends the
resulting string to the standard out. It returns the length in characters of the
string written to the output, and an error which will be nil if no error occurred during
format processing.

    answer := 42
    kind := "correct"
    count, err := fmt.Printf("The %s answer is %d\n", kind, answer)

In this example, the format string is processed, and the substitution format operators
read parameters (in the order encountered) from the call. So the first operator `%s`
looks for a string value in the variable `kind` and inserts it into the message. It
uses the second operator `%d` to indicate that it is looking for an integer value which
is inserted in the string using the value of `answer`.

See the [official Go documentation](https://golang.org/pkg/fmt/#hdr-Printing) for
detailed information on the format operators supported by the fmt package.

### fmt.Println()

The `Println` function prints one or more items using the default format for their
data type to the standard out, with a single space placed between them. The output
is followed by a newline character. There are no formatting operations available.

    answer := 42
    fmt.Println("The answer is", answer)

This results in the string `"The answer is 42"` followed by a newline character being
send to the output console.

### fmt.Sscanf()

The `Sscanf()` function accepts a string of data, a format specification, and one or
more pointers to base-type values. The data string is processed using the format
specification, and the resulting values are written to the parameter variables.
The function returns the number of items processed, and any error (such as invalid
value for a given format).

    var age int
    var temp float64

    data := "35 101.2"
    fmt.Sscanf(data, "%d%f", &age, &temp)

The `%d` format specification causes an integer value to be parsed from the string.
This is followed by a floating pointer number. These are stored in `age` and `temp`
respectively.

Any non-format characters in the format string must be present in the input string
exactly as shown.  For example,

    data := "age 35 temp 101.2"
    fmt.Sscanf(data, "age %d temp %f", &age, &temp)

Note that in both the data string and the format string, multiple white-space
characters (" ", etc) are ignored.  The supported format values are:

&nbsp;

| Format | Description |
|:------:| ----- |
| %t | Boolean "true" or "false" value |
| %f | Floating point value |
| %d | Integer value |
| %s | String values |

&nbsp;

Note that this is a subset of the format operations supported by Go's runtime.
Also note that _Ego_ does not support a width specification in the format.

### fmt.Sprintf()

The `Sprintf()` function works exactly the same as the `Printf{}` function, but returns
the formatted string as it's result value, instead of printing it anywhere. This lets
you use the formatting operations to construct a string value that includes other
values in the string.

    v := "foobar"
    msg := fmt.Sprintf("Unrecognized value %s")

This creates a string named `msg` which contains "Unrecognized value foobar" as it's
contents. The value is not printed to the console as part of this operation.

## io <a name="io"></a>

The io package supports input/output operations using native files in the file system
of the computer running _Ego_.


### io.DirList(path)

The `DirList` function produces a string containing a human-formatted directory
listing, similar to the Unix "ls" command. The result string is already formatted
with line breaks, etc.

### io.Expand(path)

The `Expand()` function produces an array of strings containing the absolute path
names of all files found within the given path.

   a := "/tmp"
   fns := io.Expand(a)

The value of `fns` is a []string and contains the names of each file found in the
directory "/tmp".

### io.Open(filename [, mode])

The `Open()` function opens a file, and returns a file handle that can be used to
perform specific operations on the file.

    fn := "mydata.txt"
    mode := "create"
    f := io.Open(fn, mode)

This program opens a file named "mydata.txt" for output, and creates the file if it
does not already exist. The mode variable can be one of the following values

&nbsp;

| Mode   | Description |
|:------:| ----------- |
| append | The file must exist, and is opened for writing. All new data is written to the end of the file. |
| create | The file is created (any previous contents are lost) and available for writing. |
| read   | The file must already exist, and is opened for reading only |
| write  | The file must already exist, and is opened for writing only |

&nbsp;

Once a file handle is created, you can use the file handle to perform additional operations
on the file, until you use the `Close()` method of the handle which closes the file so it
completes all operations and then the handle cannot be used again until another `io.Open()`
operation. The file handle functions are:

&nbsp;

| Function            | Description |
| ------------------- | ----------- |
| Close()             | Close the file, after which the file object can no longer be used. |
| ReadString()        | Read a line of text from the file and return it as a string |
| WriteString(string) | Write a string to the output file and add a newline |
| Write(value)        | Write an arbitrary value to the output file |
| WriteAt(value, int) | Write an arbitrary value at specific position in the file |

&nbsp;

### io.ReadDir(path)

The `ReadDir()` function profiles a list of all the files in a given directory
path location. This is the form of an array of structures which describe each
file.

    a := io.ReadDir("/tmp")

This will produce an array `a` containing information on each file in the "/tmp"
directory. An empty array is returned if there are no files.  Each array structure
has the following members:

&nbsp;

| Field     | Type   | Description |
| --------- | ------ | ----------- |
| directory | bool   | true if the entry is a subdirectory, else false if it is a file |
| mode      | string | Unix-style mode string for permissions for the file |
| modified  | string | Timestamp of the last time the file was modified |
| name      | string | The name of the file |
| size      | int    | The size of the file contents in bytes |

&nbsp;

### io.ReadFile(filename)

The `ReadFile` function reads input from a file. If the filename is a "." then the
function reads a single line of text from stdin (the console or a pipe). Otherwise,
the filename must be the absolute or relative path to a file in the file system, and
its' entire contents are returned as a single string value.

    fn := "mydata.txt"
    s := io.ReadFile(fn)

The variable `s` will contain a string containing the entire contents of the input
file, including with line breaks. You can use `strings.Split()` to convert this into
an array of strings based on the line breaks if you wish.

### io.WriteFile(filename, string)

The `WriteFile()` function write a string value to a file. If the file does not
exist, it is created. If the file previously existed, the contents are over-written
by the new file.

    fn := "mydata.txt"
    s := io.ReadFile(fn)
    io.WriteFile("newdata.txt", s)

This reads the contents of the "mydata.txt" file, and then writes it to the
"newdata.txt" file, in its entirety.

## json <a name="json"></a>

The `json` package is used to convert an _Ego_ data value into equivalent JSON expressed
as a string value, or convert a JSON string to a comparable _ego_ data value.

### json.Marshal(v)

The `Marshal` function converts a value into a JSON string expression, which is the function
result. Note that unlike its _Go_ counterpart, the `json` package automatically converts the
value expression to a string as the result.

    a := { name: "Tom", age: 44 }
    s := json.Marshal(a)

This results in `s` containing the value "{ \"name\":\"Tom\", \"age\": 44}". This value can
be passed as the body of a rest request, for example, to send an instance of this structure
to the REST service.

### json.MarshalIndented(v)

The `MarshalIndented` function converts a value into a JSON string expression, which is the
function result. Note that unlike its _Go_ counterpart, the `json` package automatically
converts the value expression to a string as the result. The `MarshalIndent` function differs
from the standard `Marshal` function in that it provides indentation automatically to make
the JSON string more readable.

    a := { name: "Tom", age: 44 }
    s := json.Marshal(a)

This results in `s` containing the value

    {
        "name" : "Tom",
        "age" : 44
    }

### json.UnMarshal(string)

Given a JSON string expression, this creates the equivalent JSON object value. 
This may be a scalar type (such as int, string, or float64) or it may be an 
array or structure, or a combination of them. You do not have to provide a 
model of the data type; the `UnMarshal` function creates one dynamically. 
This means you are not guaranteed that the resulting structure has all the
fields you might be expecting.

    a := json.UnMarshal(s) 

If `s` contains the JSON expressions from the `Marshal` example above, the result is a
structure { age: 44, name:"Tom"} in the variable `a`. You can use the `members()` function
to examine if a structure contains a field you expected.

## math <a name="math"></a>

The `math` package provides basic and extended math operations on common _Ego_ numeric
data types (usually `int` and `float64` values). This is not a complete set of the math
function that are offered in the comparable _Go_ package, but will be expanded as needed.

### math.Abs(n)

For a given numeric value, return the absolute value of the number.

    posInt := math.Abs(signedInt)

In this example, `posInt` will always be a positive or zero value.

### math.Factor(i)

For a given positive integer `i`, return an array of all the unique factors for that
value. The array is always an array of integers. For a prime number, this will always
return an array with two elements, one and the prime number. For all other numbers,
it returns an array that contains one, the number, and all factors of the number.

    a := math.Factor(11)
    b := math.Factor(12)

For the first example, `a` contains [1, 11] because 11 is a prime number. The value of
`b` contains [1, 2, 3, 4, 6, 12].

### math.Log(f)

For a given floating point value `f`, return the natural logarithm of the value.

   f := math.Log(2.1)

The value of `f` is 0.7419373447293773.

### math.Max(...)

For an arbitrary list of numeric values, return the largest value in the list. The list
can be sent as individual items, or as an array of items.

    a := math.Max(n, 100)
    
    b := [1, 2, 6, 3, 0]
    c := math.Max(b...)

The value of `a` is the larger of the value of `n` and the value 100. This is comparable
to _use the value of `n` but it must be at least 100_. The value of `c` will be 6. The
ellipsis "..." notation indicate that the array b is to be treated as individual parameters
to the function, and the largest value in the array `b` is 6.

### math.Min(...)

For an arbitrary list of numeric values, return the smallest value in the list. The list
can be sent as individual items, or as an array of items.

    a := math.Min(n, 10)
    
    b := [1, 2, 6, 3, 0]
    c := math.Min(b...)

The value of `a` is the smaller of the value of `n` and the value 1. This is comparable
to _use the value of `n` but it must be at no larger than 10_. The value of `c` will
be 0. The ellipsis "..." notation indicate that the array b is to be treated as individual
parameters to the function, and the smallest value in the array `b` is 0.

### math.Primes(i)

The `Primes` function accepts a positive integer value and returns an array of all the
prime numbers less than that value. Note that this can take a very long time to compute
for larger values.

    a := math.Primes(10)

The array `a` will contain the integers [3, 5, 7]. The values '1' and '2' are not considered
to be prime numbers.

### math.Sqrt(f)

Calculate the square root of the numeric value given.

    a := math.Sqrt(2)

The value of `a` will be approximately 1.4142135623730951.

### math.Sum(...)

The `Sum` function returns the arithmetic sum of all the numeric values. These can be
passed as individual values or as an array.

    a := math.Sum(n, 10)

    b := [5, 15, 25, 35]
    c := math.Sum(b...)

The value of `a` is the sum of `n` and 100, and is identical to the expression `a := n + 10`. The
value of `c` is 80, which is the sum of all the values in the array. Note that the ellipsis "..."
notation indicates that the array should be converted to a list of parameters.

## os <a name="os"></a>

The `os` package provides a number of functions that access operating system features
for whatever operating system (macOS, Windows, Linux, etc.) you are running on. The results
and the behavior of the routines can be specific to that operating system. The examples
shown here are for macOS (the "darwin" Go build).

### os.Args()

The `Args{}` function returns an array of the string command line arguments when an _Ego_
program is run from the shell/command line.  Consider the following simple program:

    func main() int {
        fmt.Println(os.Args())
    }

This has a `main` function (the function that is always invoked with the `ego run` command).
This gets the list of arguments via `os.Args()` and prints it to the standard output.  If
this is placed in a file -- for example, "args.ego" -- then it can be run with a command
line similar to:

    tom$ ego run args.ego stuff I want

The "tom$" is the shell prompt; the remainder of the command is the command line entered. Note
that after the name of the program file there are additional command line tokens. The main
function in "args.ego" will retrieve these and print them, and the output will look like:

    [ "stuff", "I", "want"]

The result is an array where each element of the array is the next token from the original
command line.

### os.Exit(i)

The `Exit()` operation stops the execution of the _Ego_ program and it's runtime environment,
and returns control to the shell/command line where it was invoked. If an optional parameter
is given, it is an integer that becomes the system return code from the `ego run` command
line.

    main() int {
        if true {
            os.Exit(55)
        }

        return 0
    }

In this example, the condition is always true so the `os.Exit(55)` call is made. When the
ego command completes, the shell return code ("$?" in most Linux/Unix shells, for example)
will be the value "55".

If the main program returns a non-zero return code, this has the same effect as calling
`os.Exit()` with that value. If no `os.Exit()` call is made and the main program simply
terminates, then the return code value is assumed to be 0, which indicates successful
completion of the code.

### os.Getenv(name)

The `Getenv()` function retrieves an environment variable from the shell that invoked
the _Ego_ processor. This can be an environment variable from a Linux shell, or a
DOS-style environment variable from the CMD.EXE Windows shell.  The argument must be
the name of the variable (case-sensitive) and the result is the value of the environment
variable. If the variable does not exist, the function always returns an empty string.

    func main() int {

        shell := os.Getenv("SHELL")
        fmt.Println("You are running the ", shell, " shell program")

        return 0
    }

Invoking this on a macOS or Linux system while running the "bash" shell will result in
output similar to:

    You are running the  /bin/bash  shell program

### os.Remove(filename)

The `Remove()` function deletes a file from the file system.

    fn := "newdata.txt"
    os.Remove(fn)

When this program runs, the physical file "newdata.txt" will have been deleted
from the file system, assuming the current user has permission to delete the
file.

## profile <a name="profile"></a>

The `profile` package help manage persistent profile settings. These are the same settings
that can be accessed from the command line using the `ego profile` command. They apply to
settings found in the current active profile.

Profile settings all have a name, which is a string value to identify the key. The prefix
"ego." is reserved for settings related to the _Ego_ compiler, runtime, and server settings.
You can use any other prefix to store settings related to your particular _Ego_ application
usage.

The profile values are stored in the .org.fernwood/ego.json file located in your default
home directory. This file must be readable to access profile settings, and the file is
rewritten when a setting value is changed and _Ego_ exits.  Note that this file contains
all the profiles, not just the default profile (or profile specified with the --profile
command-line option).

### profile.Delete(key)

The `Delete()` function deletes a setting from the active profile by name. If the profile
value does not exist, there is no error.

### profile.Get(key)

The `Get()` function retrieves the current value of a given setting by name. For example,

   path := profile.Get("ego.path")

In this case, the variable `path` is a string containing the file system location for the
_Ego_ main path, where service functions, import libraries, and test programs are found.
If you request a profile value for a setting that does not exist, an empty string is
returned.

### profile.Keys()

The `Keys()` call returns a string array containing the names of all the profile values that
are currently set (i.e. have non-empty values). This can be used to determine if a profile
setting exists or not before getting its value.

### profile.Set(key, value)

The `Set()` function creates or updates a profile setting by name, with the given value. The
value is converted to a string representation and stored in the profile data under the named
key. The key does not need to exist yet; you can create a new key simply by naming it.

## rest <a name="rest"></a>

The `rest` package provides a generalized HTTP/HTTPS client that can be used to
communicate with a server, authenticate to it (either using username/password or
an authentication token), and perform GET, POST, and DELETE operations against
URL endpoints.

The package supports sending and receiving arbitrary Ego data structures which
are expressed as JSON data to the server, or sending and receiving text payloads.

If the server being communicated with is an _Ego_ server, then you can use the
`ego logon` command to create a local token used to authenticate to the server.

### rest.New(<user, password>)

This returns a rest connection handle (an opaque Go object represented by an Ego symbol
value). If the optional username and password are specified, then the request will use
Basic authentication with that username and password. Otherwise, if the logon-token
preference item is available, it is used as a Bearer token string for authentication.

The resulting item can be used to make calls using the connection just created. For
example, if the value of `rest.New()` was stored in the variable `r`, the following
functions would become available:

&nbsp;

| Function | Description |
|----------|-------------|
| r.Base(url) | Specify a "base URL" that is put in front of the url used in get() or post()
| r.Get(url) | GET from the named url. The body of the response (typically json or HTML) is returned as a string result value
| r.Post(url [, body]) | POST to the named url. If the second parameter is given, it is a value representing the body of the POST request
| r.Delete(url) | DELETE to the named URL
| r.Media("type") | Specify the media/content type of the exchange
| r.Verify(b) | Enable or disable TLS server certificate validation
| r.Auth(u,p) | Establish BasicAuth with the given username and password strings
| r.Token(t) | Establish Bearer token auth with the given token value

&nbsp;

Additionally, the values `r.status`, `r.headers`, `r.cookies`, and `r.response` can be used
to examine the HTTP status code of the last request, the headers returned, and the value of
the response body of the last request. The response body may be a string (if the media type
was not json) or an actual object if the media type was json.

Here's a simple example:

    server := rest.New().Base("http://localhost:8080")
    server.Get("/services/debug")
     
    if server.status == http.StatusOK {
        print "Server session ID is ", server.response.session
    }

## sort <a name="sort"></a>

The `sort` package contains functions that can sort an array containing only
homogeneous base types (int, string, float64). If the array contains interface
or struct types, it cannot be sorted. The sort occurs "in place" in the array.

### sort.Ints(array)

The `Ints` function sorts an array of integers. Negative numbers sort before positive
numbers.

    a := []int{5, 3, 8, 0, -1}
    sort.Ints(a)

After this code executes, the value of the array is [-1, 0, 3, 5, 8].

### sort.Floats(array)

The `Floats` function sorts an array of floating point numbers. Negative
numbers sort before positive numbers.

    a := []float64{5.3, 3, 8.001, 0, -1.5}
    sort.Floats(a)

After this code executes, the value of the array is [-1.5, 0.0, 3.0, 5.3, 8.001].

### sort.Slice(array, func)

The `Slice` function allows you to sort an array of non-base type. For example,
you could create an array of struct types; the builtin `sort` functions don't know
how to sort that structure. You can sort it using the `Slice` function by
supplying a function constant that is able to decide which of two items in
the array is _less than_ the other.  Even though the examples could be more
complex, here's an example using integer values:

    a := []int{ 101, 5, 33, -55, 239, 3, 66}

    sort.Slice(a, func(i int, j int) bool {
        return a[i] < a[j]
    })

When this runs, the array `a` will be in sorted order. The function constant
(the _comparison function_) is called by the `sort` package algorithm as many
times as needed to compare two values in the array. The function _must_
accept two integer values as arguments, and return a bool value. The function
result is determining if the `i` element of the array is less than the `j`
element of the array. The `Slice` function manages the sort algorithm, and
calls back to your supplied function as needed.

Note that the comparison function has to be defined as an anonymous function
constant in the string, so it has access to values outside the function
(specifically, the array value)

### sort.Strings(array)

The `Strings` function sorts an array of strings. An empty string sorts to
the start of the list.

    a := []string{"apple", "pear", "", "cherry"}
    sort.Strings(a)

After this code executes, the value of the array is ["", "apple", "cherry", "pear"].

## strings <a name="strings"></a>

The `strings` package contains a library of functions to support manipulation
of string data. Unless otherwise noted, strings are interpreted as a set of
characters, so some unicode characters can take more than one byte of storage.

### strings.Chars(s)

The `Chars` function returns an array of string values. Each value represents
a single character for that position in the string.

    runes := strings.Char("test")

The value of `runes` is an string array with values ["t", "e", "s", "t"].
If the string is an empty string, it results in an array of zero elements.

### strings.Compare(a, b)

The `Compare` function compares two string values, and returns an integer containing
-1 if the first string is less than the second, 0 if they are equal, or 1 if the
second value is less than the first value.

    fmt.Println(strings.Compare("peach", "apple"))

This will print the value 1 as the second value sorts higher in order than
the first value.

### strings.Contains(str, substr)

The `Contains` function scans a string for a substring and returns a boolean
value indicating if the substring exists in the string

    a := strings.Contains("This is a test", "is a")
    b := strings.Contains("This is a test", "isa")

In this example, `a` contains the value `true`, and `b` contains the value `false`.
Note that the substring must match exactly, including whitespace, to be considered
a match.

### strings.ContainsAny(str, chars)

The `ContainsAny` function scans a string to see if instances of any of the
characters from a substring appear in the string.

    a := strings.ContainsAny("this is a test", "satx")
    b := strings.ContainsAny("this is a test", "xyz")

In this example, `a` is true because the string contains at least one of the
characters in the substring (there are instances of "s", "a", and "t"). The
value of `b` is false because the string does not contain any instances of
("x", "y", or "z")

### strings.EqualFold(a, b)

The `EqualFold` function compares two strings for equality, ignoring differences
in case.

    a := strings.EqualFold("symphony b", "Symphony B")
    b := strings.EqualFold("åto", "Åto")

In both these examples, the result is `true`.

### strings.Fields

The `Fields` function breaks a string down into individual strings based on
whitespace characters.

    s := "this is    a test"
    b := strings.Fields(s)

The result is that `b` will contain the array ["this", "is", "a", "test"]

### strings.Join

The `Join` function joins together an array of strings with a separator string.
The separator is placed between items, but not at the start or end of the
resulting string.

    a := []string{ "usr", "local", "bin"}
    b := strings.Join(a, "/")

The result is that `b` contains a string "usr/local/bin". This function is most
commonly used to create lists (with a "," for separator) or path names (using a
host-specific path separator like "/" or "\").

### strings.Format(v)

The `Format()` function returns a string that contains the formatted value of
the variable passed in. This is the same formatting operation that is done by
the `io.Println()` function, but the resulting string is returned as the
function value instead of printed to the console.

### strings.Index(string, test)

The `Index` function searches a string for the first occurrence of the test
string. If it is found, it returns the character position of the first
character in `string` that contains the value of `test`. If no instance of
the test string is found, the function returns 0.

### strings.Ints(string)

The `Ints` function returns an array of integer values. Each value represents
the Unicode character for that position in the string, expressed as an integer
value.

    runes := strings.Ints("test")

The value of `runes` is an integer array with values [116, 101, 115, 116] which
are the Unicode character values for the letters "t", "e", "s", and "t". If
the string passed is is am empty string, the `Ints` function returns an empty
array.

### strings.Left(string, count)

The `Left()` function returns the left-most characters of the given string. If
the value of the count parameter is less than 1, an empty string is returned.
If the count value is larger than the string length, then the entire string
is returned.

    name := "Bob Smith"
    first := strings.Left(name, 3)

In this example, the value of `first` will be "Bob".

### strings.Length(string)

The `Length()` function returns the length of a string _in characters_. This
is different than the builtin `len()` function which returns the length of a
string in bytes. This difference is because a character can take up more than
one byte.  For example,

     str := "\u2813foo\u2813"
     a := len(str)
     b := strings.Length(str)

In this example, the value of `a` will be 9, which is the number of bytes stored
in the string. But because the first and last characters are unicode characters
that take multiple bytes, the value of `b` will be 5, indicating that there are
five characters in the string.

### strings.Right(string, count)

The `Right()` function returns the right-most characters of the given string.
If the value of the count parameter is less than 1, an empty string is returned.
If the count value is larger than the string length, then the entire string
is returned.

    name := "Bob Smith"
    last := strings.Right(name, 5)

In this example, the value of `last` will be "Smith".

### strings.Split(string [, delimiter])

The `Split()` function will split a string into an array of strings, based on a
provided delimiter character. If the character is not present, then a newline
is assumed as the delimiter character.

    a := "This is\na test\nstring"
    b := strings.Split(a)

In this example, `b` will be an array of strings with three members, one for each
line of the string:  ["This is", "a test", "string"]. If the string given was an
empty string, the result is an empty array.

If you wish to use your own delimiter, you can supply that as the second parameter.
For example,

    a := "101, 553, 223, 59"
    b := strings.Split(a, ", ")

This uses the string ", " as the delimiter. Note that this must exactly match, so
the space is significant. The value of b will be "101", "553", "223", "59"].

### strings.String(n1, n2...)

The `String()` function will construct a string from an array of numeric values or
string values.

    a := strings.String(115, 101, 116, 115)

This results in `a` containing the value "sets", where each integer value was used
as a Unicode character specification to construct the string.

    b := strings.String("this", "and", "that")

You can also specify arguments that are string values (including individual
characters) and they are concatenated together to make a string. In the above
example, `b` contains the string "thisandthat".

### strings.Substring(string, start, count)

The `Substring()` function extracts a portion of the string provided. The
start position is the first character position to include (1-based), and
the count is the number of characters to include in the result. For example,

    name := "Abe Lincoln"
    part := strings.Substring(name, 5, 4)

This would result in `part` containing the string "Linc", representing the
starting with the fifth character, and being four characters long.

### strings.Template(name [, struct])

The `Template()` function executes a template operation, using the supplied
data structures. See the `@template` directive for more details on creating
a template name. The struct contains values that can be substituted into the
template as it is processed. The structure's fields are used as substitution
names in the template, and the field values is used in it's place in the
string.

    @template myNameIs `Hello, my name is {{.First}} {{.Last}}`

    person := { First: "Tom", Last: "Smith"}
    label , err := strings.Template( myNameIs, person )

    if err != nil {
        fmt.Println(err)
    } else {
        fmt.Println(label)
    }

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

### strings.ToLower(string)

The `ToLower()` function converts the characters of a string to the lowercase
version of that character, if there is one. If there is no lowercase for a
given character, the character is not affected in the result.

    a := "Mazda626"
    b := strings.ToLower(a)

In this example, the value of `b` will be "mazda626".

### strings.ToUpper(string)

The `ToUpper()` function converts the characters of a string to the uppercase
version of that character, if there is one. If there is no uppercase value for a
given character, the character is not affected in the result.

    a := "Bang+Olafsen"
    b := strings.ToUpper(a)

In this example, the value of `b` will be "BANG+OLAFSEN".

### strings.Tokenize(string)

The `Tokenize()` function uses the built-in tokenizer to break
a string into its tokens based on the _Ego_ language rules. The
result is passed back as an array.

    s := "x{} <- f(3, 4)"
    t := strings.Tokenize(s)

This results in `t` being a []string array, with contents ["x", "{}", "<-", "f", "(", "3", ",", "4", ")"].
Note that {} is considered a single token in the language, as is &lt;- so they each occupy a single
location in the resulting string array.

### strings.Truncate(string, len)

The `Truncate()` function will truncate a string that is too long, and add
the ellipsis ("...") character at the end to show that there is more information
that was not included.

    a := "highway bridge out of order"
    msg := strings.Truncate(a, 10)

In this example, the value of `msg` is "highway...". This is to ensure that the
resulting string is only ten characters long (the length specified as the second
parameter). If the string is not longer than the given count, the entire string is
returned.

### strings.URLPattern()

The `URLPattern()` function can be used in a web service to determine what parts of
a URL are present. This is particularly useful when using collection-style URL names,
where each part of the path could define a collection type, followed optionally by
an instance identifier of a specific member of tht collection, etc.  Consider
the following example:

    p := "/services/proc/{{pid}}/memory"
    u := "/services/proc/1553/memory"

    m := strings.URLPattern(u,p)

The `p` variable holds a pattern. It contains a number of segments of the URL, and
for one of them, specifies the indicator for a substitution value. That is, in this
part of the URL, any value is accepted and will be named "pid". The resulting
map generated by the call looks like this:

    {
        "services" : true,
        "proc": true,
        "pid": "1553",
        "memory": true,
    }

For items that are constant segments of the URL, the map contains a boolean value
indicating if it was found in the pattern. For the substitution operator(s) in the
pattern, the map key is the name from the pattern, and the value is the value from
the URL.  Note that this can be used to determine partial paths:

    p := "/services/proc/{{pid}}/memory"
    u := "/services/proc/"

    m := strings.URLPattern(u,p)

In this case, the resulting map will have a "pid" member that is an empty string,
and a "memory" value that is false, which indicates neither the substitution value
or the named field in the URL are present.

You an specify a pattern that covers an entire hierarchy, and the return will
indicate how much of the hierarchy was returned.

    p := "/services/location/state/{{stateName}}/city/{{cityName}}

If the supplied URL was `/services/location/state/nc/city/cary` then the map would
be:

    {
        "services" : true,
        "location" : true,
        "state" : true,
        "stateName" : "nc",
        "city" : true,
        "cityName" : "cary",
    }

But, if the url provided only had `services/location/state/nc` then resulting map
would be:

    {
        "services" : true,
        "location" : true,
        "state" : true,
        "stateName" : "nc",
        "city" : false,
        "cityName" : "",
    }

If the url did not include the state name field, that would be blank, which could
tell the service that a GET on this URL was meant to return a list of the state
values stored, as opposed to information about a specific state.

## sync <a name="sync"></a>

The `sync` package provides access to low-level primitive operations used to
synchronize operations between different go routine threads that might be
running concurrently.

### sync.Mutex

This is a type provided by the `sync` package, used to perform simple mutual
exclusion operations to control access to resources. The mutex can be locked
by a user, in which case any other thread's attempt to lock the item will
result in that thread waiting until the mutex is unlocked by the first owner.
Consider the following code:

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

        time.Sleep("1s")
        return 0
    }

As written above, the code will launch five go routines that will all do the same
simple operation -- increment the counter and then print it's value at the time the
go routine ran.  We know that go routines run in unpredictable order, but even if we
saw the numbers printed out of order, we would still see the counter values increment
as 1, 2, 3, 4, and 5.  

But, because the go routines are running simultaneously, between the time one routine
gets the value of counter, adds one to it, and puts it back, another routine could have
performed the same operation. This means we would over-write the value from the other
thread. In this case, the output of the count value might be more like this:

    thread 0, counter 1
    thread 4, counter 1
    thread 2, counter 2
    thread 3, counter 2
    thread 1, counter 3

To fix this, we use a mutex value to block access to the counter for each
thread, so they are forced to take turns incrementing the counter.

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

        time.Sleep("1s")
        return 0
    }

Now that there is a mutex protecting access to the counter, no matter what order the
go routines run, the increment of the count value will always be sequential, resulting
in output that might look like this:

    thread 4, counter 1
    thread 0, counter 2
    thread 2, counter 3
    thread 3, counter 4
    thread 1, counter 5

### sync.WaitGroup

This is a type provided by the `sync` package. You can declare a variable of this
type and a WaitGroup is created, and can be stored in a variable. This value is
used as a _counting semaphore_ and usually supports arbitrary numbers of go
routine starts and completions.

Consider this example code:

    func thread(id int, wg *sync.WaitGroup) {               [1]
        fmt.Printf("Thread %d\n", id)
        wg.Done()                                           [2]
    }
    
    func main() int {
        var wg sync.WaitGroup                               [3]
        
        count := 5
        for i := 1; i <= count; i = i + 1 {
            wg.Add(1)                                       [4]
            go thread(i, &wg)                               [5]
        }
        
        wg.Wait()                                           [6]
        
        fmt.Println("all done")
    }

This program launches five instances of a go routine thread, and waits for them
to complete. This simplified example does not return a value from the threads, so
channels are not used. However, the caller must know when all the go routines have
completed to know it is safe to exit this function. Note that if the code did not
include the `wg.Wait()` call then the `main` function would exit before most of the
go routines had a chance to complete.

Here's a breakdown of important steps in this example:

1. In this declaration of the function used as the go routine, a parameter is
   passed that is a __pointer__ to the WaitGroup variable. This is important
   because operations on the `WaitGroup` variable must be done on the same
   instance of that variable.

2. The `Done()` call is made by the go routine when it has completed all it's
   operations. This could also be implemented as

       defer wg.Done()

    to ensure that it is always executed whenever the function exits.

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

&nbsp;
&nbsp;

## tables <a name="tables"></a>

The tables package provides functions to help programs produce text tables of
data for output. The package allows you to create a table object with a given
set of column names. You can add rows to the table, sort the table, specify
formatting options for the table, and then generate a text or json version of
the table data.

### tables.New("colname" [, "colname"...])

This gives access to the table formatting and printing subsystem for Ego programs. The
arguments must be the names of the columns in the resulting table. These can be passed
as discrete arguments, or as an array of strings. The result is a TableHandle object
that can be used to insert data into the table structure, sort it, and format it for
output.

    t := tables.New(":Identity", "Age:", "Address")
    
    t.AddRow( {Identity: "Tony", Age: 61, Address: "Main St"} )
    t.AddRow( {Identity: "Mary", Age: 60, Address: "Elm St"} )
    t.AddRow( {Identity: "Andy", Age: 61, Address: "Elm St"} )
    
    t.Sort( "Age", "Identity" )
    
    t.Format(true,false)
    t.Print()
    t.Close()

This sample program creates a table with three column headings. The use of the ":"
character controls alignment for the column. If the colon is at the left or right
side of the heading, that is how the heading is aligned. If no colon is used, then
the default is left-aligned.

The data is added
for three rows. Note that data can be added as either a struct, where the column names
must match the structure field names. Alternatively, the values can be added as
separate arguments to the function, in which case they are added in the order of the
column headings.

The format of the table is further set by sorting the data by Age and then Identity, and
indicating that headings are to be printed, but underlines under those headings are not.
The table is then printed to the default output and the memory structures are released.

## time

The `time` package assist with functions that access or calculate time/date values. This
is similar to the "time" package in Go, but has significant differences and is not as
complete as the _Go_ version.  The `time.Now()` and `time.Parse()` functions each create
a new `time.Time` variable type, which has a set of functions that can be performed
on it.

| Function     | Example                    | Description                                                     |
|--------------|---------|------------------------------------------------------------------------------------|
| Add          | n := t.Add(nt)             | Add one time value to another.                                  |
| Format       | f := t.Format("Mon Jan 2") | Format the time value according to the reference time.          |
| SleepUntil   | t.SleepUntil()             | Pause execution until the time arrives.                         |
| String       | f := t.String()            | Convert the time value to a standard string representation.     |
| Sub          | n := t.Sub(start)            Subtract a a time value from another.                           |

A note about the `Format()` operator. The format string must be comprised of elements
from the reference time, which is a specific date value "Mon Jan 2 15:04:05 -0700 MST 2006".
This value is also available as `time.reference` if you need to refer to it. Each part of the
date has a unique value, so the `Format()` call in the table above will print the day of the
week, the month, and the day since those are the values used from the reference string in the
format specification.

### time.Now()

The `Now()` function gets the current time at the moment of the call, and sets it as the time value in the
result.

    now := time.Now()
    ...
    elapsed := time.Now().Sub(now)

In this case, the code first captures the current time and stores it in the variable `now`. It then does some other
work for the program, and when done we want to find out the elapsed time. The value of `elapsed` is a duration string
that indicates how much time passed between the `now` value and the current time. For example, this could be a
value such as "5s" for five seconds of time passing.

### time.Parse(string, model)

This converts a text representation of a time into a time value. The first parameter is the text to convert,
and the second parameter is the "model" which describes the format in which the value is parsed. This uses the
same specific date values from thee `time.reference` time.

    s := "12/7/1960 15:30"
    m := "1/2/2006 15:04"
    t := time.Parse(s, m)

The time value stored in `t` is the time value "Wed Dec 7 15:30:00 UTC 1960". Note that the model showed a month
of 1 (for January) but was still using the specific values from the reference time. The slashes are not part of
the reference time, so they must exist in the same locations in the string to be converted and will be skipped.

IF there is an error in parsing, the value of `t` will be `nil`. You can find out what the exact error is by
allowing the `time.Parse()` function to return two values:

    t, e := time.Parse(s, m)

If the value of `t` is `nil` then the value of `e` will be the error code that reflects the parsing error. If
the call is made with only one return value specified, then the error is discarded.

### time.Sleep(duration)

The `Sleep()` function of thee `time` package will sleep for the specified amount of time. The duration
is expressed as a string. For example,

    time.Sleep("10s")

This will sleep for ten seconds. The suffix can be "h", "m", or "s" and can include fractional values. While
the system is sleeping, go routines will continue to run but the current program (or go routine) will stop
executing for the given duration.

## util <a name="util"></a>

The `util` package contains miscellaneous utility functions that may be convenient
for developers writing _Ego_ programs.

### util.Memory()

The `Memory()` function returns a strutcure summarying current user memory consumption,
total consumption for the life of the program, system memory on behalf of the _Ego_
processes, and a count of the number of times the garbage collector that manages
memory for Ego has been run.

    ego> fmt.Println(util.Memory())
    { current: 0.9879989624023438, gc: 0, system: 68.58106994628906, time: "Thu Apr 22 2021 10:07:36 EDT", total: 0.9879989624023438 }


The result of the function is always a structure. The `current` and `system` values are expressed in megabytes; so
in the above example, the current memory consumption by the system on behalf of Ego is 68MB and the user memory
consumed by Ego on behalf of the user is just under 1MB. The value of `total` is the total amount of memory
ever allocated by Ego; this number will rise throughout the life of the program, but each time the memory
reclaimation thread (garbage collector) runs, it will reclaim unused memory and reude the `current` value
accordingly. You can use the `gc` field as a count of the number of times the garbage collector has run.

### util.Mode()

The `Mode()` function reports the mode the current program is running under.
The list of values are:

&nbsp;

| Mode | Description |
| ---- | ----------- |
| interactive | The `ego` program was run with no program name, accepting console input |
| server      | The program is running under control of an _Ego_ rest server as a service |
| test        | The program is running using the `ego test` integration test command |
| run         | The program is running using `ego run` with an input file or pipe |

&nbsp;
&nbsp;

### util.Symbols()

The `Symbols()` function generates a report on the current state of the active
symbol table structure. This prints the symbols defined in each scope (including
statement blocks, functions, programs, and the root symbol table). For each one,
the report includes the number of symbol table slots used out of the maximum
allowed, and then a line for each symbol, showing it's name, type, and
value.

    fmt.Println(util.Symbols())

Note that symbols that are internal to the running of the program are
not displayed; only symbols created by the user or for defined packages
are displayed.

## uuid <a name="uuid"></a>

The `uuid` package provides support for universal unique identifiers. This is an
industry-standard way of creating an identifier that is (for all practical purposes)
guaranteed to be unique, even among different instances of `ego` running on different
computers. A `uuid` value is a string, consisting of groups of hexadecimal values,
where each group is separated by a hyphen. For example, "af315ffd-6c57-46b9-af62-4aac8ba5a212".

### uuid.New()

The `New()` function generates a new unique identifier value, and returns the result
as a string.

    id := uuid.New()

### uuid.Nil()

The `Nil` function generates the _zero value_ for a UUID, which is a UUID that consists
entirely of zeroes. This value will never be generated by the `New()` function and will
never match another UUID value.

    if id == uuid.Nil() {
        fmt.Println("id value was not set")
    }

### uuid.Parse(string)

The `Parse()` function is used to parse and validate a UUID string value. This is useful
for string values received via REST API calls, etc. The `Parse()` function returns
two value; the result of the parse and an error to indicate if the parse was
successful.

    id, err := uuid.Parse(uuidString)

If the string variable `uuidString` contains a valid UUID specification, then it is
stored in `id` (case normalized) and the `err` variable is nil. But if there was an
error, the `id` will be nil, and the `err` will describe the error.

&nbsp;
&nbsp;

# User Packages

You can create your own packages which contain type definitions and
functions that are used via the package prefix you specify.  Consider
the following example files.

The first file is "employee.ego" and describes a package. It starts
with a `package` statement as the first statement item, and then
defines a type and a function that accepts a value of that type as
the function receiver.

    package employee

    type Employee struct {
        id int
        name string
    }

    func (e *Employee) SetName( s string ) {
        e.name = s
    }

The second file is "test.ego" and is the program that will use this package.
It starts with an `import` statement, which causes the compilation to include
the package definition within the named file "employee". You can specify the
file extension of ".ego" but it is not necessary.

    import "employee"

    e := employee.Employee{id:55}

    e.SetName("Frodo")

    fmt.Println("Value is ", e)

This program uses the package definitions. It creates an instance of an
`Employee` from the `employee` package, and initializes one of the fields
of the type. It then uses the object to invoke a function for that type,
the `SetName` package. Note that when this is called, you do not specify
the package name; instead you specify the object that was created using
the package's definition. In this example, it should print the structure
contents showing the `id` of 55 and the `name` of "Frodo."

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

&nbsp;
&nbsp;

# Directives <a name="directives"></a>

Directives are special _Ego_ statements that perform special functions
outside the normal language syntax, often to influence the runtime
environment of the program or give instructions to the compiler itself.

## @error <a name="at-error"></a>

You can generate a runtime error by adding in a `@error` directive,
which is followed by a string expression that is used to formulate
the error message text.

    v = "unknown"
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

## @localization <a name="at-localization"></a>
The `@localization` directive defines localized string properties for any
supported language in the current Ego program. The directive stores data in
the localization properties dictionary, which can be accessed using the
i18n.T() function. The localization is defined using structure notation,
with a field for each language. Within each language is are fields for
each message property. The property name is the field name (which can be
in double quotes if it is not a valid identifier) and the value is the
loacalized string.


    @localization {
        "en": {
            "hello.msg": "hello, {{.Name}}",
            "goodby.msg": "goodbye"
        },
        "fr": {
            "hello.msg": "bonjour, {{.Name}}",
            "goodbye.msg": "au revoir"
        },
        "es": {
            "hello.msg": "hola, {{.Name}}",
            "goodbye.msg":"adios"
        }
    }

    func main{
        m := i18n.T("hello.msg", {Name: "Tom"})
        fmt.Println(m)
    }

There can be only on `@localization` specification in a given program.
It can appear before or after the functions in the program (it is 
processed during compilation).

Use the `i18n.T()` function to get the localized string value. In the
above example, the optional second argument is used, which contains a
parameter map for each item called out in the message text. Note that the
message text is compiled and executed as a template, so you can reference
the named values but also generate loops, etc. as needed.

An optional third argument indicates the language code ("en", "fr", "es",
etc.) to use. If omitted, the current session's language is used. In the
case of a web service, the sevice may wish to ascertain the caller's language
to provde language-specific web results.

## @template <a name="at-template"></a>

You can store away a named Go template as inline code. The template
can reference any other templates defined.

    @template hello "Greetings, {{.Name}}"

The resulting templates are available to the template() function,
 whose first parameter is the template name and the second optional
 parameter is a record containing all the named values that might
 be substituted into the template.  For example,

     print strings.template(hello, { Name: "Tom"})

This results in the string "Greetings, Tom" being printed on the
stdout console. Note that `hello` becomes a global variable in the program, and
is a pointer to the template that was previously compiled. This
global value can only be used with template functions.

## @type static|dynamic <a name="at-type"></a>

You can temporarily change the language settings to allow static
typing of data only. When in static mode,

* All values in an array constant must be of the same type
* You cannot store a value in a variable of a different type
* You cannot create or delete structure members

This mode is effective only within the current statement block
(demarcated by "{" and "}" characters). When the block finishes,
type enforcement returns to the state of the previous block. This
value is controlled by the static-types preferences item or
command-line option.
