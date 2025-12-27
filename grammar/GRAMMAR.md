# CLI Grammar Processing

When an _Ego_ command line is processed, the shell breaks the arguments into an array of string
values, according to the native shell rules for quoting, etc. This array is then used by the
_Ego_ command line processor to validate the command line, store the values where they can be
accessed by the running code, and invoke the function that will execute the given command.

Grammars are defined as arrays of an Option structure. You can see this in the various grammar
definition files. Here is a simple example of a grammar specification for the `format` command:

```go

var FormatVerbGrammar = []cli.Option{
    {
        LongName:      "log",
        Description:   "ego.verb.format.log",
        OptionType:    cli.Subcommand,
        Action:        commands.FormatLog,
        DefaultVerb:   true,
        Value:         FormatLogGrammar,
        ExpectedParms: -1,
        ParmDesc:      "opt.log.file",
    },
}
```

This particular node specifies the tokens that are allowed to follow the `format` command. This
specifies a name for the next token, a description for use with `--help` output, the type of
keywords this is (invokes a function, transfers to another grammar, or stores a value), and additional
information about the keyword "log" and how it is used.

## Grammar Fields

This section describes each possible variable in an `Option` and what they mean. If a field is not
present, it has a default value of zero/empty/missing and is not processed. See the definition in
github.com/tucats/ego/app-cli/cli/options.go for more details about the specific Go definition of
the `Option` object type.

## LongName

This is the default name for the option. For options that are keywords, there may be aliases. For
options that are meant to have values (like a "-" option), the long name is specified with "--" and
there can be an optional short name specified with a single "-". When displaying the help for a
command, the LongName is always displayed.

This is a required field; if this value is blank a panic occurs during system startup.

## ShortName

For names that are meant to be command line options, if a non-empty ShortName is given, this must
be a single character name used for the "-" form of an option. For example, the global option
`--quiet` indicates that superfluous confirmation messages are suppressed. The long name of this
option is "quiet" and the short name is "q". This means you can specify `--quiet` or `-q` and the
will mean the same thing. You cannot specify a short name for an subcommand or subgrammar.

## Description

This is a string that describes this command or option in a `--help` output display. By default,
the help processor will search the localization string database for the description string given
with an "opt." prefix. If not found, the description string is displayed without localization.
During development, it is acceptable to use a literal string, but before committing code the
value should be specified as a localization key and the associated "opt." localization key
added to the messages localization file for each supported language.

## PArmDesc

If the grammar item is subcommand that accepts one or more parameters, this is the text expression
used to display the parameter information in the `--help` output. By convention, this is a short
expression using hyphens to separate keywords. For example, a command that is followed by a
parameter that contains a DSN name might have a parameter description of "dns-name". When a
parameter is optional, it should be enclosed in square braces, such as "[dsn-name]". When there
can be multiple parameters, they should be specified using a trailing ellipsis "..." string.

## EnvVar

When the option can have a value injected via an environment variable, this is the name of
the variable. When command line processing is done, any value that was not specified on the
command line but that has an EnvVar set results in a query of that environment variable. If
the value is non-empty, it is stored as if the value had be typed as part of the command
line.

By convention, all environment variables are uppercase and start with "EGO_".

## Aliases

This is an array of alternate acceptable spellings of a keyword. For example, the grammar
might specify a LongName of "dsns" (which stands for data source names). But the aliases
array might indicate that "dsn" and "data-source-names" are acceptable alternate spellings.
The `--help` output will never show the aliases, they are "syntactic sugar" to help express
command lines only.

## Keywords

This is an array of string values that is only used when the option type is "string" or
"stringlist". IF there is a keywords array, then the string (or any string value in the
list) must match one of the keywords in the list.

## Unsupported

This is an array of strings indicating platform names that match the Go `GOOS` values,
such as "windows", "linux", or "macos"). When this array is present, then the command
specified in this option is _unsupported_ when running on the named platform(s). For
example, currently the "start server" operation is not supported on Windows, so that
command option has the value "windows" in the Unsupported array. For most options, which
are supported everywhere, this array is empty.

## Prompts

This is an array of string containing prompt strings for missing required parameters.
If this option is followed by one or more parameters that are required and they are
not specified on the command line, these prompts are used to ask the user to enter
the missing information. Like the parameter description field, these values are
first checked in the localization database, with the prefix "label." to see if they
are localized string values. If not, the array value is displayed as-is.

## Excludes

This is an array of strings that represent LongName values for other options that
**cannot** be specified if this option is specified. That is, if one or more options
are mutually exclusive, they should specify the other options that cannot also be
given on the same command line here. IF an option is specified that is on the
exclude list, an error is reported to the user explaining the invalid usage.

## Value

For any option that accepts a value, it's value is stored here by command line
processing. The type of the option controls the Go native type stored in Value.
Functions that are invoked by the CLI can query for the value of the named
item during execution.

If the current Option is a token that transfers control to a subgrammar, the
subgrammar is defined as the Value of the option.

## Action

For an option that represents the last verb in a command string, this indicates the
function or subgrammar to transfer control to. When the Action is a function it is
called directly with a `*cli.Context` variable that lets it query for the values
and parameters specified.

## OptionType

This is an int value that indicates what type of option this item represents. This
can be a subcommand that invokes a function or a subgrammar, or it can be an
option with a value, in which case the OptionType tells the CLI parser hwo to
validate the option value given by the user.

The option types supported are:

| Type | Description |
| ---- | ----------- |
| StringType | A string value |
| IntType | An integer value. Radix specifications are permitted |
| BooleanType | Takes no value, is "true" if present else "false |
| BooleanValueType | Takes a value which must be "true" or "false" |
| Subcommand | This is a subcommand, Value contains the next grammar to continue with |
| StringListType | The value can be zero or more comma-separated strings, Value is an array of strings |
| ParameterType | This item is a parameter on the command line |
| UUIDType | The value must be a valid UUID value |
| Keywords | The value is a string value that must be from the Keywords list |
| RangeType | The value is a numeric range with "-" or ":" separator; Value is an array of two integers |

## ExpectedParms

This indicates the number of expected parameters that can follow this item on
the command line. If zero, no parameters are allowed. -1 indicates a variable
number of parameters are allowed.

## MinParams

Integer value indicating the minimum number of parameters that must be specified.
This is only used when the ExpectedParams is -1 for variable parameters.

## Found

This is a boolean value that indicates that this item was found on the command line.
IF false, the item was not found on the command line and no matching environment variable
value was available.

## Required

IF true, this option is required and must be specified by the user. If the item is
required and not given, then user will be prompted for the value at the console.

## Private

If true, this item will not be displayed to the user in a `--help` output. This
allows defining experimental options or command verbs that are not visible to
the user until this flag is turned off at the end of development of the functionality.

## DefaultVerb

If true and this option is of type Subcommand, then if the command line has no
subcommand given, then this option is assumed. For example, this allows the
`ego format log` command to assume that `log` is the default term if it is not
given on the command line.
