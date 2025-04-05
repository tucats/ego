# JSON Query Strings

This documents JSON query strings, which are used in the `--json-query` global option
to the Ego cli command. When this option is specified, it causes the output format to
be set to "JSON" and also specifies a query string as the option parameter. This string
is used to select elements of the JSON output to print, instead of printing the entire
item.

The JSON value can be an array, an object (with named fields) or a scalar value (integer,
floating point value, boolean, or string). Arrays and objects can have nested JSON values.
The query string format allows you to specify the named field, the array index (a zero-based
value or an `*` indicating all values), or a dot indicating the entire value.  Multiple dots
separate each part of the query.

## Single items

A single item (or the last item in a query) can be specified using the "." character. For
example, if the JSON query consists of a single value (a string in this example), then
the query result is that string.

```json
"This is a test"
````

The result of the query "." (or an empty string which means the same thing) will be the
string "This is a test". Similarly, if the JSON contains just an integer value,

```json
1337
```

The result of the query "." is the integer value `1337`.

## Named Field

If the value is an object, then the query string can be a specific named field in
that object, and the result is the value of that field. For example,

```json
{
    "name": "Fred"
}
```

A query value of "name" will return the string "Fred". Similarly, if the named
field is itself a complex item, the result will be the entire item. So for the
value:

```json
{
    "person": {
        "name": "Bob",
        "age": 53
    }
}
```

A query string of "person" will return the string `{"name": "Bob", "age": 53}`
as the result. Note that you can specify cascading values in the query string
when the value is itself and object, by specifying the field names separated
by a "." character. For the above example, the query "person.age" will return
the result `53`, since that is the value of the "age" field within the "person"
object.

## Arrays

When the object is an array, the query can specify either a specific numeric
array index, with the first item in the array having index 0 (zero). Alternatively
the array index can be an asterisk `*` character, which means _all array elements_.
For the JSON value

```json
[
    1,
    15,
    66
]

The query string "2" will return the value `66` since that is the value at the third
(0-based) array index. Similar to nested objects, you can reference additional information
about the array element using the "dot" notation. For example,

```json
{
    "items": [
        {
            "name": "Bob",
            "age": 56,
        },
        {
            "name": "Sue",
            "age": 52,
        }
    ]
}
```

To get the age value for the second item in the array, you can use a query
string of "items.1.age". This looks for the field "items" in the object, finds
the second (0-based) array element, and within that item, finds the field named
"age" to return teh integer `52`.

Note that you can specify _all_ the elements in the array in the query string
to get a list of items from the JSON object. For the above value, a query
string of "items.*.name" will return two string values,

```text
Bob
Sue
```

As the result. The items will be separated by a newline in the output of the
query expression.

## Errors

It is an error to specify a field name in an object that does not exist. It
is also an error to specify a field name when the value is not an object.

It is an error to specify a query string for an array index that does not exist
(a value larger than the length of the array) or for a value that is not an
array.

Similarly, it is an error to use array notation for a value that is not an array.
