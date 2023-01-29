# functions

The `builtins` package contains the builtin functions (those native to the Ego
language, such as len() or append()). The following provides a short summary of
each of the built-in functions, broken into categories based on the general data types
or functionality used.

## Type Casting Functions

These functions are used to explicity specify the type of a value to be used in the
evaluation of the expression. They take an arbitrary value and return that value
coerced to a function-specific type. In general, you can use the name of any scalar
type (not a map or structure) as a cast function name.

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
contain the strings defs.True or defs.False to be converted without error.

    bool(defs.True)

This returns the value true.

### float64(any)

Return the argument coerced to an float64 data type. For a boolean, this
will result in 0.0 or 1.0 values. For an, it returns the floating point
equivalent of the integer value.
A string must contain a valid representation of an floating point value to convert
without error.

    float64("3.1415")

This returns the float64 value 3.1415.

## Builtin functions

These functions are native to the language and do not need to be declared
to be used.

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
| len(false)         | 5, the number of characters in defs.False |
| len(3.1415)        | 6, the number of characters in "3.1415" |
| len([5,3,1])       | 3, the number of elements in the array |
| len({a:1, b:true}) | 2, the number of fields in the array |

### make( type, count)

The `make` function can be used to construct an array of the given type,
with `count` members containing the zero-value for the given type.

For example,

     b := make([]byte, 10)

This sets `b` to be an array of bytes, with 10 elements allocated. The
elements are all `false`, which is the zero-value for the `bool` type.
