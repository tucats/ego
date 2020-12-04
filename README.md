# ego
Implementation of the _Ego_ language. This command accepts either an input file
(via the `run` command followed by a file name) or an interactive set of commands 
typed in from the console
(via the `run` command with no file name given ). You can use the `help` command to get a full
display of the options available.

Example:

    $ ego run
    Enter expressions to evaulate. End with a blank line.
    ego> print 3*5
    
This prints the value 15. You can enter virtually any program statement that will fit on
one line using the `interactive` command. If a statement is more complex, it may be easier
to create a text file with the code, and then compile and run the file:

Example:

     ego run test1.ego
     15


## Building

You can build the program with a simple `go build` when in the `ego` root source directory.

If you wish to increment the build number (the third integer in the version number string),
you can use the shell script `build` supplied with the repository. This depends on the 
existence of the file buildver.txt which contains the last integer value used.

## Preferences
`Ego` uses the standard `persistence` package in `gopackages`. This allows the preferences
to be set from within the language (using the `profile` package) or using the Ego command
line `profile` subcommand.

These prefernces can be used to control the behavior of the Ego command-line interface.

### auto-import
This defaults to `false`. If set to `true`, it directs the Ego command line to automatically
import all the builtin packages so they are available for use without having to specify an
explicit `import` statement. Note this only imports the packages that have builtin functions,
so user-created packages will still need to be explicitly imported.

### exit-on-blank
Normally the Ego command line interface will continue to prompt for input from the console
when run interactively. Blank lines are ignored in this case.

If this preference is set to `true` then a blank line causes the interactive input to end,
as if an exit command was specified.

### use-readline
This defaults to `true`, which uses the full readline package, supporting command line recall
and command line editing. If this value is set to `false` or "off" then the readline 
processor is not used, and input is read directly from stdin. This is intended to be used
if the terminal/console window is not compatible with readline.

## Ego-specific Functions
The Ego program adds some additional functions to the standard suite in `gopackages`. These
are:

### prompt("string")
Prompt the user for input. If an argument is given, it is the prompt string written to the
console before reading the data. The input is returned as a string value.

If the prompt begins with a tilde `~` character, it means the input to the prompt is not
echoed to the user. The tilde is not printed as part of the prompt. This is used to allow
the user to enter a password, for example, without having it echoed on the console.

### eval("string")
The string is evaluated as an expression (using the `expressions` package in `gopackages`)
and the result is returned as the function value. This allows user input to contain complex
expressions or function calls that are dynamically compiled at runtime. 

