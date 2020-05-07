# solve
Simple expression solver that uses gopackages/expressions. The build command line tool
can be used to solve expressions, reference environment variables, and use built-in functions.

Because the arguments are passed in from a shell, they may have to be escaped to support
the expression syntax for functions, strings, etc.

Example:

    
    solve "3*5"
    
This prints the value "15". The quotes are required because the "*" character is a special
character in the shell language.
