# Errors in Ego

This document describes information a developer needs to know about Ego's
internal error type.

## Using Ego error types

This section covers how Ego code (written in Go) uses the Ego error type.
This assumes your code has imported "github.com/tucats/ego/error" as a
package in your program, such that references to the `errors` package
refer to the Ego version, not the standard version.

### Returning a defined error

By convention, all defined errors are declared in errors/messages.go and
are automatically initialized during Ego startup. The error messages use
a short string key as the error "value". This key is used to look up the
actual text of the error message in the localization database, such that
the formatted error is presented in the localized language if it is
available, else falls back to English if no localized version is found.

_You should never use fmt.Error() in the production code to format
a message, as those are not inherently localizable using the native Ego
localization system._

Because error messages are defined in terms of a language-neutral key
value, it is possible to compare error messages more directly and still
maintain localized text output.

Here is an example error value return:

```go
   return errors.ErrUnknownLabel.Context(labelName)
```

This uses the predefined error `ErrUnknownLabel` which, in English, has the
text format value of "unknown label". The `Context()` function adds additional
information to the error -- in this case, the name of the label string that
was not known. A `Context()` is always optional in an error message, it is
intended to provide _additional_ information to an error that is otherwise
complete.

When the `Context()` modifier is used, the message formatting adds a ":" and
the context value to the message.

```text
Error: unknown label
```

becomes

```text
Error: unknown label: fuzzy
```

There are a number of modifiers that can be added to any message that are used to
provide additional information. These are all optional, but are often used by the
compiler or runtime to give the user more information about where an error was
encountered.

| Modifier | Description |
| -------- | ----------- |
| Clone(err) | Chain a secondary message to this message - documented below |
| In("where") | Adds a phrase showing what function or source file the error occurred in |
| At(line,col) | Adds information about the line number and column position where error occurred |

Note that in the Ego implementation itself, both the runtime and compiler systems have functions
to automatically handle adding `In()` and `At()` information to the message based on current state
information in the compiler handle or the bytecode context.

### Formatting messages

To convert a message to human-readable text format, just use the standard `Error()` function.
This function will return a string that contains one or more lines of text with the error and
any of it's chained or contextual information adorning the message.

### Wrapping messages

The Go standard library uses `fmt.Errorf()` and the "%w" format operator
to create nested or wrapped messages. The Ego error does not use format
operators in this way, so there is a different mechanism used to create
wrapped or nested message -- the `Chain()` function.

If this is the Go code you would write using the standard library:

```go
err := fmt.Error("Failed to fetch user: %w", userErr)
```

This is implemented with the `Chain()` operation in Ego. An existing
error can accept a `Chain()` call which links a nested error to the
existing error message, as in:

```go
err := errors.ErrFetchUserError.Chain(errors.New(userErr))
```

There are several things to note here:

- All Ego errors are defined by a pre-declared variable value, defined in
  errors/messages.go file. This value contains information used to display
  a localized string value of the error text when formatted using `Error()`.
- The above example assumes that userErr was `error` variable that was
  possibly returned from the standard library or an external package. The
  use of `errors.New()` takes the opaque error value and rewraps it as
  an Ego error value. If `userErr` was already an Ego error, the extra
  use of `New()` does not wrap it again. It is always safe to wrap an
  error.
- The `Chain()` function links the error it has as an argument as a nested
  error, very similar to how the "%w" operator works. When the code
  attempts to format the value of `err` using the `Error()` function,
  both messages are printed out, but as a multi-line message. Each
  error should assume that it _may_ be formatted in isolation, but
  the use of `Chain()` means that the chain of messages will be
  formatted as a single string.

So the formatted version of this message might look like:

```text
Error: unable to fetch user data,
       file not found: dummy.json
```

Note that the comma(s) are added to the formatted message by Error() when
a chained message is found, the "," is not part of the message text itself.
The second line is the value of the `userErr` variable, possibly returned
from the standard library.

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
usually 80 characters long or shorter. Note that when formatted, the error
message may include additional information (such as a context value, line
number, etc) that are automatically added before or after the localized
string. As such, the localized string should not include terminating
punctuation, and any message may be formatted with a colon ":" character
and more information.

As a rule, substitution operators (like `{{count}}`) are not used in
error messages, any supplemental information is in a context value added
to the Ego error object and formatted after the ":" in the output.

### Build-time warnings

Any new localized string must be added to all messages_*.txt files in the
i18n/languages directory. If a key is present in one language but not in
all languages, a warning is generated when tools/build is run. Additionally,
if a messages is longer than 100 characters (not including substitutions,
if present), a warning is generated.
