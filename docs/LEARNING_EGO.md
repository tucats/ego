# Learning Ego: A Practical Introduction

Welcome to Ego! This guide will take you from your very first program through
the features you need to write real, useful code. No prior experience with Go
or Ego is assumed — we'll explain everything from the ground up.

---

## Table of Contents

1. [Getting Started](#1-getting-started)
   - [What Is Ego?](#11-what-is-ego)
   - [The REPL — Your Interactive Playground](#12-the-repl--your-interactive-playground)
   - [Running Programs from Files](#13-running-programs-from-files)
   - [Your First Program](#14-your-first-program)

2. [Variables and Data Types](#2-variables-and-data-types)
   - [Declaring Variables](#21-declaring-variables)
   - [Basic Types](#22-basic-types)
   - [Strings](#23-strings)
   - [Type Conversions](#24-type-conversions)
   - [Constants](#25-constants)
   - [The Blank Identifier](#26-the-blank-identifier)

3. [Operators and Expressions](#3-operators-and-expressions)
   - [Arithmetic Operators](#31-arithmetic-operators)
   - [Comparison Operators](#32-comparison-operators)
   - [Logical Operators](#33-logical-operators)
   - [Operator Precedence](#34-operator-precedence)

4. [Composite Data Types](#4-composite-data-types)
   - [Arrays and Slices](#41-arrays-and-slices)
   - [Maps](#42-maps)

5. [Flow Control](#5-flow-control)
   - [if / else](#51-if--else)
   - [for Loops](#52-for-loops)
   - [switch](#53-switch)

6. [Functions](#6-functions)
   - [Defining and Calling Functions](#61-defining-and-calling-functions)
   - [Multiple Return Values](#62-multiple-return-values)
   - [Variadic Functions](#63-variadic-functions)
   - [Anonymous Functions and Closures](#64-anonymous-functions-and-closures)
   - [The defer Statement](#65-the-defer-statement)

7. [Structs and Methods](#7-structs-and-methods)
   - [Defining a Struct](#71-defining-a-struct)
   - [Creating and Using Struct Values](#72-creating-and-using-struct-values)
   - [Methods](#73-methods)
   - [Pointer Receivers](#74-pointer-receivers)

8. [Interfaces](#8-interfaces)
   - [Defining an Interface](#81-defining-an-interface)
   - [Implementing an Interface](#82-implementing-an-interface)
   - [Using Interfaces](#83-using-interfaces)

9. [Error Handling](#9-error-handling)
   - [The Error Type](#91-the-error-type)
   - [Returning and Checking Errors](#92-returning-and-checking-errors)
   - [The try / catch Extension](#93-the-try--catch-extension)

10. [Packages](#10-packages)
    - [Importing Packages](#101-importing-packages)
    - [Commonly Used Packages](#102-commonly-used-packages)

---

## 1. Getting Started

### 1.1 What Is Ego?

Ego is a scripting language that is deliberately designed to look and behave
like [Go](https://go.dev) — one of the most readable and practical programming
languages in use today. If you've seen Go code before, Ego will look very
familiar. If you haven't, don't worry; we'll explain every concept as we go.

Ego is *dynamically typed* by default, meaning you don't have to tell the
language exactly what kind of data a variable holds — it figures that out for
you at runtime. This makes it easy to get started while still giving you the
power of a structured language.

### 1.2 The REPL — Your Interactive Playground

When you run `ego` with no arguments, you enter the **REPL** (Read–Eval–Print
Loop). Think of it as a live conversation with the language: you type a line,
Ego executes it immediately, and you see the result.

```sh
$ ego
ego> 
```

The REPL is great for experimenting. Unlike a traditional program, statements
you type at the REPL prompt are executed directly — you don't need to wrap them
in a `main` function.

An important property of the REPL: **it remembers everything**. Variables you
create and functions you define persist for the entire session. If you type:

```text
ego> x := 42
ego> y := 10
ego> fmt.Println(x + y)
52
```

The variable `x` is still set to `42` on the second line, and on the third.
This stateful memory makes the REPL ideal for exploration and step-by-step
testing.

> **Note:** The REPL helps you out by automatically importing common Ego packages
> so you don't have to do explicit imports. However, you can still `import "fmt"` before
> using `fmt.Println`, since that would be required when running an Ego program
> outside the REPL environment.
> Alternatively, if you have language extensions enabled (more on that later),
> you can use the simpler `print` statement.

To exit the REPL, press `Ctrl-D` or `Ctrl-C`, or use the command `exit`.

### 1.3 Running Programs from Files

For anything more than quick experiments, you'll want to save your code to a
file. By convention, Ego source files end in `.ego`.

Run a source file with:

```sh
ego run myprogram.ego
```

Source files follow a more formal structure than the REPL. They need a
`package` declaration and a `main` function — the function that Ego calls to
start your program. We'll see what this looks like in the very next section.

### 1.4 Your First Program

Create a file called `hello.ego` with the following contents:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

Run it:

```sh
$ ego run hello.ego
Hello, World!
```

Let's break it down line by line:

- `package main` — every runnable Ego program starts with this. It tells Ego
  that this file is the entry point for the program.
- `import "fmt"` — the `fmt` package provides formatted output functions. We
  need to import it before we can use `fmt.Println`.
- `func main() { ... }` — this defines the `main` function. When you run an
  Ego program, execution starts here.
- `fmt.Println("Hello, World!")` — this calls the `Println` function from the
  `fmt` package, which prints its arguments followed by a newline.

---

## 2. Variables and Data Types

### 2.1 Declaring Variables

The most common way to create a variable in Ego is with the `:=` operator,
called the **short variable declaration**:

```go
name := "Alice"
age  := 30
pi   := 3.14159
```

This creates the variable and assigns it a value in one step. The type of the
variable is inferred from the value — `name` becomes a `string`, `age` becomes
an `int`, and `pi` becomes a `float64`.

You can also use the `var` keyword, which is useful when you want to declare a
variable without immediately giving it a value:

```go
var score int
var message string
```

Uninitialized variables receive the *zero value* for their type: `0` for
numbers, `""` for strings, and `false` for booleans.

To update an existing variable use plain `=`:

```go
score = 100
message = "Game over"
```

Use `:=` to create; use `=` to update.

### 2.2 Basic Types

| Type | Example values | Description |
| ---- | ------------- | ----------- |
| `int` | `0`, `-5`, `1000000` | Whole numbers |
| `float64` | `3.14`, `-0.5`, `1.0e10` | Decimal numbers |
| `string` | `"hello"`, `"42"`, `""` | Text |
| `bool` | `true`, `false` | True or false |

Ego also has sized integer types (`int8`, `int16`, `int32`, `int64`) and
unsigned variants (`uint`, `uint8`, etc.), but `int` and `float64` are what
you'll use most of the time.

```go
package main

import "fmt"

func main() {
    i := 42
    f := 3.14
    s := "hello"
    b := true

    fmt.Println("int:", i)
    fmt.Println("float:", f)
    fmt.Println("string:", s)
    fmt.Println("bool:", b)
}
```

Output:

```text
int: 42
float: 3.14
string: hello
bool: true
```

### 2.3 Strings

Strings in Ego are sequences of characters enclosed in double quotes. You can
also use back-quotes for *raw strings*, which can span multiple lines and
treat backslashes literally:

```go
greeting := "Hello, World!"
path     := `C:\Users\Alice\Documents`
```

**Concatenation** joins strings together with `+`:

```go
first := "Hello"
second := "World"
fmt.Println(first + ", " + second + "!")
// Hello, World!
```

**String length** uses the built-in `len` function:

```go
s := "hello"
fmt.Println(len(s))   // 5
```

For formatted strings, use `fmt.Sprintf`, which works like `printf` in other
languages. The `%s` placeholder substitutes a string, `%d` substitutes an
integer, and `%f` substitutes a float:

```go
name := "Alice"
age  := 30
msg  := fmt.Sprintf("My name is %s and I am %d years old.", name, age)
fmt.Println(msg)
// My name is Alice and I am 30 years old.
```

The `strings` package has many useful helpers:

```go
import "strings"

s := "Hello, World!"
fmt.Println(strings.ToUpper(s))           // HELLO, WORLD!
fmt.Println(strings.ToLower(s))           // hello, world!
fmt.Println(strings.Contains(s, "World")) // true
fmt.Println(strings.Replace(s, "World", "Ego", 1)) // Hello, Ego!
fmt.Println(strings.Split("a,b,c", ","))  // ["a", "b", "c"]
fmt.Println(strings.TrimSpace("  hello  ")) // hello
```

### 2.4 Type Conversions and Type Modes

One area where Ego deliberately diverges from Go is in how strictly it enforces
types. Ego has three *type modes* that you can choose from, ranging from very
permissive to strict. The mode affects two things: whether a variable can change
its type after it is first assigned, and whether the runtime will automatically
convert values when a different type is needed.

#### Dynamic mode (the default)

In **dynamic** mode, Ego behaves more like a scripting language than a compiled
language. A variable is not locked to the type it was first assigned — you can
point it at a completely different kind of value later:

```go
package main

import "fmt"

func main() {
    x := 42          // x starts as an int
    fmt.Println(x)   // 42

    x = "hello"      // x is now a string — perfectly fine
    fmt.Println(x)   // hello

    x = 3.14         // x is now a float64
    fmt.Println(x)   // 3.14
}
```

Dynamic mode also converts values automatically wherever they are used. If a
function expects an `int` but you pass the string `"10"`, Ego converts it for
you at runtime:

```go
func addInts(a, b int) int {
    return a + b
}

result := addInts("10", "5")   // strings are converted to ints automatically
fmt.Println(result)             // 15
```

This makes quick programs easy to write, but it can hide bugs — if you pass
`"hello"` where an `int` is expected, the error only shows up at runtime.

The built-in `typeof()` function is handy for checking what type a variable
currently holds, especially useful in the REPL while experimenting:

```go
x := 42
fmt.Println(typeof(x))   // int

x = "hello"
fmt.Println(typeof(x))   // string

x = 3.14
fmt.Println(typeof(x))   // float64

fmt.Println(typeof([]int{1, 2, 3}))   // []int
```

`typeof()` works on any value or expression, not just variables.

#### Relaxed mode

**Relaxed** mode is a middle ground. Values are still auto-converted when
passed to functions or used in expressions, but a variable *keeps the type it
was first given*. Assigning a value of a different type to an existing variable
is an error:

```go
@type relaxed
package main

import "fmt"

func main() {
    x := 42
    x = "hello"   // Error: invalid type for this variable
    fmt.Println(x)
}
```

```text
Error: at main(line 6), invalid type for this variable
```

However, function calls still benefit from automatic conversion — a string
`"10"` passed to an `int` parameter is silently coerced:

```go
@type relaxed

result := addInts("10", "5")   // still works: strings are auto-converted
fmt.Println(result)             // 15
```

Relaxed mode is a good choice when you want variables to behave predictably
but don't want to add explicit casts everywhere.

#### Strict mode

**Strict** mode mirrors Go's behavior. Types are rigidly enforced: variables
cannot change type, and there is no automatic conversion anywhere. Every type
mismatch must be resolved explicitly by the programmer using a cast:

```go
@type strict
package main

import "fmt"

func addInts(a, b int) int {
    return a + b
}

func main() {
    // This is an error in strict mode — string is not int
    result := addInts("10", "5")
    fmt.Println(result)
}
```

```text
Error: at addInts(line 11), incorrect function argument type: argument 1: string
```

With explicit casts, strict mode works fine:

```go
@type strict
package main

import "fmt"

func addInts(a, b int) int {
    return a + b
}

func main() {
    result := addInts(int("10"), int("5"))   // explicit casts required
    fmt.Println(result)                       // 15

    i := 42
    f := float64(i)   // int → float64
    fmt.Println(f)    // 42
}
```

Cast syntax looks like a function call: `int(x)`, `float64(x)`, `string(x)`.
The cast truncates when converting from float to int — it does not round:

```go
n := int(3.99)   // 3, not 4
```

#### Choosing a mode

| Mode | Variable type can change | Auto-converts in calls/expressions | Explicit casts |
| ---- | :---: | :---: | :---: |
| `dynamic` (default) | Yes | Yes | Optional |
| `relaxed` | No | Yes | Optional |
| `strict` | No | No | Required |

You can set the mode in three ways:

**1. Inside a source file** with the `@type` directive (must appear before
`package`):

```go
@type strict
package main
```

**2. On the command line** when starting the REPL or running a file:

```sh
ego run myprogram.ego --types=strict 
ego --types=relaxed
```

**3. Permanently** via the profile setting:

```sh
ego config set ego.compiler.types=strict
```

If no mode is set, `dynamic` is used.

### 2.5 Constants

A **constant** is a value that never changes. Use `const` to declare one:

```go
const Pi = 3.14159
const Greeting = "Hello"
const MaxRetries = 3
```

Constants can be grouped together:

```go
const (
    StatusOK       = 200
    StatusNotFound = 404
    StatusError    = 500
)
```

The compiler will stop you from accidentally assigning a new value to a
constant, which helps prevent bugs.

### 2.6 The Blank Identifier

Sometimes you need to call a function that returns multiple values, but you
only care about some of them. Use `_` (underscore) as a throwaway variable:

```go
_, err := someFunction()   // discard the first return value
value, _ := someFunction() // discard the second return value (error)
```

We'll see this pattern frequently in the error-handling section.

---

## 3. Operators and Expressions

### 3.1 Arithmetic Operators

| Operator | Meaning | Example |
| -------- | ------- | ------- |
| `+` | Addition | `3 + 4` → `7` |
| `-` | Subtraction | `10 - 3` → `7` |
| `*` | Multiplication | `4 * 5` → `20` |
| `/` | Division | `10 / 3` → `3` (integer division) |
| `%` | Remainder | `10 % 3` → `1` |

When both operands are integers, `/` performs integer division (the fractional
part is discarded). To get a decimal result, at least one operand must be a
`float64`:

```go
fmt.Println(10 / 3)          // 3
fmt.Println(10.0 / 3)        // 3.3333333333333335
fmt.Println(float64(10) / 3) // 3.3333333333333335
```

**Compound assignment operators** combine an operation with assignment:

```go
x := 10
x += 5   // x is now 15
x -= 3   // x is now 12
x *= 2   // x is now 24
x /= 4   // x is now 6
```

**Increment and decrement** add or subtract 1:

```go
count := 0
count++   // count is now 1
count++   // count is now 2
count--   // count is now 1
```

### 3.2 Comparison Operators

Comparisons return a `bool` (`true` or `false`):

| Operator | Meaning |
| -------- | ------- |
| `==` | Equal |
| `!=` | Not equal |
| `<` | Less than |
| `<=` | Less than or equal |
| `>` | Greater than |
| `>=` | Greater than or equal |

```go
x := 5
fmt.Println(x == 5)   // true
fmt.Println(x != 3)   // true
fmt.Println(x > 10)   // false
fmt.Println(x >= 5)   // true
```

### 3.3 Logical Operators

| Operator | Meaning | Example |
| -------- | ------- | ------- |
| `&&` | AND — true if both sides are true | `true && false` → `false` |
| `\|\|` | OR — true if either side is true | `true \|\| false` → `true` |
| `!` | NOT — flips true/false | `!true` → `false` |

```go
age := 25
hasLicense := true

if age >= 18 && hasLicense {
    fmt.Println("You can drive.")
}
```

### 3.4 Operator Precedence

When operators appear together in an expression, higher-precedence operators
are evaluated first. From lowest to highest:

1. `||`
2. `&&`
3. `==` `!=` `<` `<=` `>` `>=`
4. `+` `-`
5. `*` `/` `%`
6. Unary `-` `!`

When in doubt, use parentheses to make the order explicit:

```go
result := (3 + 4) * 2    // 14, not 11
```

---

## 4. Composite Data Types

### 4.1 Arrays and Slices

A **slice** is an ordered collection of values, all of the same type. You can
think of it as a resizable list. This is one of the most useful data structures
in Ego.

**Creating a slice:**

```go
fruits := []string{"apple", "banana", "cherry"}
nums   := []int{10, 20, 30, 40, 50}
```

The type is written as `[]` followed by the element type.

**Accessing elements** uses zero-based indexing (the first element is at
index 0):

```go
fmt.Println(fruits[0])   // apple
fmt.Println(fruits[2])   // cherry
```

**Length** of a slice:

```go
fmt.Println(len(fruits))  // 3
```

**Slicing a slice** (taking a sub-range):

```go
fmt.Println(nums[0:3])   // [10, 20, 30] — indices 0, 1, 2
fmt.Println(nums[3:])    // [40, 50]     — from index 3 to the end
fmt.Println(nums[:2])    // [10, 20]     — from the start up to (not including) index 2
```

**Appending** elements creates a new (possibly larger) slice:

```go
nums = append(nums, 60)
fmt.Println(nums)   // [10, 20, 30, 40, 50, 60]
```

**Creating an empty slice** with `make`:

```go
zeros := make([]int, 5)
fmt.Println(zeros)   // [0, 0, 0, 0, 0]
```

**Iterating** with a `for range` loop:

```go
for i, fruit := range fruits {
    fmt.Println(i, fruit)
}
// 0 apple
// 1 banana
// 2 cherry
```

If you only need the values and not the indices, discard the index with `_`:

```go
for _, fruit := range fruits {
    fmt.Println(fruit)
}
```

Here's a complete example:

```go
package main

import "fmt"

func main() {
    nums := []int{10, 20, 30, 40, 50}

    fmt.Println("Slice:", nums)
    fmt.Println("Length:", len(nums))
    fmt.Println("First:", nums[0])
    fmt.Println("Last:", nums[len(nums)-1])
    fmt.Println("First three:", nums[0:3])
    fmt.Println("Last two:", nums[3:])

    nums = append(nums, 60)
    fmt.Println("After append:", nums)
}
```

Output:

```text
Slice: [10, 20, 30, 40, 50]
Length: 5
First: 10
Last: 50
First three: [10, 20, 30]
Last two: [40, 50]
After append: [10, 20, 30, 40, 50, 60]
```

### 4.2 Maps

A **map** stores key-value pairs. It's like a dictionary: given a key, you can
quickly look up its value.

**Creating a map:**

```go
scores := map[string]int{
    "Alice": 95,
    "Bob":   82,
    "Carol": 91,
}
```

The type is written as `map[KeyType]ValueType`.

**Looking up a value:**

```go
fmt.Println("Alice's score:", scores["Alice"])  // 95
```

**Adding or updating:**

```go
scores["Dave"] = 78
scores["Alice"] = 97   // update Alice's score
```

**Checking whether a key exists** — a map lookup returns two values: the value
and a boolean that is `true` if the key was found:

```go
val, ok := scores["Eve"]
if ok {
    fmt.Println("Eve's score:", val)
} else {
    fmt.Println("Eve is not in the map")
}
```

This two-value form is called the "comma ok" pattern. If you look up a key that
doesn't exist without using it, you get the zero value for the type (e.g., `0`
for `int`) — not an error.

**Iterating over a map:**

```go
for name, score := range scores {
    fmt.Println(name, "->", score)
}
```

Note that map iteration order is not guaranteed.

**Complete example:**

```go
package main

import "fmt"

func main() {
    scores := map[string]int{
        "Alice": 95,
        "Bob":   82,
        "Carol": 91,
    }

    scores["Dave"] = 78

    val, ok := scores["Eve"]
    if ok {
        fmt.Println("Eve's score:", val)
    } else {
        fmt.Println("Eve is not in the map")
    }

    for name, score := range scores {
        fmt.Println(name, "->", score)
    }
}
```

Output:

```text
Eve is not in the map
Alice -> 95
Bob -> 82
Carol -> 91
Dave -> 78
```

---

## 5. Flow Control

Flow control statements decide which code runs and how many times.

### 5.1 if / else

The `if` statement runs a block of code only when a condition is true:

```go
x := 15
if x > 10 {
    fmt.Println("x is greater than 10")
}
```

Add an `else` clause for what happens when the condition is false:

```go
if x > 10 {
    fmt.Println("x is greater than 10")
} else {
    fmt.Println("x is 10 or less")
}
```

Chain multiple conditions with `else if`:

```go
score := 85

if score >= 90 {
    fmt.Println("A")
} else if score >= 80 {
    fmt.Println("B")
} else if score >= 70 {
    fmt.Println("C")
} else {
    fmt.Println("F")
}
```

**Init statement:** You can include a short statement before the condition,
separated by a semicolon. The variable declared there is scoped to the `if`
block:

```go
if n := someFunction(); n > 0 {
    fmt.Println("positive:", n)
}
// n is not accessible here
```

This pattern is very common when calling functions that return an error.

### 5.2 for Loops

Ego has one loop keyword — `for` — that covers all looping patterns.

**Infinite loop** (must contain `break` to exit):

```go
count := 0
for {
    count++
    if count >= 5 {
        break
    }
}
fmt.Println("counted to:", count)   // 5
```

**While-style loop** (condition only):

```go
n := 1
for n < 100 {
    n = n * 2
}
fmt.Println("first power of 2 >= 100:", n)   // 128
```

**C-style loop** (init; condition; increment):

```go
for i := 0; i < 5; i++ {
    fmt.Println(i)
}
// 0 1 2 3 4
```

**Range loop** over a slice:

```go
fruits := []string{"apple", "banana", "cherry"}
for i, fruit := range fruits {
    fmt.Println(i, fruit)
}
// 0 apple
// 1 banana
// 2 cherry
```

**Range loop** over a map:

```go
scores := map[string]int{"Alice": 95, "Bob": 82}
for name, score := range scores {
    fmt.Println(name, score)
}
```

**`break` and `continue`:**

- `break` exits the loop immediately.
- `continue` skips the rest of the current iteration and moves on to the next.

```go
for i := 0; i < 10; i++ {
    if i % 2 != 0 {
        continue     // skip odd numbers
    }
    fmt.Println(i)
}
// 0 2 4 6 8
```

### 5.3 switch

`switch` compares a value against a series of cases, and runs the matching one:

```go
day := "Monday"

switch day {
case "Monday":
    fmt.Println("Start of the work week")
case "Friday":
    fmt.Println("End of the work week")
case "Saturday", "Sunday":
    fmt.Println("Weekend!")
default:
    fmt.Println("Middle of the week")
}
```

Unlike C, Ego's `switch` does **not** fall through from one case to the next
by default. Each case is independent. If you want fall-through behavior (rarely
needed), add `fallthrough` as the last statement in a case.

**Condition switch** — omit the switch expression entirely and each case acts
like an `if` condition:

```go
x := 42

switch {
case x < 0:
    fmt.Println("negative")
case x == 0:
    fmt.Println("zero")
case x > 0:
    fmt.Println("positive")
}
```

**Init statement** in switch:

```go
switch n := computeValue(); {
case n < 0:
    fmt.Println("negative result")
default:
    fmt.Println("non-negative result:", n)
}
```

---

## 6. Functions

Functions are named, reusable blocks of code. They are fundamental to keeping
programs organized and readable.

### 6.1 Defining and Calling Functions

```go
func greet(name string) string {
    return "Hello, " + name + "!"
}

func add(a, b int) int {
    return a + b
}
```

The `func` keyword introduces a function definition. After the name comes the
parameter list (name then type for each parameter), and then the return type.
The function body is the block between `{` and `}`.

Call a function by name followed by arguments in parentheses:

```go
func main() {
    fmt.Println(greet("World"))     // Hello, World!
    fmt.Println("3 + 4 =", add(3, 4))  // 3 + 4 = 7
}
```

When multiple parameters share a type, you can list them with one type at the
end:

```go
func add(a, b int) int { ... }        // both a and b are int
func combine(x, y string, n int) { ... }
```

### 6.2 Multiple Return Values

Functions in Ego can return more than one value. This is especially useful for
returning both a result and an error:

```go
func minMax(nums []int) (int, int) {
    min := nums[0]
    max := nums[0]
    for _, n := range nums {
        if n < min {
            min = n
        }
        if n > max {
            max = n
        }
    }
    return min, max
}
```

Capture multiple return values with multiple variables:

```go
nums := []int{5, 2, 8, 1, 9, 3}
lo, hi := minMax(nums)
fmt.Println("min:", lo, "max:", hi)   // min: 1 max: 9
```

Full example:

```go
package main

import "fmt"

func greet(name string) string {
    return "Hello, " + name + "!"
}

func add(a, b int) int {
    return a + b
}

func minMax(nums []int) (int, int) {
    min := nums[0]
    max := nums[0]
    for _, n := range nums {
        if n < min { min = n }
        if n > max { max = n }
    }
    return min, max
}

func main() {
    fmt.Println(greet("World"))
    fmt.Println("3 + 4 =", add(3, 4))

    nums := []int{5, 2, 8, 1, 9, 3}
    lo, hi := minMax(nums)
    fmt.Println("min:", lo, "max:", hi)
}
```

Output:

```text
Hello, World!
3 + 4 = 7
min: 1 max: 9
```

### 6.3 Variadic Functions

A **variadic** function accepts a variable number of arguments. Indicate this
with `...` before the type of the last parameter:

```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total = total + n
    }
    return total
}
```

Inside the function, `nums` is a slice of `int`. You call it with as many
arguments as you like:

```go
fmt.Println(sum(1, 2, 3))            // 6
fmt.Println(sum(10, 20, 30, 40, 50)) // 150
```

If you already have a slice, **unpack** it with `...`:

```go
values := []int{5, 6, 7}
fmt.Println(sum(values...))   // 18
```

### 6.4 Anonymous Functions and Closures

A function doesn't have to have a name. You can define an **anonymous
function** and assign it to a variable:

```go
greet := func(name string) string {
    return "Hello, " + name + "!"
}
fmt.Println(greet("World"))   // Hello, World!
```

You can also define and call a function immediately (useful for one-off
calculations):

```go
result := func(a, b int) int {
    return a + b
}(3, 4)
fmt.Println("3 + 4 =", result)   // 3 + 4 = 7
```

The powerful feature of anonymous functions is that they can *close over*
variables from their enclosing scope. A function that captures variables from
its surroundings is called a **closure**:

```go
package main

import "fmt"

func makeCounter() {
    count := 0
    increment := func() int {
        count = count + 1
        return count
    }
    fmt.Println(increment())   // 1
    fmt.Println(increment())   // 2
    fmt.Println(increment())   // 3
}

func main() {
    makeCounter()
}
```

Each call to `increment` updates the same `count` variable. The inner function
"remembers" the variable from the outer function.

### 6.5 The defer Statement

`defer` schedules a function call to happen *just before the enclosing function
returns*, no matter how the function exits. This is useful for cleanup tasks:

```go
package main

import "fmt"

func countdown() {
    defer fmt.Println("Blastoff!")
    fmt.Println("3...")
    fmt.Println("2...")
    fmt.Println("1...")
}

func main() {
    countdown()
    fmt.Println("Rocket launched")
}
```

Output:

```text
3...
2...
1...
Blastoff!
Rocket launched
```

Even though `defer fmt.Println("Blastoff!")` appears first in the function
body, it runs *last*, right before `countdown` returns. Multiple `defer`
statements run in last-in, first-out order.

---

## 7. Structs and Methods

A **struct** is a collection of named fields, each with its own type. Structs
let you group related data together and give it a meaningful name.

### 7.1 Defining a Struct

```go
type Person struct {
    Name string
    Age  int
}
```

`type` introduces a new type definition. `Person` is the name of the new type,
and its fields are `Name` (a `string`) and `Age` (an `int`).

### 7.2 Creating and Using Struct Values

**Named field initialization** (recommended — order doesn't matter):

```go
alice := Person{Name: "Alice", Age: 30}
```

**Ordered initialization** (must match the field order in the definition):

```go
bob := Person{"Bob", 25}
```

**Accessing fields** with the dot operator:

```go
fmt.Println(alice.Name)   // Alice
fmt.Println(alice.Age)    // 30
```

**Updating fields:**

```go
alice.Age = 31
```

### 7.3 Methods

A **method** is a function associated with a type. You define a method by
giving the function a **receiver** — a parameter that names the value the
method is called on.

```go
func (p Person) Greet() string {
    return "Hi, I'm " + p.Name + " and I'm " + string(p.Age) + " years old."
}
```

The `(p Person)` before the function name is the receiver. Inside the method,
`p` refers to the `Person` value the method was called on.

Call a method with dot notation:

```go
fmt.Println(alice.Greet())
// Hi, I'm Alice and I'm 30 years old.
```

### 7.4 Pointer Receivers

The method above takes a *copy* of the `Person` — any changes to `p` inside the
method don't affect the original. To write a method that *modifies* the
original value, use a **pointer receiver** with `*`:

```go
func (p *Person) Birthday() {
    p.Age = p.Age + 1
}
```

Now calling `alice.Birthday()` modifies `alice` itself:

```go
alice.Birthday()
fmt.Println("After birthday:", alice.Age)   // 31
```

**Complete example:**

```go
package main

import "fmt"

type Person struct {
    Name string
    Age  int
}

func (p Person) Greet() string {
    return "Hi, I'm " + p.Name + " and I'm " + string(p.Age) + " years old."
}

func (p *Person) Birthday() {
    p.Age = p.Age + 1
}

func main() {
    alice := Person{Name: "Alice", Age: 30}
    fmt.Println(alice.Greet())

    alice.Birthday()
    fmt.Println("After birthday:", alice.Age)

    bob := Person{"Bob", 25}
    fmt.Println(bob.Greet())
}
```

Output:

```text
Hi, I'm Alice and I'm 30 years old.
After birthday: 31
Hi, I'm Bob and I'm 25 years old.
```

---

## 8. Interfaces

An **interface** defines a set of method signatures. Any type that implements
all of those methods satisfies the interface — without ever saying so
explicitly. This is called *structural typing*, and it's one of the most
elegant features of Go-style languages.

### 8.1 Defining an Interface

```go
type Shape interface {
    Area() float64
    Perimeter() float64
}
```

This says: "anything that has `Area()` and `Perimeter()` methods that return
`float64` is a `Shape`."

### 8.2 Implementing an Interface

You don't need to write `implements Shape`. Any struct whose methods match is
automatically a `Shape`:

```go
type Rectangle struct {
    Width  float64
    Height float64
}

func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

func (r Rectangle) Perimeter() float64 {
    return 2 * (r.Width + r.Height)
}

type Circle struct {
    Radius float64
}

func (c Circle) Area() float64 {
    return 3.14159 * c.Radius * c.Radius
}

func (c Circle) Perimeter() float64 {
    return 2 * 3.14159 * c.Radius
}
```

Both `Rectangle` and `Circle` implement `Shape` because they both have `Area()`
and `Perimeter()` methods.

### 8.3 Using Interfaces

Write a function that accepts any `Shape`:

```go
func printShapeInfo(s Shape) {
    fmt.Printf("Area: %.2f, Perimeter: %.2f\n", s.Area(), s.Perimeter())
}
```

Now you can pass either a `Rectangle` or a `Circle`:

```go
package main

import (
    "fmt"
    "math"
)

type Shape interface {
    Area() float64
    Perimeter() float64
}

type Rectangle struct {
    Width  float64
    Height float64
}

func (r Rectangle) Area() float64      { return r.Width * r.Height }
func (r Rectangle) Perimeter() float64 { return 2 * (r.Width + r.Height) }

type Circle struct {
    Radius float64
}

func (c Circle) Area() float64      { return math.Pi * c.Radius * c.Radius }
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.Radius }

func printShapeInfo(s Shape) {
    fmt.Printf("Area: %.2f, Perimeter: %.2f\n", s.Area(), s.Perimeter())
}

func main() {
    r := Rectangle{Width: 4.0, Height: 3.0}
    c := Circle{Radius: 5.0}

    fmt.Print("Rectangle: ")
    printShapeInfo(r)

    fmt.Print("Circle: ")
    printShapeInfo(c)
}
```

Output:

```text
Rectangle: Area: 12.00, Perimeter: 14.00
Circle: Area: 78.54, Perimeter: 31.42
```

Interfaces make your code more flexible: `printShapeInfo` doesn't know or care
whether it's dealing with a rectangle, circle, or any other shape you might
add later.

---

## 9. Error Handling

Errors are a normal part of programming — files might not exist, network
connections might fail, user input might be invalid. Ego handles errors
explicitly rather than through exceptions, which makes it easy to see exactly
where things can go wrong.

### 9.1 The Error Type

Ego has a built-in `error` type. A value of type `error` either holds an error
message or is `nil` (meaning "no error").

Create an error with the `errors` package:

```go
import "errors"

err := errors.New("something went wrong")
```

### 9.2 Returning and Checking Errors

The standard pattern is for functions to return an error as their *last* return
value. The caller checks whether it is `nil`:

```go
package main

import (
    "errors"
    "fmt"
)

func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("cannot divide by zero")
    }
    return a / b, nil
}

func main() {
    result, err := divide(10.0, 2.0)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("10 / 2 =", result)
    }

    result, err = divide(5.0, 0.0)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("5 / 0 =", result)
    }
}
```

Output:

```text
10 / 2 = 5
Error: cannot divide by zero
```

The `if err != nil` check is the single most common pattern in Ego code. Get
comfortable with it.

Here's a slightly more realistic example — looking up an item in a map and
returning an error if it isn't found:

```go
package main

import (
    "errors"
    "fmt"
)

func findItem(id int) (string, error) {
    items := map[int]string{
        1: "apple",
        2: "banana",
        3: "cherry",
    }
    item, ok := items[id]
    if !ok {
        return "", errors.New("item not found")
    }
    return item, nil
}

func main() {
    item, err := findItem(2)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Found:", item)
    }

    item, err = findItem(99)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Found:", item)
    }
}
```

Output:

```text
Found: banana
Error: item not found
```

### 9.3 The try / catch Extension

Ego supports a `try`/`catch` construct as a language extension. To use it,
add `@extensions true` to the top of your source file.

`try` runs a block of code; if any runtime error occurs inside that block,
execution jumps to the `catch` block instead of aborting the program:

```go
@extensions true
package main

import "fmt"

func riskyDiv(a, b int) int {
    return a / b
}

func main() {
    try {
        fmt.Println("10 / 2 =", riskyDiv(10, 2))
        fmt.Println("10 / 0 =", riskyDiv(10, 0))
        fmt.Println("This line will not be reached")
    } catch(err) {
        fmt.Println("Caught an error:", err)
    }
    fmt.Println("Program continues normally")
}
```

Output:

```text
10 / 2 = 5
Caught an error: at riskyDiv(line 7), division by zero
Program continues normally
```

The identifier inside `catch(...)` receives the error value. After the catch
block finishes, the program continues normally from the statement after the
entire `try`/`catch`.

Use `try`/`catch` for situations where you expect occasional runtime errors and
want to handle them gracefully, without cluttering every line with `if err != nil`.

---

## 10. Packages

Packages are collections of related functions, types, and constants that you
can reuse across programs.

### 10.1 Importing Packages

Use the `import` statement to bring a package into your program:

```go
import "fmt"
```

For multiple packages, group them:

```go
import (
    "fmt"
    "math"
    "strings"
)
```

Access items from a package with the package name and a dot:

```go
fmt.Println("hello")
x := math.Sqrt(16.0)
s := strings.ToUpper("hello")
```

Give a package a local alias if its name conflicts or is inconvenient:

```go
import (
    f "fmt"
)

f.Println("using the alias")
```

### 10.2 Commonly Used Packages

Here is a selection of the packages available in Ego:

**`fmt`** — Formatted I/O

| Function | Purpose |
| -------- | ------- |
| `fmt.Println(...)` | Print arguments separated by spaces, followed by a newline |
| `fmt.Printf(format, ...)` | Print with format string (`%s`, `%d`, `%f`, etc.) |
| `fmt.Sprintf(format, ...)` | Return a formatted string (no output) |
| `fmt.Print(...)` | Print without a trailing newline |

**`strings`** — String manipulation

| Function | Purpose |
| -------- | ------- |
| `strings.Contains(s, sub)` | True if `s` contains `sub` |
| `strings.HasPrefix(s, pre)` | True if `s` starts with `pre` |
| `strings.HasSuffix(s, suf)` | True if `s` ends with `suf` |
| `strings.ToUpper(s)` | Convert to uppercase |
| `strings.ToLower(s)` | Convert to lowercase |
| `strings.TrimSpace(s)` | Remove leading/trailing whitespace |
| `strings.Split(s, sep)` | Split into a slice on separator |
| `strings.Join(parts, sep)` | Join a slice of strings with separator |
| `strings.Replace(s, old, new, n)` | Replace up to `n` occurrences |

**`math`** — Mathematical functions

| Function | Purpose |
| -------- | ------- |
| `math.Abs(x)` | Absolute value |
| `math.Sqrt(x)` | Square root |
| `math.Floor(x)` / `math.Ceil(x)` | Round down / up |
| `math.Min(a, b)` / `math.Max(a, b)` | Minimum / maximum |
| `math.Pi` | The constant π |

**`errors`** — Error creation

| Function | Purpose |
| -------- | ------- |
| `errors.New(msg)` | Create a new error with the given message |

**`sort`** — Sorting slices

```go
import "sort"

nums := []int{5, 2, 8, 1, 9}
sort.Ints(nums)
fmt.Println(nums)   // [1, 2, 5, 8, 9]

words := []string{"banana", "apple", "cherry"}
sort.Strings(words)
fmt.Println(words)  // [apple, banana, cherry]
```

---

## What's Next?

This guide has covered the core of the Ego language. You can now write
programs that use variables, functions, structs, interfaces, slices, maps,
and basic error handling.

As you get more comfortable, explore:

- **The `fmt.Sprintf` format verbs** — `%v` prints any value in a default
  format, which is very handy for debugging.
- **Goroutines and channels** — Ego supports concurrent programming with the
  `go` statement and channel operations (`<-`).
- **The `@extensions` directive** — enables extra features like `try`/`catch`,
  `print`, and the ternary operator (`a ? b : c`).
- **The server mode** — Ego can run as a REST server with Ego programs serving
  as endpoints.
- **The `LANGUAGE.md` reference** — for a complete description of all built-in
  packages and their functions.
- **The `SYNTAX.md` reference** — for the formal grammar of the language.
