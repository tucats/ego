# Expressions

This part of the package handles compilation and evaluation of arbitrary expressions written as 
text in a string value. The expression is evaluated using the rules for precedence and syntax of
most common languages.

## Overview

The expression handler supports values of type int, float64, bool, and string. The evaluator 
provides automatic type conversion where ever possible. The expression evaluator 
can also access values stored in symbols, which are passed in as a symbols.SymbolTable 
object to the evaluator. Additionally, built-in and caller-supplied functions can be declared 
in the symbol table as well. Functions accept arbitrary numbers of arguments of any type, 
and then operate on them, performing type coercions as needed.

Here is a simple example of using the expression handler:

    // Create a symbol table for use during expression
    // evaluation. This is optional, but must be provided
    // if your expression uses variables.
    symbols := symbols.SymbolTable()
    symbols.SetAlways("name", "Tom")
    symbols.SetAlways("age", 54)

    // Compile a string as an expression and then evaluate
    // the resulting expression to get its value
    e := expressions.New().WithText("age + 10")
    v, err := e.Eval(symbols)
  
The value of the expression is returned as an opaque interface, along with an error object. If the
error object is nil, no errors occurred during expression evaluation. If the err object is not nil,
it contains an error description, and the value returned will be set to nil.

Note the user of the `SetAlways()` method to set symbol values. This will set a value even if
the value doesn't already exist, and will also set a value whose name starts with "_" which
denotes a read-only variable.

It is up to the caller to handle the type of the return value. A number of functions exist to fetch
specific types from the opaque value, performing type conversions as needed:

* GetInt()
* GetFloat()
* GetBool()
* GetString()

## Constants
You can specify any base constant type, array, or struct directly:

|Type|Example|Description|
|:----:|-------|-----------|
|int | 42 | Signed integer value |
|float | 3.14 | Signed floating point value, requires the "." |
|bool| false | The boolean values `true` or `false` |
|string | "hello" | Characters enclosed in quotes. Supports escapes like `\n` |
|array | ["Tom", 55] | Comma-separated list of values in `[]` |
| struct | {name:"Tom", age:55} | Comma-separated list of name/value pairs in `{}` |

You can specify a constant array of integer values by specifying a range as well. For example,
the expression `[1:10]` is identical to `[1,2,3,4,5,6,7,8,9,10]`. You can omit the first value
if it is one; i.e. `[:3]` is the same as `[1,2,3]`. If the range is expressed as a high value
to a low value, the values are in reverse order; i.e. `[7:5]` is `[7,6,5]`.


## Symbols
As shown in the simple example above, you can provide a map of symbols available to the
expression evaluator. All symbol names should be stored as lower-case names as the symbol
table is case-insensitive and all symbol references in the expression are converted to
lower case.

The value of the symbol is any value of the supported types. When that symbol is referenced
in an expression, the map is used to locate the value to insert into the expression evaluator 
at that point. 

The symbol table is passed to the Eval() method to allow a single expression to be used with
many possible values. After the symbol table is created or updated, a new evaluation can be
run using the previoiusly parsed expression.

This is used in github.com/tucats/ego/app-cli/tables for example, to support a filter
expression in the tables.SetWhere() method. When an expression is set on a table, the table
printing operation will only show rows for which the where-clause results in a boolean "true"
value. The symbol table is refreshed with each row of the table (the symbol names are taken
from the table column names) so the expression can be re-evaluated with each row.

## Functions
The expression evaluator can process function calls as part of it's processing. The function call
is directed back to code that is either built-in to the expression package, or is supplied by the
user. A function may exist globally (for example, `len()`) or may be part of a package that requires
the package name be include (such as `strings.lower()`). In each case below, the function name
is given with the package name where it exists.

See the README.md file in the `functions` package for a description of each function.

### User Supplied Functions
The caller of the expressions package can supply additional functions to
supplement the built-in functions.  The function must be declared as a
function of type func([]interface{})(interface{}, error).  For example,
this is a simplified function that creates a floating point sum of all
the supplied values (which will be type-coerced to be floats):
    
    func sum( args []interface{})(interface{}, error) {
        result := 0
        for _, v := range args {
            result = result + util.GetInt(v)
        }
        return result, nil
    }

The body of the function operates on the argument list, processing values
as appropriate for the function. The service functions GetInt, GetFloat,
GetString, and GetBool can be used to get the value of an opaque argument
and coerce it to the desired type. The function can also implement type
switch statements to handle other argument types.

The result is always returned as an int, float64, string, or bool. In
addition, if the function encounters an error (for example, an incorrect
number of arguments passed in) then an error should be returned. When an
error is returned, the function result is ignored and the expression
evaluation reports an error.

To declare the function to the expression evaluator, just add it to the
symbol table. The name must be the name of the function as it would be
specified in an expression. The value of the item
in the symbol table map is the function pointer or value itself.
    
    symbols.SetAlways("sum", sum)

This will add the sum function described above to the available symbols
for processing an expression. Note that user-supplied functions must be
put in any symbol table where the function is expected to be used; 
user-defined function definitions do not persist past the Eval() call.
