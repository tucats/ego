# Validations

This directory contains JSON expressions of validation specifications. These are
used to validate JSON rest payloads. The file contains a dictionary of objects, where
the keys are the the object keys and the object values are the validation specifications.

This is a brief summary of the validation format; see the package github.com/tucats/validator
for more information.

## Name

The name of the item expressed in the JSON is given. This name must match the spelling of
the corresponding item in the JSON exactly (case sensitive). If the item is not a field in
an object, the `name` field is blank.

## Type

Each item in the validation has a type. These are expressed as integer values, and include:

| Value | Meaning |
|-------|---------|
| 1 | string value |
| 2 | integer value |
| 4 | boolean value |
| 5 | JSON object value |
| 12 | string containing a comma-separate list of items |

## Enumerations

If the value is a string or a string list, the validation can specific the allowed values for
the string (or elements in the string list). These are expressed as an array of strings. If the
enumerations are case-sensitive, additionally the validation with set `matchcase` to true.

## Minimum and Maximum Values

If the value is an integer, the minimum and maximum values can be expressed in the validation.
The flag `has_min_value` must be true, and then the minimum value is given as `min_value`.
Similarly, `has_max_value` is true if there is a maximum value, given in `max_value`.

## Minimum and Maximum Length

If the value is a string, the minimum and maximum string length can be expressed. Set the
value `has_min_len` to true and then set `minlen` to the minimum length. Set the value
`has_max_len` to true and set `maxlen` to the maximum length.
