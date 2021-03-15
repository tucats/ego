# ego
The `ego` command-line tool is an implementation of the _Ego_ language, which is an
interpreted (scripting) language similar to _Go_. Think of this as _Emulated Go_. The
command can either run a program interactive, start a REST server that uses _Ego_
programs as service endpoints, and other operations.

This command accepts either an input file
(via the `run` command followed by a file name) or an interactive set of commands 
typed in from the console (via the `run` command with no file name given ). You can
use the `help` command to get a full display of the options available.

Example:

    $ ego run
    ego> print 3*5
    
This prints the value 15. You can enter virtually any program statement that will fit on
one line using the interactive command mode. To finish entering _Ego_ statements, use
the command `exit`. You can also pipe a program directly to _Ego_, as in

   echo print 3+5 | ego
   8


If a statement is more complex, or you wish to run a complete program, it may be easier 
to create a text file with the code, and then compile and run the file. After the input
is read from the file and run, the `ego` program exits.

Example:

     ego run test1.ego
     15

&nbsp; 
&nbsp;
* Details on the _Ego_ language can be found in the [Language Reference](LANGUAGE.MD). 
* Details on using _Ego_ as a web server are in [Ego Web Server](SERVER.MD)

&nbsp; 
&nbsp;

## Building

You can build the program with a simple `go build` when in the `ego` root source directory.
This will create a build version number of 0 in the compiled program. To adopt the current
build number (stored in the text file buildvers.txt), use the `build` shell script for 
Mac or Linux development.

If you wish to increment the build number (the third integer in the version number string),
you can use the shell script `build -i`. The `-i` flag indicates that the plan is to increment
the build number; this should be done _after_ completing a series of related changes. You must
have already committed all changes in the working directory before you can use the `-i` flag.
This will increment the build number by one, rebuild the program to inject the new build number,
and generate a commit with the commit message "increment build number".

&nbsp; 
&nbsp;
## Preferences
`Ego` allows the preferences that control the behavior of the program 
to be set from within the language (using the `profile` package) or using the Ego command
line `profile` subcommand. These preferences can be used to control the behavior of the Ego c
ommand-line interface, and are also used by the other subcommands that run unit tests, the 
REST server, etc.

The preferences are stored in ~/.org.fernwood/ego.json which is a JSON file that contains
all the active profiles and their defaults. You can use the `ego profile` command to view
the list of available profiles, the current contents of the profiles, and to set or
delete profile items in the active profile.

Here are some common profile settings you might want to set.
### ego.compiler.extensions
This defaults to `false`. When set to `true`, it allows extensions to the language to be
used in programs. Examples include the `print` statement and the `exit` statement.

### ego.compiler.import
This defaults to `false`. If set to `true`, it directs the Ego command line to automatically
import all the builtin packages so they are available for use without having to specify an
explicit `import` statement. Note this only imports the packages that have builtin functions,
so user-created packages will still need to be explicitly imported.

## ego.compiler.normalized
This defaults to `false`. When set to `true`, symbol names (variables, packages, functions)
are not case-sensitive. When set to `true`, calling `fmt.Println()` is the same as `fmt.printLN()`.

## ego.compiler.types
This defaults to `dynamic` which means that a variable can take on different types during the
execution of a program. When set to `static`, it means that once a variable is declared within
a given scope, it can never contain a variable of a different type (that is, if declared as a
string, it can not be set to an int value).

### ego.console.exit.on.blank
Normally the Ego command line interface will continue to prompt for input from the console
when run interactively. Blank lines are ignored in this case, and you must use the `exit`
command to terminate command-line input.

If this preference is set to `true` then a blank line causes the interactive input to end,
as if an exit command was specified.

### ego.console.readline
This defaults to `true`, which uses the full readline package for console input.This supports
command line recall and command line editing. If this value is set to `false` or `off` then 
the readline processor is not used, and input is read directly from stdin. This is intended 
to be used if the terminal/console window is not compatible with readline.
