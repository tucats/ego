
# Table of Contents
1. [Introduction](#intro)
1. [Data Types](#datatypes)
    1. [Base Types](#basetypes)
    2. [Arrays](#arrays)
    2. [Structures](#structures)
    3. [User Types](#usertypes)
2. [Symbols and Expressions](#symbolsexpressions)
    1. [Symbols and Scope](#symbolsscope)
    2. [Constants](#const)
    2. [Operators](#operators)
    3. [Type Conversion](#typeconversion)
    4. [Builtin Functions](#builtinfunctions)
3. [Conditional and Iterative Execution](#flowcontrol)
    1. [If/Else Conditional](#if)
    2. [For &lt;condition&gt;](#forcond)
    3. [For &lt;index&gt;](#forindex)
    4. [For &lt;range&gt;](#forrange)
    5. [Break and Continue](#breakcont)
4. [User Functions](#userfuncs)
    1. [The `func` Statement](#funcstmt)
    2. [The `return` Statement](#returnstmt)
    3. [The `defer` Statement](#deferstmt)
    4. [Function Variables](#funcvars)
    5. [Function Receivers](#funcreceivers)
5. [Error Handling](#errors)
    1. [Try and Catch](#trycatch)
    2. [Signalling Errors](#signalling)


&nbsp;
&nbsp;


# Introduction to _Ego_ Language<a name="intro"></a>
_Version 0.2_

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

You can also create a variable using the `var` statement, which is followed by a comma-separated list of names and finally a type value. The variables are all created and set to the given type, with an appropriate "zero value" for that type.

      var first, last string
      var e1 Employee{}

The second example creates a variable based on a user-defined type `Employee`.  The {} characters causes an instance of that type to be created and stored in the named variable `e1` in this example. The {} charaacters can contain field initializations for the type, such as

      var e2 Employee{ Name: "Bob", Age: 55}

The type of `e2` is `Employee` and it contains initialized values for the permitted fields for the type. If the initializer does not specify a value for all fields, the fields not explicitly named are set to zero values for their types.


## Constants <a name="const"></a>
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
The simplest form of iterative execution (also referred to as a "loop") is the `for` statement, followed by a condition, and a statement or basic block that is executed as long as the condition is true.

     for <condition>
        <statement>

Here is an example

    value := 0                  (1)
    for value < 5 {             (2)
        fmt.Println(value)      (3)
        value = value + 1       (4)
    }                           (5)

1. This line initializes variable `value` to an integer zero.
2. The `for` statement specifies that as long as the `value` variable contains a number less than `5` the following basic block will be executed over and over.
3. This line of the basic block prints the current `value`
4. This line increments the variable by adding one to itself. This causes `value` to increase each time the loop runs.
5. This is the end of the basic block describing the body of the loop that will execute until the condition is no longer true.

This program results in the numbers `0` through `4` being printed to the output. When `value` is incremented to be the value `5`, the condition on line #2 is no longer true, and the loop stops executing, and the program will continue with whatever follows line 5.

Note that in this example, without line number 4 the program would run forever, because the variable `value` would be initialized to zero but then never change, so the condition will always be true.


## For _index_ <a name="forindex"></a>
You can create a `for` loop that explicitly specifies an expression that defines the starting value, ending condition, and how the value changes with each iteration of a loop. For example, 

     for i := 0; i &lt; 10; i = i + 1 {
         fmt.Println(i)
     }

This loop will print the values `0` through `9` to the standard console. The index variable `i` is first initialized with the value `0` from the first `for` statement clause. The second statement clause describes the condition that must be true for the loop body to be executed. This is evaluated before the loop body is run each time. The third clause is the statement that modifies the index value _after_ the body of the loop is run but _before_ the  next evaluation of the clause that determines if the loop continues.

The variable `i` in the above example is scoped to the `for` statement and it's loop body. That is, after this loop runs, the variable `i` will no longer exist because it was created (using the `:=` operator) in the scope of the loop. You can use a simple assignment (`=`) of an existing variable if you want the updated index value available after the loop body ends.

    var i int
    for i = 0; i &lt; 10; i = i + 1 {
        fmt.Println(i)
    }
    fmt.Println("The final value of i is ", i)

This example uses a variable that already exists outside the scope of the `for` loop, so the value continues to exist after the loop runs. This allows the final statement to print the value of the index variable that terminated the loop.

## For _range_ <a name="forrange"></a>
You can create a loop that indexes over all the values in an array, in sequential order. The index value is the value of the array element. For example,

     ids := [ 101, 143, 202, 17]
     for i := range ids {
         fmt.Println(i)
     }

This example will print a line for each value in the array, in the order they appear in the array. During execution of the loop body, the value of `i` (the _index_ variable)` contains the next value of the array for each iteration of the loop.  You can also specify a second value, in which case the loop defines an index number as well as index value, as in:

    for i, v := range ids {
        fmt.Println("Array member ", i, " is ", v)
    }

In this example, for each iteration of the loop, the variable `i` will contain the numerical array index  and the variable `v` will contain the actual values from the array for each iteration of the loop body.

## `break` and `continue` <a name="breakcont"></a>
Sometimes when running an loop, you may wish to change the flow of execution in the loop based on conditions unrelated to the index variable. For example, consider:

    for i := 1; i &lt; 10; i = i + 1 {
        if i == 5 {                      (1)
            continue                     (2)
        }
        if i == 7 {                      (3)
            break                        (4)
        }
        fmt.Println("The value is ", i)  (5)
    }
    
This loop will print the values 1, 2, 3, 4, and 6. Here's what each statement is doing:
1. During each execution of the loop body, the index value is compared to 5. If it is equal to 5 (using the `==` operator), the conditional statment is executed.
2. The `continue` statement causes control to branch back to the top of the loop. The index value is incremented, and then tested again to see if the loop should run again. The `continue` statement means "stop executing the rest of this loop body, but continue looping".
3. Similarly, the index is compared to 7, and if it equal to 7 then the conditional statment is executed.
4. The `break` statement exits the loop entirely. It means "without changing any of the values, behave as if the loop condition had been met and resume execution after the loop body.

&nbsp;
&nbsp;


# User Function Definitions <a name="userfuncs"></a>
In addition to the [Builtin Functions](#builtinfunctions) listed previously, the user program can create functions that can be executed from the _Ego_ program. Just like variables, functions have scope, and can only be accessed from within the program in which they are declared. Most functions are defined in the program file before the body of the program.

## The `func` Statement <a name="funcstmt"></a>

Use the `func` statement to declare a function. The function must have a name, optionally a list of parameter values that are passed to the function, and a return type that indicates what the function is expected to return. This is followed by the function body which is executed, with the function arguments all available as local variables.  For example,

    func addem( v1 float, v2 float) float {
        x := v1 + v2
        return x
    }

    // Calculate what a variable value (10.5) added to 3.8 is
    a := 10.5
    x := addem(a, 3.8)
    fmt.Println("The sum is ", x)

In this example, the function `addem` is created. It accepts two parameters; each is of type float in this example. The parameter values actually passed in by the caller will be stored in local variabels v1 and v2. The function defintiion also indicates that the result of the function will also be a float value.

Parameter types and return type cause type _coercion_ to occur, where values are converted to the required type if they are not already the right value type. For example,

    y := addem("15", 2)

Would result in `y` containing the floating point value 17.0. This is because the string value "15" would be converted to a float value, and the integer value 2 would be converted to a float value before the body of the function is invoked. So `type(v1)` in the function will return "float" as the result, regardless of the type of the value passed in when the function was called.  

The `func` statement allows for a special data type `interface{}` which really means "any type is allowed" and no conversion occurs. If the function body needs to know the actual type of the value passed, the `type()` function would be used.

## The `return` Statement  <a name="returnstmt"></a>
When a function is ready to return a value the `return` statement is used. This identifies an expression that defines what is to be returned. The `return` statement results in this expression being _coerced_ to the data type named in the `func` statement as the return value.  If the example above had a `string` result type,

    func addem( v1 float, v2 float) string {
        x := v1 + v2
        return x
    }

    y := addem(true, 5)

The resulting value  for `y` would be the string "6". This is because not only will the boolean `true` value and the integer 5 be converted to floating point values, bue the result will be converted to a string value when the function exits.

Note that if the `func` statement does not specify a type for the result, the function is assumed not to return a result at all. In this case, the `return` statement cannot specify a function result, and if the `return` statement is the last statement then it is optional. For example,

    func show( first string, last string) {
        name := first + " " + last
        fmt.Println("The name is ", name)
    }

    show("Bob", "Smith")
  
This example will run the function `show` with the two string values, and printe the formatted message "The name is Bob Smith". However, the function doesn't return a value (none specified after the parameter list before the function body) so there is no `return` statement needed in this case. 

Also note that the invocation of the `show` function does not specify a variable in which to store the result, becuase there is none. In this way you can see that a function can be called with a value that can be used in an assignment statement or an expression, or just called and the result value ignored. Even if the `show` function returned a value, the invocation ignores the result and it is discarded.

## The `defer` Statement  <a name="deferstmt"></a>
Sometimes a function may have multiple places where it returns from, but always wants to execute the same block of code to clean up a function (for example, closing a file that had been opened). For example,

    func getname() bool {
        f := io.Open("name")
        defer io.Close(f)

        s := io.ReadLine(f)
        if s == "" {
            return false
        }
        return true
    }

In this example, the function opens a file (the `io` package is discussed later). Because we have opened a file, we want to be sure to close it when we're done. This function has two `return` statements. We could code it such that before each one, we also call the io.Close() function each time. But the `defer` statement allows the function to specify a statement that will be executed _whenever the function actually returns_ regardless of which branch(es) through the conditional code are executed.

Each `defer` statement identifies a statement or a _basic block_ of statements (enclosed in "{" and "}") to be executed. If there are multiple `defer` statements, they are all executed in the reverse order that they were defined. So the first `defer` statement is always the last one executed before the function returns.

Note that `defer` statements are executed when the function comes to the end of the function body even if there is no `return` statement, as in the case of a function that does not return a value.

## Function Variable Values  <a name="funcvars"></a>
Functions can be values themselves. For example, consider:

    p := fmt.Println

This statement gets the value of the function `fmt.Println` and stores it in the variable p. From this point on, if you wanted to call the package function that prints items to the console, instead o fusing `fmt.Println` you can use the variable `p` to invoke the function:

    p("The answer is", x)

This means that you can pass a function as a parameter to another function. Consider,

    func show( fn interface{}, name string) {
        fn("The name is ", name)
    }
    p := fmt.Println
    show(p, "tom")

In this example, the `show` function has a first parameter that is a function, and a second parameter that is a string.  In the body of the function, the variable `fn` is used to call the `fmt.Println` function. You might use this if you wanted to send output to either the console (using `fmt.Println`) or a file (using `io.WriteString`). The calling code could make the choice of which function is appropriate, and pass that directly into the `show` function which makes the call.

You can even create a function literal value, which defines the body of the function, and either store it in a variable or pass it as a parameter. For example,

    p := func(first string, last string) string {
             return first + " " + last
         }

    name := p("Bob", "Smith")

Note that when defined as a function literal, the `func` keyword is not followed by a function name, but instead contains the parameter list, return value type, and function body directly.  There is no meaningful difference between the above and declaring `func p(first string...` except that the variable `p` has scope that might be limited to the current _basic block_ of code.  You can even define a function as a parameter to another function directly, as in:

    func compare( fn interface{}, v1 interface{}, v2 interface) bool {
        return fn(v1, v2)
    }

    x := compare( func(a1 bool, a2 bool) bool { return a1 == a2 }, true, false)

This somewhat artificial example shows a function named `compare` that has a first parameter that will be a function name, and two additional paramters of unknown type. The invocation of `compare` passes a function literal as the first parameter, which is a function that compares boolean values. Finally, the actual two boolean values are passed as the second and third parameters in the `compare` function call. The `compare` function will immediately invoke the function literal (stored in `fn`) to compare two boolean value, and return the result.

A more complex example might be a function whose job is to sort a list of values. Sorting a list of scalar values is available as a built-in function, but sorting a list of complex types can't be done wih the builtin `sort()` function. You could write a sort function, that accepts as a parameter the comparision operation, and that function knows how to decide between two values as to which one sorts first. This lets the sort function you create be generic without regard for the data types.

## Function Recivers  <a name="funcreceivers"></a>
A function can be written such that it can only be used when referenced via a variable of a specific type. This type is created by the user, and then the functions that 
are written to operate on that type use any variable of the type as a _receiver_, which means that variable in the function gets the value of the item in the function
invocation without it being an explicit parameter. This also allows multiple functions of the same name to exist which just reference different types.  For example,

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

Let's take a look more closely at this example to see what's going on.

1. This defines the type, called `Employee` which has a number of fields associated with it.
2. This declares a function that has a variable `e` of type `Employee` as it's receiver. The
   job of this function is to form a string of the employee name. Note that the name of the 
   variable is not a parameter, but is considered the receiver.

3. This creates an _instance_ of the type `Employee` and stores it in a variable `foo`.
4. This prints out a label and the value of the `Name` function when called using `foo` as
   the variable instance. The function `Name` cannot be called directly, it is an unknown 
   symbol unless called with a receiver of type `Employee` in its invocation.

In the declaration line line (2) above, you could use an asterisk ("*") character before the
receiver's type of `Employee`, as in:

    func (e *Employee) SetName(f string, l string)  { 
        e.first = f
        e.last = l
    }

This function can be used to set the names in the receiver variable. If the function was 
declared without the "*" marker, the receiver would actually only be a copy of the instance.
So the function `Name()` above can read any of the values from the receiver, if it sets them
it is changing a copy of the instance that only exists while the function is executing, and
the values in the `foo` instance are not changed.

By using the "*" as in the `SetName` example, the receiver `e` isn't a copy of the instance,
it is the actual instance. So changes made to the reciever `e` will actually be made in the
instance variable `foo`. An easy way to remember this is that, without the "*", the receiver
does not change the instance that makes the call, but with the "*" then changes will affect
the instance that makes the call.

Because specifying a receiver variable and type means the name of the function is scoped
to the type itself, you can have multiple functions with the same name that each has a
different receiver. An example would be to have each type define a `String()` method whose
job is to format the information in the value to be read by a human. The program can then
be written such that it doesn't matter was the type of an instance variable. By calling
the `String()` function from any instance, it will execute the _appropriate_ `String()`
function based on the type.

# Error Handling <a name="errors"></a>

There are two kinds of errors that can be managed in an _Ego_ program.

The first are
user- or runtime-generated errors, which are actually values of a data type called `error`.
You can create a new error variable using the `error()` function, as in:

    if v == 0 {
        return error("invalid zero value")
    }

This code, executed in a function, would return a value of type `error` that, when printed,
indicates the text string given. An `error` can also have value `nil` which means no error
is stored in the value. Some runtime functions will return an error value, and your code can check to see if the
result is nil versus being an actual error.

The second kind are panic error, which are errors generated by _Ego_ itself while running 
your program. For example, an attempt to divide by zero generates a panic error. By default,
this causes the program to stop running and an error message to be printed out.

## `try` and `catch` <a name="trycatch"></a>
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
Within this block, there is a variable `_error_` that is set to the
value of the error that was signalled. This can be used in the `catch`
block if it needs handle more than one possible error, for example.

## Signalling Errors <a name="signalling"></a>

You can cause a panic error to be signalled from within your code, which
would optionally be caught by a try/catch block, using the @error directive:

    if x == 0 {
        @error "Invalid value for x"
    }

When this code runs, if the value of `x` is zero, then a panic error is
signalled with an error message of "Invalid value for x". This error is
indistinguishable from a panic error generated by _Ego_ itself. If there
is a `catch{}` block, the value of the `_error_` variable will be the
string "Invalid value for x".


# Threads

# Builtin Packages

## fmt

## io

## math

## profile

## rest

## string

## time

## util

# User Packages


