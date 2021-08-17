# functions

The `functions` package contains the builtin functions (some of which are global and
some of which are arranged into packages). The following provides a short summary of 
each of the built-in functions, broken into categories based on the general data types 
or functionality used.

## Type Casting Functions

These functions are used to explicity specify the type of a value to be used in the
evaluation of the expression. They take an arbitrary value and return that value
coerced to a function-specific type.

### int(any)
Return the argument coerced to an int data type. For a boolean, this
will result in 0 or 1. For a float64, it returns the integer component.
A string must contain a valid representation of an integer to convert
without error.

    int(33.5)

This returns the value 33.

### bool(any)
Return the argument coerced to a bool data type. For numeric values,
this means zero for false, or non-zero for true. For a string, it must
contain the strings "true" or "false" to be converted without error.

    bool("true")

This returns the value true.

### float64(any)
Return the argument coerced to an float64 data type. For a boolean, this
will result in 0.0 or 1.0 values. For an, it returns the floating point
equivalent of the integer value.
A string must contain a valid representation of an floating point value to convert
without error.
    
    float64("3.1415")

Thsi returns the float64 value 3.1415.

## String Functions

These functions act on string values, and usually return a string values as the
result.

### strings.Format(fmtstring, values...)
This returns a `string` containing the formatted value of the values array, using
the Go-style `fmtstring` value. This supports any Go-style formatting.

### strings.Left(string, count)
This returns `count` characters from the left side of the string.
    
    strings.Left("abraham", 3)

This returns the value "abr".


### strings.Right(string, count)
This returns `count` characters from the right side of the string.
    
    strings.Right("abraham", 4)

This returns the value "aham".

### strings.Substring(string, start, count)
This extracts a substring from the string argument. The substring
starts at the `start` character position, and includes `count` characters
in the result.
    
    j := strings.Substring("Thomas Jefferson", 8, 4)

This returns the string "Jeff".

### strings.Index(string, substring)
This searches the `string` parameter for the first instance of the
`substring` parameter. If it is found, the function returns the
character position where it starts. If it was not found, it returns
an integer zero.
    
    strings.Index("Scores of fun", "ore")

This returns the value 3, indicating that the string "ore" starts
at the third character of the string.

### strings.Lower(string)
This converts the string value given to lower-case letters. If the
value is not a string, it is first coerced to a string type.
    
    strings.Lower("Tom")

This results in the string value "tom".

### strings.Upper(string)
This converts the string value given to uooer-case letters. If the
value is not a string, it is first coerced to a string type.
    
    strings.Upper("Jeffrey")

This results in the string value "JEFFREY".

### strings.Tokenize(string)
This converts a string into an array of strings, tokenized using the same
syntax rules that the `Ego` language uses itself. An empty string results
in an empty token array.

## General Functions

These functions work generally with any type of value, and perform coercsions
as needed. The first value in the argument list determines the type that all
the remaining items will be coerced to.

### append(array, items...)
You can use the `append` function to add items to an array. The first argument
is the source array to which data is to be appended. If the first argument is
not an array, a new array is constructed containing that value.

The subsequent arguments are added to the array, and the resulting array is
returned as the new value. If you add in an array, the array becomes a single
new member of the resulting array. You can use the `...` operator after the
array name to cause it to be flattened out to be individual arguments, as if
you has passed in each array member independantly. 

### delete(v, k)
The `delete` function is used to efficiently remove an element from an array, 
or remove a field from a structure. The second argument is either the zero-based
array index to delete, or a string value that describes the name of the field
to be removed from the structure.

    a := [101, 102, 103, 104]
    b := delete(a, 2)           \\ Result is [101, 102, 104]
    
    a := { name: "Tom", age:55 }
    a = delete(a, "age")         \\ Result is { name: "Tom" }

You cannot use the delete function with anything other than a struct
or array item.

### len(string)
Return the length of the argument. The meaning of length depends on the 
type of the argument. For a string, this returns the number of characters
in the string. For an int, float64, or bool value, it returns the number of
characters when the value is formatted for output.

Some examples:

| Example | Result |
|:-|:-|
| len("fortitude")   | 9, the number of characters in the string. |
| len(135)           | 3, the number of characters when 135 is converted to string "135" |
| len(false)         | 5, the number of characters in "false" |
| len(3.1415)        | 6, the number of characters in "3.1415" |
| len([5,3,1])       | 3, the number of elements in the array | 
| len({a:1, b:true}) | 2, the number of fields in the array |

### min(v1, v2...)
This gets the minimum (smallest numeric or alphabetic) value from the list.
If the first item is a string, then all values are converted to a string for
comparison and the result will be the lexigraphically first element. IF the
values are int or float64 values, then a numeric comparison is done and the
result is the numerically smallest value.
    
    min(33.5, 22.76, 9, 55)
    
This returns the float64 value 9.0


### max(v1, v2...)
This gets the maximum (largest numeric or alphabetic) value from the list.
If the first item is a string, then all values are converted to a string for
comparison and the result will be the lexigraphically lsat element. IF the
values are int or float64 values, then a numeric comparison is done and the
result is the numerically largest value.
    
    max("shoe", "mouse", "cake", "whistle")
    
This returns the string value "whistle".

### sum(v1, v2...)
This function returns the sum of the arguments. The meaning of sum depends
on the arguments. The values must not be arrays or structures.

For a numeric value (int or float64), the function returns the mathematical
sum of all the numeric values.

    x := sum(3.5, 15, .5)

This results in `x` having the value 19.  For a boolean value, this is the
same as a boolean "and" operation being performed on all values.

For a string, it concatenates all the string values together into a single
long string.

## IO Functions
These functions handle general input and output to files.

### io.Readdir(path)
This reads the contents of a directory, specified as a string path. The result
is an array of structures, one structure for each file. The information in the
structure contains the name, mode, size, modification date, and a flag indicating
if this entry is a directory or not.

### io.Readfile(name)
This reads the entire contents of the named file as a single large string,
and returns that string as the function result.

### io.Writefile(name, text)
This writes the string text to the output file, which is created if it
does not already exist. The text becomes the contents of the file; any
previous contents are lost.

### strings.Split(text [, delimiter])
This will split a single string (typically read using the `io.Readfile()`
function) into an array of strings on line boundaries.

    buffer := io.Readfile("test.txt")
    lines := strings.Split(buffer)

The result of this is that lines is an array of strings, each of which
represents a line of text. If you wish to split the data using a
different delimiter string than a newline, specify it as the optional
second parameter:

    urlParts := strings.Split("/services/debug/windows", "/")

results in `urlParts` being an array of three string values, containing
["service", "debug", "windows"] as its value.

### io.Open(name [, createFlag ]) 
This opens a new file of the given name. If the optional second parameter
is given it specifies the mode "create", "append", or "read". This determines
if the file is created/overwritten versus appended to for writes, or only
used for reads. The result is a file handle and an error value. If the error
is nil, the file handle can be used for additional operations documented
below.

     f := io.Open("file.txt", "create")

This creates a new file named "file.txt" in the current directory, and
returns the identifier for the file as the variable `f`. The variable `f`
is a readonly struct, which also contains a field `name` that contains
the fully-qualified file name of the file that was opened.

### f.ReadString()
This reads the next line of text from the input file and returns it as
a string value. The file identifier f must have previously been returned
by an `io.Open()` function call.

### f.WriteString(text)
This writes a line of text to the output file `f`. The line of text is
always followed by a newline character.

### f.Close()
This closes the file, and releases the resources for the file. After the
`Close()` call, the identifier cannot be used in a file function until it
is reset using a call to `io.Open()`.


## Utility Functions

These are miscellaneous funcctions to support writing programs in Ego.

### sort(v...)
This sorts list of values into ascending order. The elements can be scalar
values or an array; they are all concatenated into a single array for the 
purposes of sorting the data. The type of the first element in the
array determines the type used to sort all the data; the second and following
array elements are cast to the same type as the first element for the purposes
of sorting the data.

It is an error to call this function with an array that contains elements that
are arrays or structures. 

### util.UUID()
This generates a UUID (universal unique identifier) and returns it formatted
as a string value. Every call to this function will result in a new unique
value.

### members(st)

Returns an array of strings containing the names of each member of the 
structure passed as an argument. If the value passed is not a structure
it causes an error. Note that the resulting array elements can be used
to reference fields in a structure using array index notation.

    e := { name: "Dave", age: 33 }
    m := utils.members(e)

    e[m[1]] := 55

The `util.members()` function returns an array [ "age", "name" ]. These are
the fields of the structure, and they are always returned in alphabetical
order. The assignment statement uses the first array element ("age") to access
the value of e.age.

### util.Symbols()
Returns a string containing a formatted expression of the symbol table at
the moment the function is called, including all nested levels of scope.
The typical use is to simply print the string:

    x := 55
    {
        x = 42
        y := "test"
        fmt.Println( util.symbols() )
    }

This will print the symbols for the nested basic block as well as the
symbols for the main program.

## Formatted Output
You can print values to the console (or stdout device) by using calls
into the functions of the `fmt` package.

### fmt.Print(...)
The `Print` function prints any values passed as parameters to the
standard output, using the default formatting operations for Ego
values. There is no trailing newline after the output, so multiple
Print() calls will produce output on a single line of text.

### fmt.Println(...)
The `Println` function prints any values passed as parameters to the
standard output, using the default formatting operations. After any
values are printed, a newline is printed. This can also be used with
no parameters to produce a newline character after a series of `Print`
calls.

### fmt.Printf(format, ...)
The `Printf` function formats output and prints it. You must specify
at least one parameter which is a string containing the format 
specification to print. This can include substitution operators
for values in thie string, which correspond to the additional
parameters passed after the format string.

This uses the Go format processor, so it can support the exact
same format specifications as Go.

### fmt.Sprintf(format, ...)
The `Sprintf` function formats output and returns it as a string
value. You must specify
at least one parameter which is a string containing the format 
specification to output. This can include substitution operators
for values in thie string, which correspond to the additional
parameters passed after the format string.

This uses the Go format processor, so it can support the exact
same format specifications as Go.

## JSON Support
You can formalize the parsing of JSON strings into Ego variables using
the `json` package. This converts a string containing a JSON represetnation
into an Ego object, or converts an Ego object into the corresponding JSON
string.

### json.Marshal()
Returns a string containing the JSON representation of the arguments. If only
one argument is given, the result is the JSON for that specific argument. If
more than one argument is given, the result is always encoded as a JSON array
where each element matches a parameter to the call.

### json.MarshalIndented()
Returns a string containing the JSON representation of the arguments. If only
one argument is given, the result is the JSON for that specific argument. If
more than one argument is given, the result is always encoded as a JSON array
where each element matches a parameter to the call.

This function differs from `json.Marshal` in the the resulting string contains
newline and indentation spaces to make the string more human-readable.

### json.UnMarhsal()
This accepts a string that must contain a syntactically valid JSON expression,
which is then converted to the matching `Ego` data types. Supported types
are int, float64, bool, string, array, and struct elements.

## Profile
This collection allows an Ego program to interact with the persistent settings
profile data.

### profile.Keys()
This returns an array of strings, each of which is a key value in the active
profile. Each of these values can be used in a `profile.get()` call to read 
a value.

    keys := profile.Keys()
    for i := range keys {
        fmt.Printf("profile key is %s\n", i)
    }

### profile.Get()
Use this to read a single key value from the profile. The only parameter is
the name of the profile key.

     password := profile.Get("secret-password")

If the key is not valid, then an empty string is returned as the value.

### profile.Set()
Use this to store a value in the profile. The first parameter is the name of
the profile key, and the second parameter is coerced to be a string value that
is stored in the profile. If the key does not already exist, it is created.

    profile.Set("secret-password", "gronzeldabelle")

Note this can only be invoked as a `call` statement; it does not return a value.

### profile.Delete()
This will delete a key from the current profile. If you try to delete a key
that does not exist, there is no action taken.

    profile.Delete("secret-password")

This must be invoked as a `call` statement; there is no function result value.
This delete operation cannot be undone; the key value is immediately removed
from the profile and the profile is rewritten to disk.

## Strings
This collection of functions support string operations.

### strings.String(v...)

The function accepts one or more arguments, which are processed and concatenated
into a single string value.

|Type|Example|Description|
|--|--|--|
| int | 65 | Character with matching unicode value |
| string | "test" | String value |
| []string | ["a", "b", "c"] | Array of string values |
| []int ] [ 65, 66, 67 ] | Array of runes expressed as integer unicode value |

### strings.Ints(string)
Given a string value, returns an array containing all the runes of the string
expressed as integers containing the unicode values. If this array was passed
to strings.string() it would return the original string.

### strings.Chars(string)
Given a string value, return an array containing all the characters of the
string as individual string array elements. So "test" becomes ["t", "e", "s", "t"]

## time
The `time` package supports basic time calculations.

### time.Now()
This returns the current time as a formatted string. Time values are generally always
expressed as strings.

### time.Add(t, d)
Adds a given duration ("1s", "-5h3m", etc) to the time and return a new time string.

### time.Subtract(t1, t2)
Subtract t2 from t1 and return the duration ("-5s", etc). The sign will be "-" if t2
is greater than t1.
