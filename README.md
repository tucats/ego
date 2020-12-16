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

## Language

This is a summary of the _Ego_ language, adapted from the README.md file in the
github.com/tucats/gopackages/compiler suite. It describes the _Ego_ programming
language and the library of builints available.

The `ego` command line tool will compile text in the _Ego_ language into
bytecode that can be executed. This allows for
compiled scripts to be integrated into the application, and run repeatedly
without incurring the overhead of parsing and semantic analysis each time.

The _Ego_ language is loosely based on _Go_ but with some important
differences. Some important attributes of _Ego_ programs are:

* There are no pointer types, and no dynamic memory allocation.
* All objects are passed by value in function calls.
* Variables are untyped, but can be cast explicitly or will be type converted
automatically when possible.

The program stream executes at the topmost scope. You can define one or more
functions in that topmost scope, or execute commands directly. Each function
runs in its own scope; it can access variables from outer scopes but cannot
set them. Functions defined within another function only exist as long as
that function is running.

&nbsp;
&nbsp;

### Data types
_Ego_ support six data types, plus limited support for function pointers
as values.

| type | description |
|:-:|:-|
|int| 64-bit integer value |
|float| 64-bit floating point value |
|string| Unicode string |
|bool| Boolean value of true or false |
| [...] | Array of values |
| {...} | Structure of fields |

An array is a list of zero or more values. The array values can be of any
type, including other arrays. The array elements are always indexed starting
at a value of zero. You can also reference a range (slice) of an array by using
the notation `a[b:e]` which returns an array containing elements from `b` to
`e` from array `a`. You cannot reference a subscript that has not been allocated.
You can use the `array` statement to initialize an array, and the `array()`
function to change the size of an existing array.

You can also define an array constant, and assign it to a value, where
the initial array is defined by the constant.

    names := [ "Tim", "Sue", "Bob", "Robin" ]

This results in `names` being assigned an array containing the four string
values given.

Structures contain zero or more labeled fields. Each field label must be
unique, and can reference a value of any type. You can create a struct
using a literal:

    employee := { name: "Susan", rate: 23.50, active:true }

This creates a structure with three members (`name`, `rate`, and `active`).
Once this is stored in a variable, you can use dot-notation to reference
a field directly,

    pay := 40 * employee.rate

If the member does not exist, an error is generated. You can however add
new fields to a structure simply by naming them in an assignment, such as

    employee.weekly = pay

If there isn't already a field named `weekly` in the structure, it is
created automatically. The field is then set to the value of `pay`.

Note that structures and arrays are always passed around by reference.
That is, if you create a structure `a`, assign it to `b`, and then change
a field in `b`, it will also be changed in `a`. To make an exact duplicate
of an existing structure, use the `new()` function which creates a new
instance of the existing type. For example,

     a := { age: 55, name: "Timmy"}
     b := a
     b.age = 4
     print a.age    // Prints the value 4

     c := { age: 55, name: "Timmy"}
     d := new(c)
     d.age = 4
     print c.age    // Prints the value 55


### Scope
The console input (when `ego` is run with no arguments or parameters) or
the source file named when `ego run` is used creates the main symbol table,
available to any statement entered by the user or in the source file.

The first time a symbol is created, you must use the := notation to create
a variable as well as set its value. All subsequent sets of that variable
should use the = notation to store in an existing symbol.

Whenever a `{ }` block is used, a new symbol table is created for use during
that block. Any symbols created within the block are deleted when the block
exits. The block can of course reference symbols in outer blocks using 
standard "=" notation. For example,

    x := 55
    {
        y := 66
        x = 42
    }

After this code runs, the value of `x` will be 42 (because it was changed
within the block) and the symbol `y` will not be defined (because it went
out of scope at the end of the basic block.)


### array
The `array` statement is used to allocate an array. An array can also be
created as an array constant and stored in a variable. The array statement
identifies the name of the array and the size, and optionally an initial
value for each member of the array.

    array x[5]
    array y[2] = 10

The first example creates an array of 5 elements, but the elements are
`<nil>` which means they do not have a usable value yet. The array elements
must have a value stored in them before they can be used in an expression.
The second example assigns an initial value to each element of the array,
so the second statement is really identical to `y := [10,10]`.

### const
The `const` statement can define constant values in the current scope. These
values are always readonly values and you cannot use a constant name as a
variable name. You can specify a single constant or a group of them; to specify
more than one in a single statement enclose the list in parenthesis:

    const answer = 42

    const (
        first = "a"
        last = "z"
    )

This defines three constant values. Note that the value is set using an
`=` character since a symbols is not actually being created.

### if
The `if` statement provides conditional execution. The statement must start
with a expression which can be cast as a boolean value. That value is
tested; if it is true then the following statement (or statement block)
is execued. By convention, even if the conditional code is a single
statement, it is enclosed in a statement block. For example,

    if age >= 50 {
        call aarp(name)
    }

This tests the variable age to determine if it is greater than or
equal to the integer value 50, and if so, it calls the function 
named `aarp` with the value of the `name` symbol.

You can optionally include an "else" clause to execute if the
condition is false, as in 

    if flag == "-d" {
        call debug()
    } else {
        call regular()
    }

If the value of `flag` does not equal the string "-d" then the 
code will call the function `regular()` instead of `debug()`.

### print
The `print` statement accepts a list of expressions, and displays them on
the current stdout stream. There is no formatting built into the `print`
statement at this time; each term in the list is printed sequentially,
and a newline is added after all the items are printed.

    print "My name is ", name


Using `print` without any arguments just prints a newline character.

### call
The `call` statement is used to invoke a function that does not return
a value. It is followed by a function name and arguments, and these are
used to call the named function. However, even if the function uses a
`return` statement to return a value, it is ignored by the `call` 
statement. 

    call profile("secret", "cookies")

This calls the `profile()` function. When that function gets two
paramters, it sets the profile value named in the first argument to
the string value of the second argument. The function returns true
because all functions must return a value, but the `call` statement
discards the result.  This is the same as using the statement:

    
    x := profile("secret", "cookies")

Where the "x" is the name of an ignored value.


### func
The `func` statement defines a function. This must have a name
which is a valid symbol, followed by an argument list. 

The argument list is a list of names which become local variables in the running
function, set to the value of the arguments from the caller. After each argument
name in the `func` statement, you can specify a type of `int`, `string`, `float`, 
`bool`, `struct`, or `array`, in which case the value is
coerced to that type regardless of the value passed into the function.

After the  (possibly empty)  argument list you must specify the type of the
function's return value. This can be one of the base
types (`int`, `float`, `string`, or `bool`). It can also be `[]` which
denotes a return of an array type, or `{}` which denotes the return
of a `struct` type. Finally, the type can be `any` which means any
type can be returned, or `void` which means no value is returned from
this function (it is intended to be invoked as a `call` statement).

The type declaration is then followed by
a statement or block defining the code to execute when the function
is used in an expression or in a `call` statement. For example,

    func double(x) float {
        return x * 2
    }

This accepts a single value, named `x` when the function is running.
The function returns that value multiplied by 2. The type of the result
is coerced to be a float value. Note that the braces are not required
in the above example since the function consists of a single `return`
statement, but by convention braces are always used to indicate the
body fo the function. The function just created can
then be used in an expression, such as:

    fun := 2
    moreFun := double(fun)

After this code executes, `moreFun` will contain the value 4.0 as a
float value.

### return
The `return` statement contains an expression that is identified as
the result of the function value. The generated code adds the value
to the runtime stack, and then exits the function. The caller can
then retrieve the value from the stack to use in an expression or
statement.
    
    return salary/12.0

This statement returns the value of the expression `salary/12.0` as
the result of the function.

If you use the `return` statement with no value, then the function
simply stops without leaving a value on the arithmetic stack. This is
the appropriate behavior for a function that is meant to be invoked
with a `call` statement.

### for
The `for` statement defines a looping construct. A single statement
or a statement block is executed based on the definition of the 
loop. There are two kinds of loops.

    x := [101, 232,363]
    for n:=0; n < len(x); n = n + 1 {
        print "element ", n, " is ", x[n]
    }

This example creates an array, and then uses a loop to read all
the values of the array. The `for` statement is followed by three
clauses, each separated by a ";" character. The first clause must
be a valid assignment that initializes the loop value. The second
clause is a condition which is tested at the start of each loop;
when the condition results in a false value, the loop stop 
executing. The third clause must be a statement that updates the
loop value.  This is followed by a block containing the statement(s)
to execute each time through the loop.

When using a loop to index over an array, you can use a short
hand version of this.

    x := [ 101, 232, 363 ]
    for n := range x {
        print "The value is ", n
    }

In this example, the value of `n` will take on each element of
the array in turn as the body of the loop executes. You can
have the `range` option give you both the index number and
the value.

    x := [ 101, 232, 363 ]
    for i, n := range x {
        print "Element ", i, " is ", n
    }

Here, the array index is stored in `i` and the value of
the array index is stored in `n`. This is symantically
identical to the following more explicit loop structure:

    for i := 1; i <= len(x); i = i + 1 {
        n := x[i]
        print "Element ", i, " is ", n
    }

### break
The `break` statement exits from the currently running loop, as if the
loop had terminated normally.

    for i := 0; i < 10; i = i + 1 {
        if i == 5 {
            break
        }
        print i
    }

This loop run run only five times, printing the values 0..4. On the next
iteration, because the index `i` is equal to 5, the loop is terminated.
Note that a `break` will only exit the current loop; if there are nested
loops the `break` only exits the loop in which it occurred and all outer
loops continue to run.

The `break` statement cannot be used outside of a `for` loop.

### continue
The `continue` statement exits from the current iteration of the loop, 
as if the loop had restarted with the next iteration.

    for i := 0; i < 10; i = i + 1 {
        if i == 5 {
            continue
        }
        print i
    }

This loop run run only all ten times, but will only print the values
0..4 and 6..9. When the index `i` is equal to 5, the loop starts again
at the top of the loop with the next index value.

The `continue` statement cannot be used outside of a `for` loop.

### Error handling
You can use the `try` statement to run a block of code (in the same
scope as the enclosing statement) and catch any runtime errors that
occur during the execution of that block. The error causes the code
to execute the code in the `catch` block of the statement.
If there are no errors,  execution continues after the catch block.

    x := 0
    try {
        x = pay / hours
    } catch {
        print "Hours were zero!"
    }
    print "The result is ", x

If the value of `hours` is non-zero, the assignment statement will assign
the dividend to `x`. However, if hours is zero it will trigger a runtime
divide-by-zero error. When this happens, the remainder of the statements
(if any) in the `try` block are skipped, and the `catch` block is executed.
Within this block, there is a variable `_error_` that is set to the
value of the error that was signalled. This can be used in the `catch`
block if it needs handle more than one possible error, for example.

### package
Use the `package` statement to define a set of related functions in 
a package in the current source file. A give source file can only
contain one package statement and it must be the first statement.

    package factor

This defines that all the functions and constants in this module will
be defined in the `factor` package, and must be referenced with the 
`factor` prefix, as in

    y := factor.intfact(55)

This calls the function `intfact()` defined in the `factor` package.

### import
Use the `import` statement to include other files in the compilation
of this program. The `import` statement cannot appear within any other
block or function definition. Logically, the statement stops the
current compilation, compiles the named object (adding any function
and constant definitions to the named package) and then resuming the
in-progress compilation.

    import factor
    import "factor"
    import "factor.ego"

All three of these have the same effect. The first assumes a file named
"factor.ego" is found in the current directory. The second and third
examples assume the quoted string contains a file path. If the suffix
".ego" is not included it is assumed.

If the import name cannot be found in the current directory, then the
compiler uses the environment variables EGO_PATH to form a directory
path, and adds the "lib" directory to that path to locate the import.
So the above statement could resolve to `/Users/cole/ego/lib/factor.ego`
if the EGO_PATH was set to "~/ego".

Finally, the `import` statement can read an entire directory of source
files that all contribute to the same package. If the target of the
import is a directory in the $EGO_PATH/lib location, then all the
source files within that directory area read and processed as part
of one package. 

### @error
You can generate a runtime error by adding in a `@error` directive, 
which is followed by a string expression that is used to formulate
the error message text. 

    v = "unknonwn"
    @error "unrecognized value: " + v

This will result in a runtime error being generated with the error text
"unrecognized value: unknown". This error can be intercepted in a try/catch
block if desired.

### @global
You can store a value in the Root symbol table (the table that is the
ultimate parent of all other symbols). You cannot modify an existing
readonly value, but you can create new readonly values, or values that
can be changed by the user.

    @global base "http://localhost:8080"

This creates a variable named `base` that is in the root symbol table,
with the value of the given expression. If you do not specify an expression,
the variable is created as an empty-string.

### @template
You can store away a named Go template as inline code. The template
can reference any other templates defined. 

    @template hello "Greetings, {{.Name}}"

The resulting templates are available to the template() function,
 whose first parameter is the template name and the second optional
 parameter is a record containing all the named values that might
 be substituted into the template.  For example,

     print strings.template(hello, { Name: "Tom"})

This results in the string "Greetings, Tom" being printed on the
stdout console. Note that `hello` becomes a global variable in the program, and
is a pointer to the template that was previously compiled. This
global value can only be used with template functions.


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
`Ego` uses the standard `persistence` package in `gopackages`. This allows the preferences
to be set from within the language (using the `profile` package) or using the Ego command
line `profile` subcommand. These preferences can be used to control the behavior of the Ego c
ommand-line interface, and are also used by the other subcommands that run unit test, the 
REST server, etc.

The preferences are stored in ~/.org.fernwood/ego.json which is a JSON file that contains
all the active profiles and their defaults. You can use the `ego profile` command to view
the list of available profiles, the current contents of the profiles, and to set or
delete profile items in the active profile.

Here are some common profile settings you might want to set.

### auto-import
This defaults to `false`. If set to `true`, it directs the Ego command line to automatically
import all the builtin packages so they are available for use without having to specify an
explicit `import` statement. Note this only imports the packages that have builtin functions,
so user-created packages will still need to be explicitly imported.

### exit-on-blank
Normally the Ego command line interface will continue to prompt for input from the console
when run interactively. Blank lines are ignored in this case, and you must use the `exit`
command to terminate command-line input.

If this preference is set to `true` then a blank line causes the interactive input to end,
as if an exit command was specified.

### use-readline
This defaults to `true`, which uses the full readline package for console input.This supports
command line recall and command line editing. If this value is set to `false` or `off` then 
the readline processor is not used, and input is read directly from stdin. This is intended 
to be used if the terminal/console window is not compatible with readline.

## Ego-specific Functions
The Ego program adds some additional functions to the standard suite in `gopackages`. These
are provided to support the unique features of an interactive programming interface, and
to extend the _Ego_ language to support additional data sources like web interfaces and
graph databases.

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

### rest.open(<user, password>)
This returns a rest connection handle (an opaque Go object represented by an Ego symbol
value). If the optional username and password are specified, then the request will use
Basic authentication with that username and password. Otherwise, if the logon-token 
preference item is available, it is used as a Bearer token string for authentication.

The resulting item can be used to make calls using the connection just created. For 
example, if the value of `rest.open()` was stored in the variable `r`, the following
functions would become available: 

| Function | Description |
|----------|-------------|
| r.base(url) | Specify a "base URL" that is put in front of the url used in get() or post()
| r.get(url) | GET from the named url. The body of the response (typically json or HTML) is returned as a string result value
| r.post(url [, body]) | POST to the named url. If the second parameter is given, it is a value representing the body of the POST request

&nbsp; 
&nbsp;     

Additionally, the values `r.status`, `r.headers`, and `r.response` can be used to examing the HTTP status
code of the last request, the headers returned, and the value of the response body of the last request.
The response body may be a string (if the media type was not json) or an actual object if the media type
was json.

Here's a simple example:

    
    server := rest.open().base("http://localhost:8080")
    call server.get("/services/debug")
     
    if server.status == 200 {
        print "Server session ID is ", server.response.session
    }

&nbsp; 
&nbsp;
## REST Server
You can start Ego as a REST server, which responds to standard REST calls on a given port.
When a valid REST call is made, Ego programs located in the $EGO_PATH/services directory
are used to respond to the request. Each program becomes an endpoint.

### Authentication
If you do nothing else, the server will start up and support a username of "admin" and a
password of "password" as the required Basic authentication. You can specify a JSON file
that contains a map of valid names instead with the `--users` option.  

The server would allow two usernames (_admin_ and _user_) with the associated passwords.
Additionally, if a rest call is received with an Authentication value of token followed
by a string, that string is made available to the service program, and it must determine
if the token is acceptable.

The command line option `--realm` can be used to create the name of the authentication
realm; if not specified the default realm name is "Ego Server". This is returned as part
of the 401 HTTP response when authentication is not provided.

Server startup scans the `services/` directory below the Ego path to find the Ego programs
that offer endpoint support. This directory structure will map to the endpoints that the
server responds to.  For example, a service program named `foo` in the `services/` directory 
will be referenced with an endoint like http://host:port/services/foo

It is the responsibility of each endpoint to do whatever validation is requireed for
the endpoint. To help support this, a number of global variables are set up for the
endpoint service program which describe  information about the rest call and the
credentials (if any) of the caller.

### Profile items
The REST server can be easily controlled by persistent items in the current profile,
which are set with the `ego profile set` command or via program operation using the
`profile` package.

| item | description |
|------| ------------|
| default-credential | A string value of "user:pass" describing the default credential to apply when there is no user database |
| logon-userdata | the path to the JSON file containing the user data |
| token-expiration | the default duration a token is considered value. The default is "15m" for 15 minutes |
| token-key | A string used to encrypt tokens. This can be any string value |
&nbsp; 
&nbsp;

### Global Variables
Each time a REST call is made, the program associated with the endpoint is run. When it
runs, it will have a number of global variables set already that the program can use
to control its operation.

| Name        | Type    | Description                                         |
|-------------|---------|-----------------------------------------------------|
| _body       | string  | The text of the body of the request.                |
| _headers    | struct  | A struct where the field is the header name, and the value is an array of string values for each value found  |
| _parms      | struct  | A struct where the field name is the parameter, and the value si an array of string values for each value found |
| _password   | string  | The Basic authentication password provided, or empty string if not used |
| _superuser  | bool | The username or token identity has root privileges |
| _token      | string  | The Token authentication value, or an empty string if not used |
| _token_valid | bool | The Token authentication value is a valid token |
| _user       | string  | The Basic authentication username provided, or identify of a valid Token |
&nbsp; 
&nbsp;     

### Functions
There are additional functions made available to the Ego programs run as services. These are generally used to support writing
services for administrative or privileged functions. For example, a service that updates a password would use all of the following
functions.

| Function | Description | 
|----------|------------|
| u := getuser(name) | Get the user data for a given user
| call setuser(u) | Update or create a user with the given user data
| f := authenticated(user,pass) | Boolean if the username and password are valid
&nbsp; 
&nbsp;     

A sample program might look like this:

    // Assume u is username
    //        p is password
    //        n is a new password to assign to the user

    if authenticated(u, p) {
        d := getuser(u)
        d.password = newpass
        call setuser(d)
    }
