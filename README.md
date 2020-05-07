# solve
Simple expression solver that uses gopackages/expressions. The build command line tool
can be used to solve expressions, reference environment variables, and use built-in functions.

Because the arguments are passed in from a shell, they may have to be escaped to support
the expression syntax for functions, strings, etc.

Example:

    
    solve "3*5"
    
This prints the value "15". The quotes are required because the "*" character is a special
character in the shell language.

In addition to the built-in functions provided by the expressions package, this also demonstrates
how to add a new function to the available functions.

## pi()
This simple function accepts no arguments, and returns a float64 value for pi. It returns no errors.

## sum()
This is a more complex function that handles a variable argument list, of heterogenous types. The
function scans over the argument list, and performs a sum operation on the arguments. The type of the
first argument in the sum() parameter list deterimes the return type.

Each argument is coerced to match the type of the first arguemnt (supported types are int, float64, bool,
and string). A type-switch is used to select the appropriate summation operation to perform based on the
type of the value.

The result of the summations is returned. This function returns no errors.

