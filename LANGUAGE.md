
# Table of Contents
1. [Introduction](#intro)
1. [Data Types](#datatypes)
    1. [Base Types](#basetypes)
    2. [Arrays](#arrays)
    2. [Structures](#structures)
    3. [User Types](#usertypes)
2. [Symbols and Expressions](#symbolsexpressions)
    1. [Symbols and Scope](#symbolsscope)
    2. [Operators](#operators)
    3. [Type Conversion](#typeconversion)
    4. [Builtin Functions](#builtinfunctions)
3. [Conditional and Iterative Execution](#flowcontrol)
    1. [If/Else Conditional](#if)
    2. [For &lt;condition&gt;](#forcond)
    3. [For &lt;index&gt;](#forindex)
    4. [For &lt;range&gt;](#forrange)

&nbsp;
&nbsp;


# Introduction to _Ego_ Language<a name="intro"></a>
_Version 0.1_

This document describes the language _Ego_, which is a scripting language and tool set patterned off of the _Go_ programming language. The _Ego_ language name is a portmanteaux for _Emulated Go_. The data types and language statements are very similar to _Go_ with a few exceptions:
* If enabled by settings, _Ego_ offers a try/catch model for intercepting runtime errors
* The language can be run with either dynamic or static typing.
* The available set of packages that support runtime functionality is limited.

The _Ego_ language is run using the `ego` command-line interface. This provides the ability to run a program from an external text file, to interactivly enter _Ego_ programming statements, or to run _Ego_ programs as web services. This functionality is documented elsewhere; this guide focusses on writing _Ego_ programs regardless of the runtime environment.

The _Ego_ language is Copyright 2020, 2021 by Tom Cole, and is freely available for any use, public or private, including commercial software. The only requirement is that any software that incorporates or makes use of _Ego_ or the packages written by Tom Cole to support must include a statement attributing authorship to _Ego_ and it's runtime environment to Tom Cole.


# Data Types<a name="datatypes"></a>
The _Ego_ language supports a number of base types which express a single value of some type (string, integer, boolean, etc.). These base types can be members of complex types consisting of arrays (ordered lists) and structs (key/value pairs). Additionally, the user can create types based on the base or complex types, such as a type describing a structure that records information about an employee; this type can be used to create instances of the structure, etc.

## Base Types<a name="basetypes"></a>

A value can be a base type; when it is a base type is contains only one value at a time, and that value has a specific type.  These are listed here.

|   Type   | Example  |    Range         | Description  |
| -------- | -------- | ---------------- | -------------|
| `nil`    | nil      | nil | The `nil` value indicates no value or type specified |
| `bool`   | true     | true, false      | A Boolean value that is either true or false |
| `int`    | 1573     | -2^63 to 2^63 -1 | A 64-bit integer value |
| `float`  | -153.35  | -1.79e+308 to 1.79e+308 | A 64-bit floating point value |
| `string` | "Andrew" | any              | A string value, consisting of a varying number of Unicode characters |
| `chan`   |  chan    | any              | A channel, used to communicate values between threads |

_Note that the numeric range values shown are approximate._

A value expressed in an _Ego_ program has an implied type. The language processor will attempt to determine the type of the value. For booelan values, the value can only be `true` or `false`. For numeric types, the language differentiates between integer and float value. The value `1573` will be intepreted as an int value because it has no exponent or factional part, but `-153.35` will be interpreted as a float value because it has a decimal point and a fractional value. A string value enclosed in double quotes (") cannot span multiple lines of text. A string value enclosed in back-quotes (`) are allowed to span multiple lines of text if needed.

A `chan` value has no constant expression; it is a type that can be used to create a variable used to communicate between threads. See the section below on threads for more information.

## Arrays<a name="arrays"></a>

An array is an ordered list of values. That is, it is a set where each value has a numerical position referred to as it's index. The first value in an array has an index value of 0; the second value in the array has an index of 1, and so on. An array has a fixed size; once it is created, you cannot add to the array directly.

Array constants are expressed using square brackets, which contain a list of values separated by commas. The values may be any valid value (base, complex, or user types).  The values do not have to be of the same type. For example,

    [ 101, 335, 153, 19, -55, 0 ]
    [ 123, "Fred", true, 55.738]

The first example is an array of integers. The value at position 0 is `101`. The value at position 1 is `335`, and so on.  The second example is a heterogenous array, where each value is of varying types. For example, the value at position 0 is the integer `123` and the value at position 1 is the string `"Fred"`.


## Structures<a name="structures"></a>
A structure (called `struct` in the _Ego_ language) is a set of key/value pairs. The key is an _Ego_ symbol, and the value is any supported value type. Each key must be unique. The values can be read or written in the struct based on the key name. Once a struct is created, it cannot have new keys added to it directly. A struct constant is indicated by braces, as in:

    {  Name: "Tom", Age: 53 }

This struct has two members, `Name` and `Age`. Note that the member names (the keys of the key/value pair) are case-sensitive. The struct member `Name` is a string value, and the struct member `Age` is an int value.

## User Types<a name="usertypes"></a>
The _Ego_ language includes the ability to create use-defined types. These are limited to `struct` definitions. They allow the program to define a short-hand for a specific type, and then reference that type when creating a new variable of that type. The `type` statement is used to define the type. Here is an example:

    type Employee struct {
       Name    string
       Age     int
    }

This creates a new type called `Employee` which is a struct with two members, `Name` and `Age`. A variable created with this type will always be a struct, and will always contain these two members. The use of this statement will be more clear later when we see how to create a variable of a given type.

# Variables and Expressions<a name="symbolsexpressions"></a>
This section covers variables (named storage for values) and expressions (sequences of variables, values, and operators that result in a computed value).

## Symbols and Scope<a name="symbolsscope"></a>
A variable is a named storage location, identified by a _symbol_. The _Ego_ language is, by default, a case-senstive language, such that the variables `Age` and `age` are two different values. A symbol names can consist of letters, numbers, or the underscore ("_") character. The first character must be either an underscore or a alphabetic character. Here are some examples of valid and invalid names:

| Name | Description |
| ---- | ----------- |
| a123 | Valid name |
| user_name | Valid name |
| A123 | Valid name, different than `a123`|
| _egg | Valid name, but is a read-only variable |
| 15States | Invalid name, does not start with an alphabetic character |
| $name | Invalid name, `$` is not a valid symbol character |

There is a reserved name that is just an underscore, "_". This name means _value we will ignore._ So anytime you need to referene a variable to conform to the syntax of the language, but you do not want or need the value for your particular program, you can specify "_" which is a short-hand value for "discard this value".

A symbol name that starts with an underscore character is a read-only variable. That is, it's value can be set once when it is created, but can not be changed once it has its initial value. For example, when an _Ego_ program runs, there is always a read-only variable called `_version` that can be read to determine the version of the `ego` command line tool, but the value cannot be set by a user program.

The term _scope_ refers to the mechanism by which a symbol name is resolved to a variable. When an _Ego_ program runs, each individual function, and in fact each basic block (code enclosed within `{...}` braces) has its own scope. A variable that is created at one scope is _visible_ to any scopes that are contained within that scope. For example, a variable created at the first line of a function is visible to all the code within the function. A variable created within a basic-block is only visible within the code inside that basic block. When a scope ends, any variables created in that scope are deleted and they no longer have a value.

This will become clearer below when we talk about functions and basic blocks. In general, just know that you can reference (read or write) a value to any symbol in your same scope or an outer scope, but you can only create variables at the current scope, and once that scope is completed, the created variables no longer exist.
A symbol can be assigned a value using an _assignment_ statement. This consists of the variable name that is to receive the value, and either `=` or `:=`, followed by the value to store in that variable. The difference between the two assignment operators is that `:=` will create a new value in the current scope, while `=` will locate a value that already exists, and write a new value to it.

      name := "Bob"
      name = "Mary"

In this example, the first statement uses the `:=` operator, which cause the symbol `name` to be created, and then the string value "Bob" is stored in the variable. If the variable already exists, this is an error and you should use the `=` instead to update an existing value. The `=` operator looks for the named value in the current scope, and if it finds it, the value "Mary" is assigned to it. If the variable does not exist at this scope, but does in an outer scope level, then the variable at the outer scope is updated.


## Operators<a name="operators"></a>
Operators is the term for language elements that allow you to perform mathmatical or other other operations using constant values as well as variable values, to produce a new computed value. Some operators can operate on a wide range of different value types, and some operators have more limited functionality.

There are _dereference_ operators that are used to access members of a struct, values of a type, or index
into an array.

| Operator | Example | Description |
| --- | --- | --- |
| .   | emp.age | Find the menber named `age` in the struct named `emp` |
| []  | items[5] | Find the value at index 5 of the array named `items` |
| {}  | emp{} | Create an instance of a struct of the type `emp` |
&nbsp;
&nbsp;

There are _monadic_ operators which precede an expression and operate on the single value given.
| Operator | Example | Description |
| --- | --- | --- |
|  -  | -temp | Calculate the negative of the value in `temp` |
| !  | !active | Calculate the boolean NOT of the value in `active` |
&nbsp;
&nbsp;

There are _diadic_ operators that work on two values, one of which preceeds the operator and one of which follows the operator.

| Operator | Example | Description |
| --- | --- | --- |
|  +  | a+b | Calculate the sum of numeric values, or the AND of two boolean values. For strings, concatenate the strings |
| - | a-b | Calculate the difference of the integer or float values |
| * | a*b | Calculate the product of the numeric value, or the OR of two boolean values |
| / | a/b | Calculate the division of the numeric values |
| ^ | 2^n | Calculate `2` to the power `n` |

For division, integer values will result in the integer value of the division, so `10/3` will result in `3` as the expression value. A floating point value retains the fractional value of the conversion, so `10.0/3.0` results in `3.333333333` as the result.

Expressions can be combined together, and follow normal mathematical order of precidence (multiplication and division are done before subtraction and addition). So the expression `a+b*c` will first multiply `b` and `c`, and then add the product to the value of `a`. You can use parenthesis to control the order of evaluation, so `(a+b)*c` will calculate the sum of `a+b` and then multiply that result by the value in `c`.
&nbsp;
&nbsp;

There are _relational_ operators that work on two values, one of which preceeds the operator and one of which follows the operator. The result of the operator is always a boolean (`true` or `false`) value describing the relationship between the two values.

| Operator | Example | Description |
| --- | --- | --- |
|  ==  | a == b | True if `a` is equal to `b` |
|  !=  | a != b | True if `a` is not equal to `b` |
|  &gt; | a &gt; b | True if `a` is less than `b` |
|  &gt;= | a &gt;= b | True if `a` is less than or equal to `b` |
|  &lt; | a &lt; b | True if `a` is greater than `b` |
|  &lt;= | a &lt;= b | True if `a` is greater than or equal to `b` |


## Type Conversions<a name="typeconversion"></a>
When an operation is done on two values (either a variable or a constant value) of the same type, no additional conversion is performed or required. If the operation is done on two values of different types, _Ego_ will convert the values to the same type if possible before performing the operation. For example, the expression `10.0/3` divides an integer value into a floating point value; the _Ego_ languae converts the integer to a floating point value before performing the division. In general, _Ego_ will convert the values to the value that will result in minimal or no loss of precision.

These conversions happen automatically, though you can use type casting functions like `int()` or `string()` discussed later to force a specific type of conversion operation. For example, if a boolean value is used in an expression that requires a float value, then the boolean is converted such that `false` is converted to `0.0` and `true` is converted to `1.0`. Similarly, if a numeric or boolean value is needed as a string, the string value is the formatted version of the original value. So a value of `123.5` as a float becomes the string `"123.5"`.

## Builtin Functions<a name="builtinfunctions"></a>

The _Ego_ language includes a library of built-in functions which can also be used as elements of an expression, including having the function value be assigned to a variable. A function consists of a name, followed by a list of zero or more values in parenthesis, separated by commas. If there are no values, you still must specify the parenthesis. The function may accept a fixed or variable number of arguments, and typically returns a single value.

| Function | Example               | Description |
| -------- | --------------------- | ----------- |
| append() | append(list, 5, 6, 7) | Append the items together into an array. |
| array()  | array(list, 5)        | Create a new array using the values of `list` that is `5` elements long |
| bool()   | bool(55)              | Convert the value to a boolean, where zero values are false and non-zero values are true |
| close()  | close(sender)         | Close a channel. See the information on [Threads](#threads) for more info. |
| delete() | delete(emp, "Name")   | Remove the struct member Name from the `emp` struct |
| error()  | error("panic") | Generate a runtime error named "panic". |
| eval()   | eval("3 + 5")  | Evaluate the expression in the string value, and return the result, `8` |
| float()  | float(33)      | Convert the value to a float, in this case `33.0` |
| index()  | index(items, 55) | Return the array index of `items` that contains the value `55` |
| int()    | int(78.3)      | Convert the value to an integer, in this case `78` |
| len()    | len(items)     | If the argument is a string, return its length in characters. If it is an array, return the number of items in the array |
| make()   | make([]int, 5) | Create an array of int values with `5` elements in the array |
| max()    | max(11, 17, 3) | Return the mathmatically greatest value in the argument list, in this case `17`. |
| members() | members(emp)  | Return an array of strings containing the struct member names of the argument |
| min()    | min(5,2,33)    | Return the mathmatically smalles value in the argument list, in this case `2` |
| sort()   | sort(items)    | Sort the array members of `items` from smallest to largest values |
| string() | string(true)   | Convert the argumennt to a string value, in this case `true` |
| sum()    | sum(5,6,3)     | Return the arithmetic sum of the values, in this case `14` |
| type()   | type(emp)      | Return a string with the type of the argument. If emp is a struct, the result will be `"struct"` |
&nbsp;
&nbsp;

# Conditional and Iterative Execution <a name="flowcontrol"></a>
We have discussed how variables are created, and how expressions are used to calculate values based on variables, constant values, and functions. However, most interesting programs require some decision making to control the flow of execution based on the values of variables. This section will describe how to make _either/or_ decisions in the code, and how to execute a block of code repeatedly until a condition is met.

## If-Else <a name="if"></a>

The general nature of a conditional `if` statement is

     if <condition> 
         <statement>
     else 
         <statement>

The `else` clause is optional, as described below. In all cases where the syntax says &lt;statement&gt;, it can be a single statement or a basic block which is a set of statements enclosed in braces ("{" and "}"). By convention, even when there is only a single statement in the block, a basic block is used for readability.

Consider the following example code:

    salary := hours * wage                  (1)
    if salary < 100.0 {                     (2)
        fmt.Println("Not paid enough!")     (3)
    }                                       (4)
    total = total + salary                  (5)
This introduces a number of new elements to a program, so let's go over them line-by-line. The numbers is parenthesis are not part of the program, but are used to identify each line of the code.

1. This first line calculates a new value by multiplying the `hours` times the `salary`, and store it in a new value called `salary`. This uses an assignment statement; the `:=` indicates the variable does not already exist and will be created by this operation. We assume the values of `hours` and `wage` were calculated already in the program.

2. This statement performs a conditional evaluation. After the `if` statement, there is a relational expresssion thta can be converted to a boolean value. In this case, if `salary` has a value less than `100.0` then the code will execute some additional statement(s). After the expression, the `{` character defines the start of a _basic block_ which is a group of statements that are all executed together. 

3. If salary is less than 100.0, then the fmt.Println() operation is performed. Don't worry that we haven't talked about this yet; its covered below in the section on the `fmt` package, but it is enough to know that this produces a line of output with the string `Not paid enough!`. If, however, the value of `salary` is not less than 100.0, then the basic block is not executed, and the program continues with the statement after the block. 

4. The `}` character defines the end of the basic block. If the `if` statement condition is not met, execution continues after this end-of-block indicator.

5. This statement will be executed next, regardless of whether `salary` was less than 100.0 or not. This statement updates the value of `total` to be the sum of `total` and the `salary`.

Instead of having the `if` statement advance to the next statement if the condition is not true, a second _basic block_ can be defined that has the statements to execute if teh condition is false. That is, the result of the expression will result in one or the other of two basic blocks being executed.

    salary := hours * wage
    if salary < 100 {
        scale = "small"
    } else {
        scale = "large"
    }


In this example, after calculating a value for `salary`, it is compared to see if it is less than 100. If so, then the value `scale` is set to the value `"small"`. But if the value of `salary` is not less than 100, the value of `scale` is set to `"large"`. Regardless of which basic block was executed, after the block executes, the program resumes with the next statement after the `if` statements.

## For _condition_ <a name="forcond"></a>

## For _index_ <a name="forindex"></a>

## For _range_ <a name="forrange"></a>

# User Function Definitions

## The `func` Statement

## The `return` Statement

# Threads

# Error Handling

## `try` and `catch`

## Signalling Errors

# Builtin Packages

## io

## math

## profile

## rest

## string

## time

## util



