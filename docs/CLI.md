# Command Line Interface Processing

When an _Ego_ command line is processed, the shell breaks the arguments into an array of string
values, according to the native shell rules for quoting, etc. This array is then used by the
_Ego_ command line processor to validate the command line, store the values where they can be
accessed by the running code, and invoke the function that will execute the given command.

## Grammar Types

_Ego_ supports two different grammars for command line processing.

The default form is "class" based, in which the first subcommand or verb on the command line after `ego`
is the class of operation, and the second and subsequent verbs are subcommands for that class.

```sh
$ ego server start
Server started as PID 3351
```

In this example, `server` is the class of command, and `start` is the subcommand for the `server` class.
There are many subcommands for that class.

The alternate (non-default) grammar form is "verb" based, in which the subcommands following `ego` are
a more natural-language format with verbs and objects. The above example would then be:

```sh
$ ego start server
Server started as PID 3351
```

In this case, the first token after `ego` is always a verb, followed by one or more tokens describing
the object the verb is to act on.

In both cases, the executed code is identical. There are "class" and "verb" forms of every command. The
grammar style must be selected by creating an environment variable `EGO_GRAMMAR` which can have a value
of either "verb" or "class". This value can be set either in the environment before the `ego` command is
run, or specified in the "env.json" file in the default configuration directory.

The "env.json" file is a simple JSON file located in the ".ego" subdirectory of the current user's home
directory. This JSON file contains an object describing the environment variables and their values to
apply before running the _Ego_ command. For example,

```json
{
    "EGO_GRAMMAR": "verb",
    "EGO_COMPILER_EXTENSIONS": "true"
}
```

This file creates two environment variables before the _Ego_ command processor starts. The first defines
the grammar to use (the "verb" grammar, in this case) as well as setting a default configuration option
to set `ego.compiler.extensions` to `true`.

Note that the help output generated whenever an `ego` command ends in `--help` or `-h` is formatted
based on the current grammar configuration. That is, the help output shows the command syntax based
on the current grammar setting.

In the ./grammar subdirectory of the project you will find all the grammar definitions. The traditional
default "class" grammar is entirely contained in the file "traditional.go". The alternative "verb"
grammar is expressed in the other .go files in the directory, and the root of this grammar is in "verbs.go".

## Error Handling

While not strictly an error, any command can be typed in followed by `-h` or `--help` and, instead
of executing the command, the help processor shows the command you've typed and all it's options
with descriptions. You can use this to determine if you have a correctly formed command, or to
describe any additional options or parameters.

If a command is entered that is incomplete (that is, it is missing a subcommand), then the error
output will show the possible expected next terms in the command. Additionally, if the configuration
option "ego.console.auto.help" is set to `true`, then any incorrectly-formed command also
automatically shows the help information for the valid part of the command entered.

If a parameter or option value is required, the CLI processor will prompt for that value on the
console, and insert it into the correct place on the command line.
