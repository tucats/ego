# Errors in Ego

This document describes information a developer needs to know about Ego's
internal error type.

## Add a new error

Adding a new error involves several steps:

- Adding a new error to errors/messages.go
- Adding localized versions of the error text to the i18n/languages files

### Adding a new error value

Each runtime error is declared by a value declared in the errors/messages.go
file. Error names are always preceded by the string "Err", following by a
succinct camel-case representation of the error condition in English. For
example, an error that reports a division by zero error would be named
something like `ErrDivByZero`.

Each declaration is a Go `var` statement of the error identifier being set
to a value from a call to the `Message()` function. This call creates an
error object whose internal value is the string passed to `Message`. For
example,

```go
var ErrDivByZero = Message("div.by.zero")
```

The string is a key value for localization lookup. This allows the message
to have a constant value, which is then localized when the message is
printed or formatted for output (via `Error()`, etc.)

The key value should be similar if not identical to the camel-cased words
used to define the error. This helps guarantee each error code and it's
localization key are unique and won't collide with another error.

The `var` declarations are stored in alphabetical order after the first
group declared in the file which are special purpose signals not meant to
be formatted as error messages.

### Adding localized message strings

The localized values are stored in the files in i18n/languages/ in the
files like messages_en.txt for English, messages_es.txt for Spanish, etc.
The file uses the standard two-character localization identifier for a
language.

This file is organized into sections based on usage (messages, labels,
log messages, etc.). The errors are always added to the `[error]` section,
which contains that string before the messages. The `[error]` heading also
means that all messages that follow (until the next heading section) will
automatically have the prefix "error" attached to the key values. Note that
this string is not part of the `Message()` string when the error is declared,
but is used to ensure that a log message and an error message with the same
key string will be different localized values.

For the example of ErrDivByZero with a key string of "div.by.zero", the
line in the localization file might be:

```text
div.by.zero=Division by zero
```

The text following the `=` is the localized string, so the above example
is for the English version. The key values are stored in the `[error]`
section in alphabetical order.

Messages should not be capitalized. The message should also be succinct,
usually 80 characters long or shorter. Note that the error message may
include additional information (such as a context value, line number, etc)
that are automatically added before or after the localized string. As such,
the localized string should not include terminating punctuation, and any
message may be formatted with a colon ":" character and more information.

As a rule, substitution operators (like `{{count}}`) are not used in
error messages, any supplemental information is in a context value added
to the Ego error object and formatted after the ":" in the output.
