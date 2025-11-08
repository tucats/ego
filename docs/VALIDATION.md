# REST Payload Validation

When a REST call is made to an Ego server with a payload for a POST, PATCH, or PUT method,
the server will attempt to validate the payload JSON contents before calling the endpoint
handler. This allows the handler to avoid doing extra or unnecessary validation as the
handler can be assured that the JSON payload was valid and was able to be correctly
unmarshalled into the internal payload structure.

This is done by affixing tags to the structure definitions for the payload objects. These
tags are used by the validation system to check the required attributes for a given
payload type. These tags are read during initialization of the server validation, which
assigns each structure type a validation name (beginning with an "@" character). Additionally,
the initialization also adds alias for each endpoint name to the correct type that is to be
processed for that endpoint and method.

## Valid Tags

The tags for the structure appear in the back-tick string following each field definition,
just like "json" tag values are stored. In fact, often a tag will contain both a "json" and
a "valid" specification.  For example,

```go

type DSNPermissionItem struct {
   DSN     string   `json:"dsn"     validate:"required"`
   User    string   `json:"user"    validate:"required"`
   Actions []string `json:"actions" validate:"required,minsize=1,enum=read|write|admin|+read|+write|+admin|-read|-write|-admin"`
}
```

In this structure definition for a DSN permission object, the tag includes the JSON information
(at a minimum, the name of the field to use when expressing the JSON equivalent). Additionally,
a validation entry is in the same tag string (enclosed by the back-tick characters). In the first
two fields, the validation information specifies that this element is _required_ in the payload.
If it wasn't required, then a payload doesn't have to contain that field and if it is missing the
null or zero value is assigned to the field structure.

The third field (the array of strings) contains additional validation information. It indicates
that if present, the array must contain at least one element ("minsize") and each string is
required to be one of the enumerated values. This allows for keyword checking of parameters. The
valid values are a list separated by "|" characters, so a valid string could include "-read" or
"+admin" as values.

Here are the validation specifications that can be included in the "valid" tag. In the table below,
if there is a _value_ for the keyword, it must follow the keyword and an equals sign. If there is
no _value_ then it is true if present or false if not present in the validation string.

| keyword | value | description |
|--|--|--|
| case     |         | The enumerated values are case-insensitive |
| enum     | list    | The list of allowed string or numeric values for this item |
| max      | any     | The maximum value for this field (string or number) |
| maxlen   | integer | The maximum length of this string value |
| maxsize  | integer | The maximum size of this array value |
| min      | any     | The minimum value for this field (string or number) |
| minlen   | integer | The minimum length of this string value |
| minsize  | integer | The minimum size of this array value |
| required |         | This field _must_ be present in the JSON payload |
| type     | string  | Override the type specification for this field |

Note the use of `type` as a validation entity is useful for extended types. For example, a duration
value may be passed as a string, but if the validation includes `type=_duration` then the string
value is parsed to confirm it contains a valid time duration.  Extended types include `_duration`
and `_uuid` in addition to the default types based on the native Go structure values.

## Endpoint Matching

By convention, structures are all defined using types that start with "@" characters. These
types can be attached to specific endpoints by creating an alias for the endpoint that
defines the type to use.

By default, the server rest handler uses the endpoint definition to form a default name.
This is done by removing leading and trailing "/" characters, converting internal "/"
characters to ".", removing URL parameter markers "{{" and "}}", and appending the
method used to operate on this type.

| endpoint              | method | definition name        |
|:----------------------|:-------|:-----------------------|
| /admin/users/{{name}} |  PATCH | admin.users.name:patch |
| /admin/users          |  POST  | admin.users:post       |
| /dsns                 |  POST  | dsns:post              |

To match up the definition types created above (the "@" names) to the definition name that
the REST server will look up, an alias object is created. During server initialization, the
names that are defined based on internal data structures using the "valid" tag are defined
as aliases.

For example, here is a sample of the server initialization table used to initialize the
validation entries:

```go
var validationDefinitions = map[string]any{
    "@user":                  defs.User{},
    "@credentials":           defs.Credentials{},
    "@dsn":                   defs.DSN{},
    "@dsn.permission":        defs.DSNPermissionItem{},
    "admin.users:post":       "@user",
    "admin.users.name:patch": "@user",
    "dsns:post":              "@dsn",
    "dsns.@permissions:post": "@dsn.permission",
}
```

In the above examples, "@user" is created as a type for the data structure `defs.User{}`, which
is defined in the defs/representations.go file.  This type is used for multiple endpoints, so
aliases are created for them. The a POST to `/admin/users` is validated by converting the endpoint
and method to the name "admin.users:post" and this is mapped to the "@user" type previously
defined. This allows multiple endpoints to use the same payload definition; in this case a PATCH
to `/admin/users/{{name}}` resolves to "admin.users.name:patch" which also uses the "@user" payload
definition.

## JSON Validations

When the server initializes, it recursively searches the lib/validations directory for files that
end in ".json" to read them to determine if they contain additional definitions. These files
contain JSON definition objects that define additional types not necessarily defined by internal
Go structures. For example, the use can create definitions for payloads associated with a
rest service written as an Ego program, and get the automatic payload validation as well.
See the lib/validations/loggers.json for an example of the definition of logging settings payloads.
