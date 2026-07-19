# Ego Language Syntax Reference

This document is a formal description of the _Ego_ language grammar using
Extended Backus-Naur Form (EBNF). Ego is closely modeled on the
[Go programming language](https://go.dev/ref/spec). Readers familiar with Go
will recognize most of the syntax; this document highlights where Ego diverges.

## EBNF Notation

| Notation | Meaning |
| -------- | ------- |
| `::=` | Defines a production rule |
| `\|` | Alternative (either/or) |
| `{ x }` | Zero or more repetitions of `x` |
| `[ x ]` | Optional `x` (zero or one) |
| `( x )` | Grouping |
| `"word"` | Literal terminal token |
| `UPPER` | Lexical token category |
| `lower` | Reference to another production |

---

## 1. Lexical Tokens

The tokenizer delivers tokens pre-classified into the following categories.
Multi-character operator sequences such as `!=`, `:=`, `<<`, and `>=` are
presented as single tokens.

```ebnf
IDENTIFIER  ::= letter { letter | digit | "_" }
                (* must begin with a letter or underscore *)

INTEGER     ::= decimalDigits
              | "0" octalDigits                    (* bare leading-zero octal *)
              | "0b" binaryDigits
              | "0o" octalDigits
              | "0x" hexDigits

FLOAT       ::= decimalDigits "." decimalDigits [ exponent ]

IMAGINARY   ::= ( decimalDigits | FLOAT ) "i"      (* complex128 imaginary literal, e.g. 3i, 2.5i *)

STRING      ::= '"' { unicodeChar } '"'
              | '`' { unicodeChar } '`'

RUNE        ::= "'" unicodeChar "'"
              | "'" unicodeChar unicodeChar { unicodeChar } "'"  (* extension: []int32 *)

BOOLEAN     ::= "true" | "false"
```

### 1.1 Reserved Words

The following identifiers are reserved and cannot be used as symbol names:

```text
break       case        const       continue    default
defer       else        fallthrough  for         func
go          if          import      interface   make
nil         package     panic       range       recover
return      switch      type        var
```

`panic` and `recover` are ordinary reserved words (not extension-gated),
matching Go. Most primitive type names are also reserved — including the
complex types `complex64` and `complex128` (see [Types](#6-types)).

### 1.2 Extension Reserved Words

These words are reserved only when language extensions are enabled
(see [Language Extensions](#9-language-extensions)):

```text
call        catch       exit        print       throw       try
```

---

## 2. Program Structure

```ebnf
program        ::= { statement }
```

A program is a flat sequence of statements. Executable statements (assignments,
control flow, function calls) must appear inside a function body. Declarations
(`const`, `import`, `package`, `type`, `var`) and function definitions are
legal at top level.

---

## 3. Statements

```ebnf
statement      ::= ";"
                 | "{}"
                 | directive
                 | funcDef
                 | declStmt
                 | execStmt

declStmt       ::= constDecl
                 | importDecl
                 | packageDecl
                 | typeDecl
                 | varDecl

execStmt       ::= assertStmt           (* extension *)
                 | assignStmt
                 | breakStmt
                 | callStmt             (* extension *)
                 | continueStmt
                 | deferStmt
                 | exitStmt             (* REPL / interactive mode only *)
                 | forStmt
                 | funcCallStmt
                 | goStmt
                 | ifStmt
                 | panicStmt            (* extension *)
                 | printStmt            (* extension *)
                 | returnStmt
                 | switchStmt
                 | throwStmt            (* extension *)
                 | tryStmt              (* extension *)
```

`"{}"` is treated as an empty struct value and discarded as a no-op statement.
Executable statements are only permitted inside a function body (or at the top
level in interactive/REPL mode).

---

## 4. Blocks

```ebnf
block          ::= "{" { statement } "}"
```

A block opens a new symbol scope. `if`, `for`, `switch`, `func`, `try`,
`catch`, and `else` all require a block body. The two-character token `"{}"` is
accepted as shorthand for an empty block.

---

## 5. Declarations

### 5.1 const

```ebnf
constDecl      ::= "const" ( constSpec
                           | "(" { constSpec [ ";" ] } ")" )

constSpec      ::= IDENTIFIER "=" expression
```

The right-hand side must be a constant expression; the compiler rejects any
reference to a non-constant symbol.

### 5.2 import

```ebnf
importDecl     ::= "import" ( importSpec
                            | "(" { importSpec [ ";" ] } ")" )

importSpec     ::= [ IDENTIFIER ] STRING
```

The optional leading identifier gives the imported package a local alias.
Standard Go path segments (e.g., `"os/exec"`) are remapped to their Ego names
(`"exec"`). Imports are only allowed at block depth zero.

### 5.3 package

```ebnf
packageDecl    ::= "package" IDENTIFIER
```

`package main` is recognized but treated as a no-op. Other package names affect
how the compiled unit is registered.

### 5.4 type

```ebnf
typeDecl       ::= "type" IDENTIFIER typeSpec
```

Defines a named type in the current scope. The new name becomes available in
subsequent type specifications within that scope.

### 5.5 var

```ebnf
varDecl        ::= "var" ( varSpec
                         | "(" { varSpec [ ";" ] } ")" )

varSpec        ::= identList typeSpec [ "=" initializer ]
                 | identList IDENTIFIER [ "." IDENTIFIER ]

identList      ::= IDENTIFIER { "," IDENTIFIER }
```

When no initializer is given, each declared variable receives the zero value for
its type. Multiple names may share a single type: `var x, y int`.

---

## 6. Types

```ebnf
typeSpec       ::= "*" typeSpec                        (* pointer *)
                 | "[" "]" typeSpec                    (* slice *)
                 | "map" "[" typeSpec "]" typeSpec
                 | "error"
                 | "interface" "{" "}"
                 | "interface{}"                       (* single-token form *)
                 | "any"
                 | "type"                              (* meta-type, extension *)
                 | funcTypeSpec
                 | interfaceTypeSpec
                 | structTypeSpec
                 | primitiveType
                 | IDENTIFIER                          (* previously declared user type *)
                 | IDENTIFIER "." IDENTIFIER           (* package-qualified type *)

primitiveType  ::= "bool" | "byte" | "chan"
                 | "int" | "int8" | "int16" | "int32" | "int64"
                 | "uint" | "uint8" | "uint16" | "uint32" | "uint64"
                 | "float32" | "float64"
                 | "complex64" | "complex128"
                 | "string"

funcTypeSpec   ::= "func" "(" [ paramList ] ")" [ returnTypes ]

interfaceTypeSpec
               ::= "interface" "{" { funcDecl [ ";" ] } "}"

structTypeSpec ::= "struct" "{" { structField [ ";" ] } "}"

structField    ::= identList typeSpec
                 | IDENTIFIER                          (* embedded type *)
                 | IDENTIFIER "." IDENTIFIER           (* embedded package type *)

funcDecl       ::= IDENTIFIER "(" [ paramList ] ")" [ returnTypes ]
```

The `complex64` and `complex128` types hold Go-style complex numbers. A value
is written either as an imaginary literal (`3i`, `2.5i`) added to a real part
(`3 + 4i`) or built with the `complex(real, imag)` builtin; the `real(c)` and
`imag(c)` builtins extract the components. A real number coerces implicitly to
a complex value (imaginary part 0) at assignment and call boundaries, but the
reverse always requires `real()`/`imag()`. Complex division by zero raises a
runtime error rather than yielding an IEEE Inf/NaN. The `cmplx` package
mirrors Go's `math/cmplx`.

---

## 7. Functions

```ebnf
funcDef        ::= "func" [ receiver ] IDENTIFIER paramDecl [ returnTypes ] block

funcLiteral    ::= "func" paramDecl [ returnTypes ] block [ "(" [ argList ] ")" ]

receiver       ::= "(" IDENTIFIER [ "*" ] IDENTIFIER ")"

paramDecl      ::= "(" [ paramList ] ")"

paramList      ::= paramGroup { "," paramGroup }

paramGroup     ::= identList [ "..." ] typeSpec

returnTypes    ::= returnItem
                 | "(" returnItem { "," returnItem } ")"

returnItem     ::= [ IDENTIFIER [ "*" ] ] typeSpec
```

Named return values (e.g., `func f() (result int, err error)`) are declared as
variables in the function body and are returned by a bare `return` statement.

The last parameter group may be variadic; prefix the type with `"..."` to
receive a variable number of arguments as a slice.

Function literals (anonymous functions / closures) may be immediately invoked
by appending a call argument list: `func(x int) int { return x+1 }(42)`.

---

## 8. Assignments

```ebnf
assignStmt     ::= lvalueList assignOp expression
                 | lvalueList "<-" expression          (* channel send *)
                 | lvalue incOp                        (* post-increment / decrement *)
                 | lvalue ":=" "<-" expression         (* receive and declare *)
                 | lvalue "="  "<-" expression         (* receive and assign *)

lvalueList     ::= lvalue { "," lvalue }

lvalue         ::= "*" lvalue                          (* pointer store *)
                 | IDENTIFIER { lvalueSuffix }

lvalueSuffix   ::= "." IDENTIFIER
                 | "[" expression "]"

assignOp       ::= "=" | ":=" | "+=" | "-=" | "*=" | "/="

incOp          ::= "++" | "--"
```

`_` is a valid discard target; the corresponding value is silently dropped.

Multi-target assignment (`a, b = f()`) requires the right-hand side to produce
exactly as many values as there are targets.

Compound operators (`+=`, `-=`, `*=`, `/=`) read the current value of the
target, apply the operation, and store the result.

---

## 9. Expressions

Operators are listed from lowest to highest precedence.

```ebnf
expression     ::= logicalOr

logicalOr      ::= logicalAnd { "||" logicalAnd }

logicalAnd     ::= relational { "&&" relational }

relational     ::= additive { relOp additive }

relOp          ::= "==" | "!=" | "<" | "<=" | ">" | ">="

additive       ::= multiplicative { addOp multiplicative }

addOp          ::= "+" | "-" | "|" | "<<" | ">>"

multiplicative ::= unary { mulOp unary }

mulOp          ::= "*" | "/" | "%" | "^" | "&"

unary          ::= "-" unary
                 | "!" unary
                 | funcOrRef

funcOrRef      ::= reference [ "(" [ argList ] ")" ]

reference      ::= atom { refSuffix }

refSuffix      ::= "." IDENTIFIER                              (* member access *)
                 | "." "(" typeSpec ")"                        (* type assertion *)
                 | "[" expression "]"                          (* index *)
                 | "[" [ expression ] ":" [ expression ] "]"   (* slice *)
                 | "(" [ argList ] ")"                         (* call *)
                 | "{" structFieldInits "}"                    (* struct initializer *)

argList        ::= argument { "," argument }

argument       ::= expression [ "..." ]
```

The `"..."` suffix on the last argument unpacks a slice into the variadic
parameter of the called function.

### 9.1 Atoms

```ebnf
atom           ::= macroInvocation
                 | ifExpression                        (* extension *)
                 | optionalExpression                  (* extension *)
                 | "nil"
                 | "{}"
                 | funcLiteral
                 | "&" addressTarget
                 | "*" atom
                 | "<-" reference                       (* channel receive *)
                 | "(" expression ")"
                 | arrayLiteral
                 | mapLiteral
                 | structLiteral
                 | INTEGER
                 | FLOAT
                 | IMAGINARY
                 | BOOLEAN
                 | RUNE
                 | STRING
                 | typeCast
                 | typeRef
                 | IDENTIFIER [ incOp ]

addressTarget  ::= IDENTIFIER [ "{" structFieldInits "}" ]
                 | atom

ifExpression   ::= "if" expression "{" expression "}" "else" "{" expression "}"

optionalExpression
               ::= "?" unary ":" unary                  (* extension *)
```

### 9.2 Composite literals

```ebnf
arrayLiteral   ::= "[" "]" typeSpec ( "{}" | "{" expressionList "}" )
                 | "[" expression ":" expression "]"   (* range: low to high *)
                 | "[" ":" expression "]"              (* range: 0 to high *)
                 | "[" expressionList "]"              (* inferred-type shorthand *)

mapLiteral     ::= "{" keyValuePair { "," keyValuePair } [ "," ] "}"

keyValuePair   ::= ( IDENTIFIER | STRING ) ":" expression

structLiteral  ::= "struct" "{" structFieldDecl { ";" structFieldDecl } "}"
                              "{" initializer "}"

structFieldInits
               ::= IDENTIFIER ":" expression { "," IDENTIFIER ":" expression }
                 | expressionList                       (* ordered form *)

expressionList ::= expression { "," expression }

typeCast       ::= typeSpec "(" expression { "," expression } ")"

typeRef        ::= typeSpec [ "{" structFieldInits "}" | "{}" ]
```

Range array literals (`[1:5]` → `[1,2,3,4,5]`) generate a sequence of integer
constants. If the low end is greater than the high end the sequence counts down.

### 9.3 Initializers

Initializers appear in `var` declarations and composite-literal bodies.

```ebnf
initializer    ::= structInitializer
                 | mapInitializer
                 | arrayInitializer
                 | expression

structInitializer
               ::= "{" namedFieldList "}"
                 | "{" orderedFieldList "}"

namedFieldList ::= IDENTIFIER ":" initializer { "," IDENTIFIER ":" initializer } [ "," ]

orderedFieldList
               ::= initializer { "," initializer } [ "," ]

mapInitializer ::= "{" keyValuePair { "," keyValuePair } [ "," ] "}"

arrayInitializer
               ::= "{" initializer { "," initializer } [ "," ] "}"
```

---

## 10. Control Flow

### 10.1 if

```ebnf
ifStmt         ::= "if" [ initStmt ";" ] expression block [ "else" ( ifStmt | block ) ]

initStmt       ::= assignStmt
```

The optional `initStmt` declares a variable that is scoped to the `if`
statement and its `else` branches: `if x := f(); x != nil { … }`.

### 10.2 for

```ebnf
forStmt        ::= "for" block                                      (* infinite loop *)
                 | "for" expression block                           (* conditional loop *)
                 | "for" IDENTIFIER [ "," IDENTIFIER ] ":=" "range" expression block
                 | "for" lvalue ":=" expression ";" expression ";" forIncr block

forIncr        ::= lvalue incOp
                 | lvalue "=" expression
```

All four forms use `break` and `continue` to exit or advance the loop.

An infinite `for { … }` requires at least one reachable `break` statement.

In range loops, `_` can be used as a discard for either the index or the value:
`for _, v := range slice { … }`.

A `for` statement can be preceded by a label which identifies the specific loop
in a set of nested loops, and is used by the `break` and `continue` statements.
This is the only place that a statement label is permitted in Ego.

```ebnf
label          ::= IDENTIIFER ":"
```

### 10.3 switch

```ebnf
switchStmt     ::= "switch" [ switchInit ] "{" switchBody "}"

switchInit     ::= IDENTIFIER ":=" expression   (* assigns and switches on value *)
                 | expression                   (* switches on value *)
                                                (* omitted: conditional switch *)

switchBody     ::= { caseClause | defaultClause }

caseClause     ::= "case" expression ":" { statement } [ "fallthrough" [ ";" ] ]

defaultClause  ::= "default" ":" { statement }
```

When `switchInit` is omitted, each `case` expression is evaluated as a boolean
condition (analogous to `switch true` in Go). Otherwise, each `case` value is
compared with the switch expression for equality.

`fallthrough` transfers control to the first statement of the next case body,
skipping that case's condition check (same as Go).

### 10.4 return

```ebnf
returnStmt     ::= "return" [ expression { "," expression } ]
```

A bare `return` inside a function with named return variables implicitly returns
the current values of those variables.

### 10.5 break and continue

```ebnf
breakStmt      ::= "break" [ label ]
continueStmt   ::= "continue" [ label ]
```

Both are only legal inside a `for` loop body. A bare `continue` or `break` affects
the innermost loop. If an optional label is provided, the loops unwind to the `for`
loop at the given label. Note, labels can _only_ be used to label `for` loop statements.

### 10.6 defer

```ebnf
deferStmt      ::= "defer" ( funcLiteral "(" [ argList ] ")"
                           | funcCallExpr )
```

Deferred calls execute in LIFO order when the enclosing function returns,
identical to Go semantics.

### 10.7 go

```ebnf
goStmt         ::= "go" ( funcLiteral "(" [ argList ] ")"
                        | funcCallExpr )
```

Launches the call as a goroutine. The `@wait` directive (see
[Directives](#12-directives)) blocks until all outstanding goroutines finish.

---

## 11. Extension Statements

These statements are available only when language extensions are enabled via
`@extensions true` or the `ego.compiler.extensions` profile setting.

### 11.1 try / catch

```ebnf
tryStmt        ::= "try" block [ "catch" [ "(" IDENTIFIER ")" ] block ]
```

If the `try` block raises a runtime error, the `catch` block executes. If an
identifier is given in parentheses after `catch`, it is bound to the error
value inside the catch block; otherwise the error is available as `_error_`.

### 11.2 panic

```ebnf
panicStmt      ::= "panic" "(" [ expression ] ")"
```

Raises a runtime error, unwinding through any enclosing `try/catch` blocks.
With no argument, panics with a default message.

### 11.3 print

```ebnf
printStmt      ::= "print" [ expression { "," expression } [ "," ] ]
```

Writes its arguments to standard output, separated by spaces. A trailing comma
suppresses the automatic newline.

### 11.4 call

```ebnf
callStmt       ::= "call" funcCallExpr
```

Calls a function and discards all return values. Useful when the called
function returns multiple values that would otherwise require explicit targets.

### 11.5 assert

```ebnf
assertStmt     ::= "assert" "(" expression [ "," expression ] ")"
```

Only valid in test mode (programs run via `ego test`). Fails the current test
case if the first expression is not `true`. The optional second expression is a
string message included in the failure report.

In current practice, `assert` is not lexed as a reserved word, so test
assertions are written with the [`@assert`](#12-directives) directive
(`@assert expr`) rather than this statement form.

### 11.6 exit

```ebnf
exitStmt       ::= "exit" [ expression ]
```

Available in interactive/REPL mode only. Terminates the process with the given
integer status code (default 0).

### 11.7 throw

```ebnf
throwStmt      ::= "throw" expression
```

The expression must evaluate to an `error` value. If it is non-nil, it is
raised as a catchable runtime error — exactly as if a genuine runtime error
(such as division by zero) had occurred — so it can be caught by an
enclosing `try`/`catch`. If the expression evaluates to `nil` (or an Ego
zero-value error), nothing happens and execution continues with the next
statement. This lets `throw err` stand in for Go's `if err != nil { return
err }` idiom, collapsed into a single statement, when the intent is purely
to propagate an error condition.

---

## 12. Directives

```ebnf
directive      ::= "@" IDENTIFIER { directiveArg }
```

Directives are processed at compile time. They begin with `@` followed
immediately by an identifier. Arguments vary by directive. Unknown directive
names are treated as user-defined macro invocations (see [Macros](#13-macros)).

| Directive | Arguments | Purpose |
| --------- | --------- | ------- |
| `@assert` | `expression` | Assertion in test mode |
| `@authenticated` _(deprecated)_ | `none` \| `user` \| `admin` \| `root` | Require authentication (server mode); superseded by `@endpoint`'s own `authenticated`/`admin`/`root`/`permissions=` terms |
| `@capture` | `IDENT ( ":=" \| "=" ) block` | Capture a block's console output into a variable — see [12.1](#121-capture) |
| `@compile` | `{ flag } ( block \| eof-delimited code ) [ catch ]` | Compile a block or program fragment in an isolated sub-compiler — see [12.2](#122-compile) |
| `@debug` | — | Enable debug logging |
| `@define` | `IDENT { "," IDENT }` | Pre-declare global variables |
| `@dump_errors` | — | Dump the error-message catalog (localization `error.` class) |
| `@endpoint` | `[ method ] [ "path=" ] STRING [ "media=" STRING { "," STRING } ] [ "permissions=" STRING { "," STRING } ] [ "parameter=" STRING { "," STRING } ] [ "authenticated" \| "admin" \| "root" ]` | Declare service endpoint path, method, media types, permissions, and parameter validation — see [`@endpoint`](SERVER.md#directives) in SERVER.md |
| `@entrypoint` | `[ IDENT ]` | Set entry-point function name (default `main`) |
| `@error` | `[ expression ]` | Signal a compile-time error |
| `@extensions` | `true` \| `false` \| `default` \| _(empty = true)_ | Toggle language extensions |
| `@fail` | `[ expression ]` | Raise a fatal test failure |
| `@file` | `name` | Override the recorded source file name |
| `@global` | `IDENT [ expression ]` | Create or set a global variable |
| `@handler` | `[ IDENT ]` | Declare an HTTP request handler |
| `@json` | `statement` | Execute statement only when response format is JSON |
| `@line` | `INTEGER` | Override the recorded source line number |
| `@localization` | `mapLiteral` | Set a runtime localization table |
| `@log` | `IDENT [ expression ]` | Emit a log entry at the named level |
| `@optimizer` | `on \| off \| always` | Enable/disable the bytecode optimizer for subsequently compiled code — see [12.3](#123-optimizer) |
| `@package` | `IDENT { "," IDENT \| "*" }` | Dump package symbol information |
| `@packages` | — | Dump all loaded package names |
| `@profile` | `start\|enable\|on \| stop\|disable\|off \| report \| dump\|print` | Control the runtime profiler |
| `@sandbox` | `( true \| false ) [ "path=" expression ]` | Set the filesystem sandbox (test mode only) — see [12.4](#124-sandbox) |
| `@status` | `expression` | Set the HTTP response status code |
| `@symbols` | `[ expression ]` | Dump the current symbol table |
| `@template` | `IDENT expression` | Compile a text template |
| `@test` | `STRING` | Begin a named test case |
| `@text` | `statement` | Execute statement only when response format is plain text |
| `@type` | `strict` \| `relaxed` \| `dynamic` | Set type-checking mode |
| `@validation` | `dump` | Dump validation information |
| `@wait` | — | Block until all goroutines have finished |

### 12.1 @capture

```ebnf
captureDirective ::= "@" "capture" IDENTIFIER ( ":=" | "=" ) block
```

Runs `block` and collects everything its statements would otherwise have
printed to the console (via `fmt.Println`, `fmt.Printf`, the `print`
extension statement, etc.) into a single string, instead of letting it reach
the console. On success, the collected string is stored into `IDENTIFIER`:
`":="` declares a new variable (an error if it already exists in the current
scope, exactly as ordinary `:=` requires); `"="` assigns to an existing
variable and is checked for existence at compile time. `IDENTIFIER` may be
`_`, in which case the captured text is discarded on success rather than
stored anywhere — this is a shorthand for tests that only wrap a block to
keep its output out of the console/log, with nothing to inspect afterward.

If `block` raises a runtime error, capturing stops, the text collected up
to that point is printed to the console (headed `@capture output:`, or
`@capture _:` when the variable is `_`) so it isn't silently lost, and the
same error is re-raised (as if by [`throw`](#117-throw)) so it continues
propagating to any enclosing `try`/`catch`.

### 12.2 @compile

```ebnf
compileDirective ::= "@" "compile" { compileFlag } compileBody [ catchClause ]

compileFlag      ::= "block"
                    | "bytecode" | "disasm"
                    | "unused" "=" boolLiteral
                    | "unknown" "=" boolLiteral
                    | "optimize" "=" optimizeLevel
                    | "eof" "=" STRING

boolLiteral      ::= "true" | "false" | "on" | "off" | "1" | "0"
optimizeLevel    ::= "off" | "false" | "low" | "high" | INTEGER

compileBody      ::= block
                    | { token }

catchClause      ::= "catch" [ "(" IDENTIFIER ")" ] block
```

Compiles `compileBody` in an isolated sub-compiler and, if compilation
succeeds, splices the resulting bytecode inline; the directive acts like a
`try`/`catch` wrapped around compilation itself, so `catchClause` (if
present) runs on a compile error instead of the compile error aborting the
whole program. This is used mainly to write unit tests for the compiler.

`compileBody`'s form depends on whether the `eof=` flag was given: with no
`eof=` flag, `compileBody` is the usual brace-delimited `block`. With
`eof=` given, there are no braces at all — `compileBody` is every token
that follows, up to (but not including) a run of tokens whose spellings,
concatenated together, exactly match the marker string. This lets a test
exercise code with intentionally mismatched braces, which brace-counting
could never delimit correctly.

The flags, which may appear in any combination and order before
`compileBody`:

| Flag | Values | Description |
| ---- | ------ | ----------- |
| `block` | _(bare)_ | The code is a statement block rather than a full program (no `package`/`func main` prolog required). |
| `bytecode` (alias `disasm`) | _(bare)_ | Print a disassembly of the bytecode generated for this block once it compiles successfully. |
| `unused` | `boolLiteral` | Override the "unused variable is a compile error" setting for this compilation only. |
| `unknown` | `boolLiteral` | Override the "unknown symbol is a compile error" setting for this compilation only. |
| `optimize` | `optimizeLevel` | Override the compiler optimization level for this compilation only. `off`/`false`/`0` disables it, `low`/`1` optimizes conditionally, `high`/`2` always optimizes. |
| `eof` | `STRING` | Delimit `compileBody` with a text marker instead of `{ }` braces (see above). |

### 12.3 @optimizer

```ebnf
optimizerDirective ::= "@" "optimizer" optimizerMode

optimizerMode      ::= "on" | "true" | "1"        (* conditional optimization *)
                     | "off" | "false" | "0"      (* disable *)
                     | "always"                   (* force optimization: level 2 *)
```

Sets the bytecode-optimizer level for code compiled after the directive. The
setting persists until changed again or until the compilation unit ends. Under
`ego test` the optimizer is off by default; use `@optimizer on` in a test file
only when exercising optimizer-specific behavior.

### 12.4 @sandbox

```ebnf
sandboxDirective ::= "@" "sandbox" ( "true" | "false" ) [ "path=" expression ]
```

Valid only in test mode (`ego test`). Enables or disables the filesystem I/O
sandbox for subsequent statements. The optional `path=` clause is any
expression (not just a string literal), letting a test compute the sandbox
root — for example from a temp-directory helper. When `path=` is omitted the
previously configured sandbox root is left untouched; an explicit `path=""`
clears it.

---

## 13. Macros

```ebnf
macroInvocation ::= "@" IDENTIFIER { token }
```

When a directive name is not one of the built-in names listed above, the
compiler looks for a function of that name in the `macros` package. The
function receives the remaining tokens on the line as a string slice and returns
a source fragment that is spliced back into the token stream for further
compilation.

Macro invocations may appear both at statement level (as directives) and inside
expressions (as atoms).

---

## 14. Type Assertions and Unwrap

```ebnf
typeAssertion  ::= expression "." "(" typeSpec ")"

unwrapAssign   ::= IDENTIFIER "," IDENTIFIER ":=" expression "." "(" typeSpec ")"
```

The two-value form `v, ok := x.(T)` (analogous to Go's comma-ok pattern)
assigns the converted value to `v` and sets `ok` to `true` if the conversion
succeeded, or the zero value and `false` if it did not.

---

## 15. Channels

```ebnf
channelSend       ::= lvalue "<-" expression

channelReceive     ::= lvalue ":=" "<-" reference
                     | lvalue "="  "<-" reference

channelReceiveOK   ::= lvalue "," lvalue ":=" "<-" reference

channelReceiveAtom ::= "<-" reference
```

Channel operations use the same `<-` operator syntax as Go. Channels are
created with `make(chan type)`.

`<-` also works as a general expression atom (`channelReceiveAtom`, part of
the `atom` production in [9.1](#91-atoms)) — as a function-call argument, an
operand of another operator, an array-literal element, and so on — not just
as the direct right-hand side of an assignment statement:

```go
fmt.Println(<-ch)          // as a function-call argument
total := 10 + <-ch         // as an operand
total := <-ch + 10         // "<-" binds tighter than "+"
arr := []int{<-ch, 1, 2}   // as an array-literal element
```

In every position, `<-` binds only as tightly as `reference` (an atom plus
any `.member`/`[index]`/`(args)` suffix chain) — it does **not** extend to
swallow a following binary operator. `channelReceiveOK` (the two-value
"comma-ok" form, `v, ok := <-ch`) is the one exception: its right-hand side
must be exactly `<-ch` with nothing else, matching Go's own grammar for that
form.

---

## 16. Differences from Go

Ego deliberately mirrors Go syntax. The following is a summary of where Ego
diverges from the Go specification:

| Feature | Go | Ego |
| ------- | -- | --- |
| Type system | Static, enforced at compile time | Dynamic by default; `@type strict` enables static checking |
| `try`/`catch` | Not present; uses `panic`/`recover` | Present as an extension statement |
| `print` statement | Not present | Present as an extension statement |
| `call` statement | Not present | Present as an extension statement |
| `assert` statement | Not present | Present in test mode |
| `if` expression | Not present | `if cond { a } else { b }` as atom, extension |
| Optional expression | Not present | `?expr : default` traps runtime errors, extension |
| Range array literal | Not present | `[1:5]` → `[]int{1,2,3,4,5}` |
| Multi-char rune literal | Not present | `'abc'` → `[]int32{…}`, extension |
| Complex numbers | `complex64`/`complex128`, `3i` literals, `complex`/`real`/`imag` | Same types and builtins; but real→complex coerces implicitly, and complex ÷ 0 raises a runtime error instead of Inf/NaN |
| `exit` statement | Not present | Present in REPL / interactive mode |
| Directives (`@...`) | Not present | Compile-time directives for server, test, and tool use |
| Goroutine tracking | Manual | `@wait` directive blocks until all goroutines finish |
| `iota` in `const` | Supported | Not supported |
| Labeled `break`/`continue` | Supported | Supported |
| Multiple `return` | Supported | Supported |
| Named return values | Supported | Supported |
| Variadic functions | Supported | Supported |
| Method receivers | Supported | Supported (pointer receivers with `*`) |
| Closures | Supported | Supported |
| Goroutines | Supported | Supported |
| Channels | Supported | Supported |
| `defer` | Supported | Supported |
| Interfaces | Supported | Supported (structural) |
| Embedding | Supported | Supported in structs |

---

## 17. Operator Precedence Summary

From lowest to highest:

| Level | Operators |
| ----- | --------- |
| 1 (lowest) | `\|\|` |
| 2 | `&&` |
| 3 | `==` `!=` `<` `<=` `>` `>=` |
| 4 | `+` `-` `\|` `<<` `>>` |
| 5 | `*` `/` `%` `^` `&` |
| 6 | Unary `-` `!` |
| 7 (highest) | `.` `[…]` `(…)` (member, index, call) |

---

## 18. Complete Grammar (compact form)

```ebnf
(* --- Top level --- *)
program          ::= { statement }
statement        ::= ";" | "{}" | directive | funcDef | declStmt | execStmt
declStmt         ::= constDecl | importDecl | packageDecl | typeDecl | varDecl
execStmt         ::= assignStmt | breakStmt | callStmt | continueStmt
                   | deferStmt | exitStmt | forStmt | funcCallStmt | goStmt
                   | ifStmt | panicStmt | printStmt | returnStmt
                   | switchStmt | throwStmt | tryStmt | assertStmt

(* --- Declarations --- *)
constDecl        ::= "const" ( constSpec | "(" { constSpec [ ";" ] } ")" )
constSpec        ::= IDENTIFIER "=" expression
importDecl       ::= "import" ( importSpec | "(" { importSpec [ ";" ] } ")" )
importSpec       ::= [ IDENTIFIER ] STRING
packageDecl      ::= "package" IDENTIFIER
typeDecl         ::= "type" IDENTIFIER typeSpec
varDecl          ::= "var" ( varSpec | "(" { varSpec [ ";" ] } ")" )
varSpec          ::= identList typeSpec [ "=" initializer ]
identList        ::= IDENTIFIER { "," IDENTIFIER }

(* --- Types --- *)
typeSpec         ::= "*" typeSpec | "[" "]" typeSpec
                   | "map" "[" typeSpec "]" typeSpec
                   | "error" | "interface{}" | "any" | "type"
                   | funcTypeSpec | interfaceTypeSpec | structTypeSpec
                   | primitiveType | IDENTIFIER | IDENTIFIER "." IDENTIFIER
primitiveType    ::= "bool" | "byte" | "chan" | "int" | "int8" | "int16"
                   | "int32" | "int64" | "uint" | "uint8" | "uint16"
                   | "uint32" | "uint64" | "float32" | "float64"
                   | "complex64" | "complex128" | "string"
funcTypeSpec     ::= "func" "(" [ paramList ] ")" [ returnTypes ]
interfaceTypeSpec::= "interface" "{" { funcDecl [ ";" ] } "}"
structTypeSpec   ::= "struct" "{" { structField [ ";" ] } "}"
structField      ::= identList typeSpec | IDENTIFIER | IDENTIFIER "." IDENTIFIER
funcDecl         ::= IDENTIFIER "(" [ paramList ] ")" [ returnTypes ]

(* --- Functions --- *)
funcDef          ::= "func" [ receiver ] IDENTIFIER paramDecl [ returnTypes ] block
funcLiteral      ::= "func" paramDecl [ returnTypes ] block [ "(" [ argList ] ")" ]
receiver         ::= "(" IDENTIFIER [ "*" ] IDENTIFIER ")"
paramDecl        ::= "(" [ paramList ] ")"
paramList        ::= paramGroup { "," paramGroup }
paramGroup       ::= identList [ "..." ] typeSpec
returnTypes      ::= returnItem | "(" returnItem { "," returnItem } ")"
returnItem       ::= [ IDENTIFIER [ "*" ] ] typeSpec

(* --- Assignments --- *)
assignStmt       ::= lvalueList assignOp expression
                   | lvalueList "<-" expression
                   | lvalue incOp
                   | lvalue ( ":=" | "=" ) "<-" expression
lvalueList       ::= lvalue { "," lvalue }
lvalue           ::= "*" lvalue | IDENTIFIER { lvalueSuffix }
lvalueSuffix     ::= "." IDENTIFIER | "[" expression "]"
assignOp         ::= "=" | ":=" | "+=" | "-=" | "*=" | "/="
incOp            ::= "++" | "--"

(* --- Expressions --- *)
expression       ::= logicalOr
logicalOr        ::= logicalAnd { "||" logicalAnd }
logicalAnd       ::= relational { "&&" relational }
relational       ::= additive { relOp additive }
relOp            ::= "==" | "!=" | "<" | "<=" | ">" | ">="
additive         ::= multiplicative { addOp multiplicative }
addOp            ::= "+" | "-" | "|" | "<<" | ">>"
multiplicative   ::= unary { mulOp unary }
mulOp            ::= "*" | "/" | "%" | "^" | "&"
unary            ::= ( "-" | "!" ) unary | funcOrRef
funcOrRef        ::= reference [ "(" [ argList ] ")" ]
reference        ::= atom { refSuffix }
refSuffix        ::= "." IDENTIFIER | "." "(" typeSpec ")"
                   | "[" expression "]"
                   | "[" [ expression ] ":" [ expression ] "]"
                   | "(" [ argList ] ")"
                   | "{" structFieldInits "}"
argList          ::= argument { "," argument }
argument         ::= expression [ "..." ]

(* --- Atoms --- *)
atom             ::= macroInvocation | ifExpression | optionalExpression
                   | "nil" | "{}" | funcLiteral
                   | "&" addressTarget | "*" atom | "<-" reference
                   | "(" expression ")"
                   | arrayLiteral | mapLiteral | structLiteral
                   | INTEGER | FLOAT | IMAGINARY | BOOLEAN | RUNE | STRING
                   | typeCast | typeRef | IDENTIFIER [ incOp ]
ifExpression     ::= "if" expression "{" expression "}" "else" "{" expression "}"
optionalExpression
                 ::= "?" unary ":" unary
arrayLiteral     ::= "[" "]" typeSpec ( "{}" | "{" expressionList "}" )
                   | "[" [ expression ] ":" expression "]"
                   | "[" expressionList "]"
mapLiteral       ::= "{" keyValuePair { "," keyValuePair } [ "," ] "}"
keyValuePair     ::= ( IDENTIFIER | STRING ) ":" expression
structLiteral    ::= "struct" "{" structField { ";" structField } "}" "{" initializer "}"
structFieldInits ::= IDENTIFIER ":" expression { "," IDENTIFIER ":" expression }
                   | expressionList
expressionList   ::= expression { "," expression }
typeCast         ::= typeSpec "(" expression { "," expression } ")"
typeRef          ::= typeSpec [ "{" structFieldInits "}" | "{}" ]
macroInvocation  ::= "@" IDENTIFIER { token }
addressTarget    ::= IDENTIFIER [ "{" structFieldInits "}" ] | atom

(* --- Initializers --- *)
initializer      ::= "{" namedFieldList "}" | "{" orderedFieldList "}"
                   | "{" keyValuePair { "," keyValuePair } [ "," ] "}"
                   | expression
namedFieldList   ::= IDENTIFIER ":" initializer { "," IDENTIFIER ":" initializer } [ "," ]
orderedFieldList ::= initializer { "," initializer } [ "," ]

(* --- Control flow --- *)
block            ::= "{" { statement } "}" | "{}"
ifStmt           ::= "if" [ assignStmt ";" ] expression block
                     [ "else" ( ifStmt | block ) ]
forStmt          ::= "for" block
                   | "for" expression block
                   | "for" IDENTIFIER [ "," IDENTIFIER ] ":=" "range" expression block
                   | "for" lvalue ":=" expression ";" expression ";" forIncr block
forIncr          ::= lvalue incOp | lvalue "=" expression
switchStmt       ::= "switch" [ switchInit ] "{" switchBody "}"
switchInit       ::= IDENTIFIER ":=" expression | expression
switchBody       ::= { caseClause | defaultClause }
caseClause       ::= "case" expression ":" { statement } [ "fallthrough" [ ";" ] ]
defaultClause    ::= "default" ":" { statement }
returnStmt       ::= "return" [ expression { "," expression } ]
breakStmt        ::= "break"
continueStmt     ::= "continue"
deferStmt        ::= "defer" ( funcLiteral "(" [ argList ] ")" | funcCallExpr )
goStmt           ::= "go"   ( funcLiteral "(" [ argList ] ")" | funcCallExpr )

(* --- Extension statements --- *)
tryStmt          ::= "try" block [ "catch" [ "(" IDENTIFIER ")" ] block ]
panicStmt        ::= "panic" "(" [ expression ] ")"
printStmt        ::= "print" [ expression { "," expression } [ "," ] ]
callStmt         ::= "call" funcCallExpr
assertStmt       ::= "assert" "(" expression [ "," expression ] ")"
exitStmt         ::= "exit" [ expression ]
throwStmt        ::= "throw" expression

(* --- Directives --- *)
directive        ::= "@" IDENTIFIER { directiveArg }
captureDirective ::= "@" "capture" IDENTIFIER ( ":=" | "=" ) block
compileDirective ::= "@" "compile" { compileFlag } compileBody [ catchClause ]
compileFlag      ::= "block" | "bytecode" | "disasm"
                   | "unused" "=" boolLiteral
                   | "unknown" "=" boolLiteral
                   | "optimize" "=" optimizeLevel
                   | "eof" "=" STRING
boolLiteral      ::= "true" | "false" | "on" | "off" | "1" | "0"
optimizeLevel    ::= "off" | "false" | "low" | "high" | INTEGER
compileBody      ::= block | { token }
catchClause      ::= "catch" [ "(" IDENTIFIER ")" ] block
optimizerDirective ::= "@" "optimizer" ( "on" | "true" | "1"
                                       | "off" | "false" | "0" | "always" )
sandboxDirective ::= "@" "sandbox" ( "true" | "false" ) [ "path=" expression ]
```
