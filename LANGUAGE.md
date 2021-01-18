# Introduction to _Ego_ Language
_Version 0.1_

This document describes the language _Ego_, which is a scripting language and tool set patterned off of the _Go_ programming language. The _Ego_ language name is a portmanteaux for _Emulated Go_. The data types and language statements are very similar to _Go_ with a few exceptions:
* If enabled by settings, _Ego_ offers a try/catch model for intercepting runtime errors
* The language can be run with either dynamic or static typing.
* The available set of packages that support runtime functionality is limited.

The _Ego_ language is run using the `ego` command-line interface. This provides the ability to run a program from an external text file, to interactivly enter _Ego_ programming statements, or to run _Ego_ programs as web services. This functionality is documented elsewhere; this guide focusses on writing _Ego_ programs regardless of the runtime environment.

The _Ego_ language is Copyright 2020, 2021 by Tom Cole, and is freely available for any use, public or private, including commercial software. The only requirement is that any software that incorporates or makes use of _Ego_ or the packages written by Tom Cole to support must include a statement attributing authorship to _Ego_ and it's runtime environment to Tom Cole.

# Table of Contents
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

&nbsp;
&nbsp;

# Data Types<a name="datatypes"></a>
The _Ego_ language supports a number of base types which express a single value of some type (string, integer, boolean, etc.). These base types can be members of complex types consisting of arrays (ordered lists) and structs (key/value pairs). Additionally, the user can create types based on the base or complex types, such as a type describing a structure that records information about an employee; this type can be used to create instances of the structure, etc.

## Base Types

A value can be a base type; when it is a base type is contains only one value at a time, and that value has a specific type.  These are listed here.

|  Type  | Example  |    Range         | Description  |
| ------ | -------- | ---------------- | -------------|
| `bool`   | true     | true, false      | A Boolean value that is either true or false |
| `int`    | 1573     | -2^63 to 2^63 -1 | A 64-bit integer value |
| `float`  | -153.35  | -1.79e+308 to 1.79e+308 | A 64-bit floating point value |
| `string` | "Andrew" | any              | A string value, consisting of a varying number of Unicode characters |
| `chan` |  chan      | any | A channel, used to communicate values between threads |
(Note that the range values shown are approximate.)

A value expressed in an _Ego_ program has an implied type. The language processor will attempt to determine the type of the value. For booelan values, the value can only be `true` or `false`. For numeric types, the language differentiates between integer and float value. The value `1573` will be intepreted as an int value because it has no exponent or factional part, but `-153.35` will be interpreted as a float value because it has a decimal point and a fractional value. A string value enclosed in double quotes (") cannot span multiple lines of text. A string value enclosed in back-quotes (`) are allowed to span multiple lines of text if needed.

A `chan` value has no constant expression; it is a type that can be used to create a variable used to communicate between threads. See the section below on threads for more information.

## Arrays

An array is an ordered list of values. That is, it is a set where each value has a numerical position referred to as it's index. The first value in an array has an index value of 0; the second value in the array has an index of 1, and so on. An array has a fixed size; once it is created, you cannot add to the array directly.

Array constants are expressed using square brackets, which contain a list of values separated by commas. The values may be any valid value (base, complex, or user types).  The values do not have to be of the same type. For example,

    [ 101, 335, 153, 19, -55, 0 ]
    [ 123, "Fred", true, 55.738]

The first example is an array of integers. The value at position 0 is `101`. The value at position 1 is `335`, and so on.  The second example is a heterogenous array, where each value is of varying types. For example, the value at position 0 is the integer `123` and the value at position 1 is the string `"Fred"`.


## Structures

## User Types

# Symbols and Expressions<a name="symbolsexpressions"></a>

## Symbols and Scope

## Operators

## Type Conversions

## Builtin Functions

# Conditional and Iterative Execution

## If-Else

## For _condition_

## For _index_

## For _range_

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



