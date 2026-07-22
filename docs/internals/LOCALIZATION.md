# Localized Messages

The localization system for Ego supports individual localizations for any message in any language
that can be identified by a two-letter code. Common examples are "en" for English, "fr" for French,
"es" for Spanish, etc.

This document describes how localization calls are made, how the build process constructs the
data structures that are used by the localization mechanism, and how localization files are
constructed. This also includes a discussion on the formatting and substitution operators
available in localized message text.

## Accessing Localized Text

With Ego, the i18n package is used to generate localized text. The text to be used is identified
by a string designation, typically a mnemonic based on the keywords in the message. Each message
is identified by a unique keyword string.

The following functions are available for accessing text. The primary difference between them
is the presumed prefix for the message string. Any message can be accessed by the T() function,
all other functions are helper functions with well-known prefixes.

Each function has a required keyword name for the message, and an optional map of substitution
values. The map uses string keys, and can have any value type for the map values. The key name
is used in the localized text to indicate where substitutions are written into the localized string.
The format of those substitution operations is described later.

| Function | Description |
| -------- | ----------- |
| text := i18n.T("keyword"[, map]) | Get the localized string identified by "keyword" |
| text := i18n.E("keyword") | Get the localized string starting with "error." and "keyword" |
| text := i18n.L("keyword") | Get the localized string starting with "label." and "keyword" |
| text := i18n.M("keyword") | Get the localized string starting with "msg." and "keyword" |

For each form, the localization system first searches for the keyword message in the current
language (typically set by the native operating system or environment variable lke "LANG" or
"EGO_LANG"). If the localization is not found in the current language, it looks for the
string in the English localization file. If not found in the English localization, the
"keyword" string itself is used as the message text.

## Localized language files

The localized text strings are stored in the `languages/` directory of the package. There is
one file for each language, of the form messages_xx.txt, where `xx` is the two-character
language encoding. For example, the English message text is stored in `languages/messages_en.txt`.
A language file can be incomplete; i.e. not support every localization item, since a lookup for
a given language will check the English localization if the specific native language entry is
not yet translated/supported.

The files can contain comments, indicated by a `#` character at the start of a line. Lines with
comments and blank lines are ignored while processing. The default format for a line in the
file is:

```text
error.loop.index=invalid loop index variable
```

In this example, `error.loop.index` is the keyword for the message, which is what is passed
to the `i18n.T()` function. The text after the equals sign is the translated text for the
current language for that message. In this case, an error message used to indicate that the
loop index variable is invalid.

Because this message starts with "error." which is the default prefix for all error messages,
instead of calling `i18n.T("error.loop.index")`, the caller could instead use
`i18n.E("loop.index")` to access the text. The `E()` method assumes the prefix for the provided
keyword string is "error.". This serves only as a convenience function, there is no functional
difference between using the two forms shown above.

To make the creation of the message files easier, groups of messages with the same prefix
can be written with a default prefix. For example,

```text
[error]
loop.index=invalid loop index variable
syntax=bad syntax
verb=unknown verb
```

In this section of the message file, three messages are defined. Each has the default prefix
of "error." because the `[error]` notation precedes them. When a prefix is given this way, all
messages following it have the same prefix until a new prefix is specified. The purpose is to
simplify the creation of the messages by removing unnecessary redundancy.

## Building Message Data

When the build script for Ego runs (or a `go build` command is issued), part of the build
process will be to generate the message data structure from all supplied message text files.
This is accomplished by the comment at the start of the `strings.go` file in the package:

```go
//go:generate go run ../tools/lang/ -c -p languages -s messages.go
```

The `go:generate` command tag directs the build system to execute the command defined by
the remainder of the comment text. In this case, it runs (building first if necessary) the
`tools/lang` program and passing it the additional parameters. This command reads all the
messages found in the given path (languages) and generates a new Go source file "messages.go".
This file is not part of the saved source repository, but is generated each time a build is
done and there have been changes to any of the files in the languages directory.

The resulting source file "messages.go" contains a map that is initialized at runtime with
all required localizations. This is the map used by the `i18n.T()` function (and it's helper
functions) to translate the keyword string into the appropriate localized text.

__As stated in the `messages.go` text file, the file should _never_ be directly edited by the user.__

## Message Substitutions

Some messages (like the errors in the examples above) do not need substitution values. That is,
the message text is complete as is. For most error messages, there is additional information
added to the message, but that information is encoded in the Ego error data type and is not
part of the message text.

However, many other messages such as logging messages from the server have additional data stored
in the message text. Consider a message whose job is to report the length of a buffer generated.
Here is an example entry in the messages_en.txt file:

```text
msg.data.length=Length of reply: {{length}}
```

In this example, there is a single substitution value for `length`. The substitution operator is
defined by text with double braces (`{{` and `}}`). When calling the localization function, an
additional parameter is supplied which is a map that defines the substitution values to be put
in the text. Consider the following snippet of Go code, which presumes the length value is stored
in a variable called `numBytes`:

```go
text := i18n.T("msg.data.length", map[string]any{
    "length": numBytes,
})
```

In this case, the number of bytes is passed into the map with a key value of "length". When the
text is being formatted by the `i18n.T()` function, when the `{{length}}` text is found in the
localization, it directs the formatter to look in the map and place the value assigned to "length"
in the message. If the value of `numBytes` is 357, the message text resulting would be

```text
Length of reply: 357
```

By default, whatever value is found in the map is formatted using the default String() formatter
or default output type for that data value. The substitution text can specify a different format
or other information to control how the information is written. For example, if the length must
always be a five-digit string with leading zeros, you can specify the format in the localization
text as follows:

```text
msg.data.length=Length of reply: {{length|%05d}}
```

In this case, additional formatting information is given in the substitution operator by putting
a "|" character followed by the additional formatting operation. There can be multiple operators
given, all separated by the "|" character. In this case, the item is considered a Go format
specification because it starts with the "%" character. The string `%05d` indicates that the value
is meant to be formatted as an integer value with five spaces and leading zeros. In this case the
resulting text would be:

```text
Length of reply: 00357
```

Below is a table of the formatting operators that can be specified. If multiple format operations
are given in a substitution operator, they are processed in order specified.

| Format Operator | Description |
| --------------- | ----------- |
| lines | The item is an array, make a separate line for each array element |
| list | The item is an array, output each item separated by "," |
| size n | If the substitution is longer than `n` characters, truncate with `...` ellipses |
| pad "a" | Use the value to write copies of the string "a" to the output |
| left n | Left justify the value in a field n characters wide |
| right n. | Right justify the value in a field n characters wide |
| center n. | Center justify the value in a field n characters wide |
| empty "text" | If the value is zero, an empty string, or an empty array, output "text" instead |
| nonempty "Text" | If the value is non-zero, non-empty string, or non-empty array, output "Text" instead |
| zero "text" | If the value is numerically zero, output "text" instead of the value |
| one "text" | If the value is numerically one, output "text" instead of the value |
| many "text" | If the value is numerically greater than one, output "text" instead of the value |
| card "a","b" | If the value is numerically one, output "a" else output "b" |

These can be combined as needed, and a single value from the map of values can be used multiple
times in substitution operators. Consider the following message:

```text
row.count=There are {{count}} rows
```

In many languages (English included) both the verb and the noun are affected by the cardinality of
the value of count. Additionally, we might want to specify "no rows" when the count is zero. This
can all be done in the localization substitution defines. If the message was defined as:

```text
row.count=There {{count|card is,are}} {{count|empty "no"}} {{count||card row,rows}}.
```

This uses the count value to control the verb "to be", whether a numeric value or "no" for an empty
value, and the cardinality of the row noun. Note that in the example the `card` format operator
replaces the value with the string, while the `empty` format operator formats the value of `count`
normally using the default integer output, unless the value is empty/zero in which case the string
"no" will be used instead. That is, some operators affect the formatting of the value and other
operators use the value to made decisions about what to output instead of the value itself.

```text
There are no rows.   # For count of zero
There is 1 row.      # For count of 1
There are 32 rows.   # For count of 32
```
