# Introduction to Ego

The `ego` command-line tool is an implementation of the _Ego_ language, which is a
scripting language with syntax and functionality based on _Go_. Think of this as
_Emulated Go_. The command can either run a program interactively, start a REST
server that uses _Ego_ programs as service endpoints, and other operations.

This command accepts either an input file (via the `run` command followed by a file
name) or an interactive set of commands typed in from the console (via the `run`
command with no file name given ). You can use the `help` command to get a full
display of the options available.

Example:

```sh
    $ ego run
    ego> fmt.Println(3*5)
```

This prints the value 15. You can enter virtually any program statement using the
interactive command mode. If the line is incomplete due to mismatched quotes,
parentheses, or braces, then _Ego_ will prompt for additional lines before trying
to execute the statement(s) entered.

In this mode, _Ego_ maintains the state of all values and variables you create directly
from the command line, including functions you might define. This allows you to interactively
examine values, create functions, execute individual statements, and access packages from
the console in a single session. While not strictly a REPL, this behaves in a very similar
way to a REPL environment. Each time you enter a statement or command, it is compiled
immediately and executed, and can access all values and functions previously entered into
the console.

To finish entering _Ego_ statements, use the command `exit`. You can also pipe a program
directly to _Ego_, as in

```sg
    echo 'print 3+5' | ego
    8
```

Note that in this example, the _Ego_ language extension verb `print` is used in place
of the more formal `fmt.Println()` call. See the [Language Reference](#LANGUAGE.MD) for
more information on extensions to the standard Go syntax provided by _Ego_.

If a statement is more complex, or you wish to run a complete program, it may be easier
to create a text file containing the code, and then run the file (which reads the text
from disk and performs in internal compilation phase before running it). After the input
is read from the file and run, the `ego` program exits.

Example:

```sh
     ego run test1.ego
     15
```

&nbsp;
&nbsp;

* Details on the _Ego_ language can be found in the [Language Reference](LANGUAGE.md).
* Details on using _Ego_ as a web server are in [Ego Web Server](SERVER.md)
* Details on using _Ego_ as a command-line database are in [Ego Table Server Commands](TABLES.md)
* Details on connecting to _Ego_ as a REST-based server are in [Ego Server APIs](API.md)

&nbsp;
&nbsp;

## Building

You can build the program with a simple `go build` when in the `ego` root source directory.
This will create a build version number of "developer build" in the compiled program. To
adopt the current build number (stored in the text file buildvers.txt), use the `build` shell
script for Mac or Linux development, or the `build.ps1` PowerShell script for builds on
Windows.

If you wish to increment the build number (the third integer in the version number string),
you can use the shell script option `build -i`. The `-i` flag indicates that the build is
to increment the build number. By convention this should be done _after_ completing a series
of related changes.

&nbsp;
&nbsp;

## EGO_PATH

The _Ego_ runtime can be used entirely on its own with no additional files. However, for full
functionality, _Ego_ requires a directory of library items that support the operation of the
language, its use as a server, etc. If the library does not exist, it will be created in a
location known as the `Ego path`.

By default, the first time _Ego_ is run it will create the lib directory and store the minimum
number of required files in the directory as part of initialization. The location of the lib
directory becomes the EGO_PATH location. For example, you might want to create a directory to
contain the _Ego_ materials, using

```sh
    mkdir -p ~/ego
```

You can specify the path in one of three ways when running the `ego` command line tool.

If nothing else is specified, then the EGO_PATH is assumed to be the directory where
the _Ego_ command line program was first executed from.

If there is a profile preference called `ego.runtime.path` it contains the absolute
directory path of the _Ego path_. You can set this value using the command like:

```sh
ego config set ego.runtime.path=/home/tom/ego
```

This sets the _Ego path_ value to be `/home/tom/ego` each time the `ego`
command line is run.

You can set the path location in the `EGO_PATH` environment variable, which
is the path value; i.e.

```sh
export EGO_PATH=/home/tom/ego
ego
```

Typically, once you have decided where to place the _Ego_ directories, use the
`ego config` command to store this location in the persistent profile store so
it is available anytime the `ego` command is run. You can use the `ego config show`
command to display the current profile values.

&nbsp;
&nbsp;

## Logging

The _Ego_ command has a number of logging message classes that can be enabled to
produce diagnostic information to the stdout console. These are enabled with the
`--log` option immediately following the `ego` command and before any sub-command
is given. The option must be followed by one or more logger names, separated by
commas. For example,

```sh
    ego --log trace,symbols run myprogram.ego
```

This enables the TRACE and SYMBOLS loggers and runs the program "myprogram.ego".
The trace messages all have the same basic format, as shown by this sample line
from the trace logger:

```text
    [20210402123152] 8981  TRACE  : (65) vartypes.ego 154: DropToMarker  ...
                ^     ^      ^        ^        ^
                |     |      |        |        |
    timestamp --+     |      |        |        |
    sequence number --+      |        |        |
    logging class -----------+        |        |
    thread id ------------------------+        |
    Logging message ---------------------------+
```

In this example, the timestamp represents 2021-04-02 12:32:52. The sequence number
indicates how many logging messages have been output to the log file. The class
name is the logger than contributed the message. The thread ID uniquely identifies
the execution context of the Ego program function(s) that are running, whether on
the main thread or as a "go routine". This is followed by the text of the logging
message, which varies by logging class. In this case, it shows the program name,
the instruction program counter, the instruction executed, and this is followed
by information about the runtime stack (not shown here for brevity).

By default, no logging is enabled except for running in server mode, which
automatically enables SERVER logging.

| Logger   | Description |
|:---------|:------------|
| AUTH     | Shows authentication operations when _Ego_ used as a REST server         |
| BYTECODE | Shows disassembly of the pseudo-instructions that execute _Ego_ programs  |
| CLI      | Logs information about command line processing for the _Ego_ application |
| COMPILER | Logs actions taken by the compiler to import packages, read source, etc. |
| DB       | Logs information about active database connections.                      |
| REST     | Shows REST server operations when _Ego_ used as a REST server            |
| SERVER   | Logs information about the use of _Ego_ as a REST server.                |
| SYMBOLS  | Logs symbol table and symbol name operations                             |
| TABLES   | Shows detailed SQL operations for the _Ego_ REST /tables endpoint.       |
| TRACE    | Logs execution of the pseudo-instructions as they execute.               |
| USER     | Logs messages generated by @LOG directives in _Ego_ programs.            |

&nbsp;
&nbsp;

## Preferences

`Ego` allows the preferences that control the behavior of the program to be set from the command line, as well as within the language (using the `profile` package).
These preferences can be used to control the behavior of the Ego command-line interface, the language processor, and the REST server mode.

The preferences are stored in the ~/.ego/ directory. This contains a JSON file for each of
the active profiles. The JSON file contains the settings values for that profile. You can use the `ego` commands to view the list of available profiles, the contents of
the current profile, and to set or delete profile items in the active profile. Note that a few configuration items (those
containing server keys or logon tokens) are not stored in the main JSON file, but are
stored in separate encrypted files linked to the configuration file.)

Here are some common profile settings you might want to set. Additional preferences are
referenced in the relevant sections of the [Language](LANGUAGE.MD), [Server](SERVER.MD),
[Table](TABLES.MD), and [API](API.MD) guides.

### ego.compiler.extensions

This defaults to `false`. When set to `true`, it allows extensions to the language to be
used in programs. Examples include the `print` statement and the `exit` statement.

### ego.compiler.import

This defaults to `false`. If set to `true`, it directs the Ego command line to automatically
import all the builtin packages so they are available for use without having to specify an
explicit `import` statement. Note this only imports the packages that have builtin functions,
so user-created packages will still need to be explicitly imported.

### ego.compiler.normalized

This defaults to `false`, which means that names in _Ego_ are case-sensitive. By default,
a symbol `Tom` is not considered the same as `tom`. When set to `true`,
symbol names (variables, packages, functions) are not case-sensitive. For example, when set to
'true', referencing `fmt.Println` is the same as `fmt.printLN`.

### ego.compiler.types

This defaults to `dynamic` which means that a variable can take on different types during the
execution of a program. When set to `static`, it means that once a variable is declared within
a given scope, it can never contain a variable of a different type (that is, if declared as a
string, it can not be set to an int value).

### ego.console.readline

This defaults to `true`, which uses the full Unix readline package for console input.
This supports command line recall and command line editing operations. If this value
is set  to `false` or `off` then the readline processor is not used, and input is
read directly from Unix stdin. This is intended  to be used if the terminal/console
window is not compatible with the standard readline library.

### ego.runtime.path

This defaults to the same lactation where the `ego` program was first run from. This is
the location where `ego` looks for the lib and test directories, which contain package source
files, server definitions, and the unit test library used by the `ego test` command.

This location is the location returned by the `ego path` command. Once the value is set during
the first run of the program, it generally is not changed. However, if you wish to specify
that `ego` store it's lib file in a different location (such as /usr/local/libexec) you can
specify that location in the config file.
