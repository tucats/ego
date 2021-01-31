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


### make
The `make` pseudo-function is used to allocate an array, or a channel with
the capacity to hold multiple messages. This is called a pseudo-function
because part of the parameter processing is handled by the compiler to
identify the type of the array or channel to create. 

The first argument must be a data type specification, and the second argument
is the size of the item (array elements or channel messages)

    a := make([]int, 5)
    b := make(chan, 10)


The first example creates an array of 5 elements, each of which is of type `int`,
and initialized to the _zero value_ for the given type. This could have been
done by using `a := [0,0,0,0,0]` as a statment, but by using the make() function
you can specify the number of elements dynamically at runtime.

The second example creates a channel object capable of holding up to 10 messages.
Creating a channel like this is required if the channel is shared among many threads.
If a channel variable is declare by default, it holds a single message. This means
that before a thread can send a value, another thread must read the value; if there
are multiple threads waiting to send they are effectively run one-at-a-time. By
creating a channel that can hold multiple messages, up to 10 (in the above example)
threads could send a message to the channel before the first message was read.


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

### @type static|dynamic
You can temporarily change the langauge settings to allow static
typing of data only. When in static mode,

* All values in an array constant must be of the same type
* You cannot store a value in a variable of a different type
* You cannot create or delete structure members

This mode is effective only within the current statement block
(demarcated by "{" and "}" characters). When the block finishes,
type enforcement returns to the state of the previous block. This
value is controlled by the static-types preferences item or
command-line option.

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
The string is evaluated as an expression
and the result is returned as the function value. This allows user input to contain complex
expressions or function calls that are dynamically compiled at runtime. 

### rest.New(<user, password>)
This returns a rest connection handle (an opaque Go object represented by an Ego symbol
value). If the optional username and password are specified, then the request will use
Basic authentication with that username and password. Otherwise, if the logon-token 
preference item is available, it is used as a Bearer token string for authentication.

The resulting item can be used to make calls using the connection just created. For 
example, if the value of `rest.New()` was stored in the variable `r`, the following
functions would become available: 

| Function | Description |
|----------|-------------|
| r.Base(url) | Specify a "base URL" that is put in front of the url used in get() or post()
| r.Get(url) | GET from the named url. The body of the response (typically json or HTML) is returned as a string result value
| r.Post(url [, body]) | POST to the named url. If the second parameter is given, it is a value representing the body of the POST request
| r.Delete(url) | DELETE to the named URL
| r.Media("type") | Specify the media/content type of the exchange
| r.Verify(b) | Enable or disable TLS server certificate validation
| r.Auth(u,p) | Estabish BasicAuth with the given username and password strings
| r.Token(t) | Establish Bearer token auth with the given token value


&nbsp; 
&nbsp;     

Additionally, the values `r.status`, `r.headers`, `r.cookies`, and `r.response` can be used to examing the HTTP status
code of the last request, the headers returned, and the value of the response body of the last request.
The response body may be a string (if the media type was not json) or an actual object if the media type
was json.

Here's a simple example:

    
    server := rest.New().Base("http://localhost:8080")
    server.Get("/services/debug")
     
    if server.status == http.StatusOK {
        print "Server session ID is ", server.response.session
    }

&nbsp; 
&nbsp;
### db.New("connection-string-url")
There is a simplified interface to SQL databases available. By default, the only provider supported
is Postgres at this time.

The result of the db.New() call is a database handle, which can be used to execute statements or
return results from queries.

     d := db.New("postgres://root:secrets@localhost:5432/defaultdb?sslmode=disable")
     r, e := d.QueryResult("select * from foo")
     d.Close()

This example will open a database connection with the specified URL, and perform a query that returns
a result set. The result set is an Ego array of arrays, containing the values from the result set.
The Query function call always returns all results, so this could be quite large with a query that
has no filtering. You can specify parameters to the query as additional argument, which are then
substituted into the query, as in:

     age := 21
     r, e := d.QueryResult("select member where age >= $1", age)

The parameter value of `age` is injected into the query where the $1 string is found.

Once a database handle is created, here are the functions you can call using the handle:

| Function | Description |
|----------|-------------|
| d.Begin() | Start a transaction on the remote serve for this connection. There can only be one active transaction at a time
| d.Commit() | Commit the active transation
| d.Rollback() | Roll back the active transaction
| d.QueryResult(q [, args...]) | Execute a query string with optional arguments. The result is the entire query result set.
| d.Query(q, [, args...]) | Execute a query and return a row set object
| d.Execute(q [, args...]) | Execute a statement with optional arguments. The result is the number of rows affected.
| d.Close() | Terminate the connection to the database and free up resources.
| d.AsStruct(b) | If true, results are returned as array of struct instead of array of array.

When you use the Query() call it returns a rowset object. This object can be used to step through the
result set a row at a time. This allows the underlying driver to manage buffers and large result sets
without filling up memory with the entire result set at once.

| Function | Description |
|----------|-------------|
| r.Next() | Prepare the next row for reading. Returns false if there are no more rows
| r.Scan() | Read the next row and create either a struct or an array of the row data
| r.Close() | End reading rows and release any resources consumed by the rowset read.


&nbsp; 
&nbsp;
### tables.New("colname" [, "colname"...])
This gives access to the table formatting and printing subsystem for Ego programs. The
arguments must be the names of the columns in the resulting table. These can be passed 
as descrete arguments, or as an array of strings. The result is a TableHandle object 
that can be used to insert data into the table structure, sort it, and format it for
output.

    
    t := tables.New(":Identity", "Age:", "Address")
    
    t.AddRow( {Identity: "Tony", Age: 61, Address: "Main St"})
    t.AddRow( {Identity: "Mary", Age: 60, Address: "Elm St"})
    t.AddRow( {Identity: "Andy", Age: 61, Address: "Elm St"})
    
    t.Sort( "Age", "Identity" )
    
    t.Format(true,false)
    t.Print()
    t.Close()
    
This sample program creates a table with three column headings. The use of the ":" 
character controls alignment for the column. If the colon is at the left or right
side of the heading, that is how the heading is aligned. If no colon is used, then
the default is left-aligned.

The data is added
for three rows. Note that data can be added as either a struct, where the column names
must match the column names. Alternatively, the values can be added as separate arguments
to the function, in which case they are added in the order of the column headings.

The format of the table is further set by sorting the data by Age and then Identity, and 
indicating that headings are to be printed, but underlines under those headings are not.
The table is then printed to the default output and the memory structures are released.

&nbsp; 
&nbsp;
## REST Server
You can start Ego as a REST server, which responds to standard REST calls on a given port.
When a valid REST call is made, Ego programs located in the $EGO_PATH/services directory
are used to respond to the request. Each program becomes an endpoint.
### Server subcommands
The `ego server` command has subcommands that describe the operations you can perform. The
commands that start or stop a rest server or evaluate it's status must be run on the same
computer that the server itself is running on. For each of the commands below, you can 
specify the option `--port n` to indicate that you want to control the server listening on the given port number, where
`n` is an integer value for a publically available port number.

| Subcommand | Description |
|------------| ------------|
| start | Start a server. You can start multiple servers as long as they each have a different --port number assigned to them. |
| stop | Stop the server that is listening on the named port. If the port is not specified, then the default port is assumed. |
| restart | Stop the current server and restart it with the exact same command line values. This can be used to restart a server that has run out of memory, or when upgrading the version of ego being used. |
| status | Report on the status of the server. |
| users set | Create or update a user in the server database |
| users delete | Remove a user from the server database |
| users list | List users in the server database |
| caches list | List the endpoints currently in the service cache |
| caches flush | Flush the service cache on the server |
| caches set-size | Set the number of service endpoints the cache can hold |

&nbsp;
&nbsp;

The commands that start and stop a server only required native operating system permissions to start or stop
a process. The commands that affect user credentials in the server can only be executed when logged into the
server with a process that has `root` privileges, as defined by the credentials database in that server.

When a server is running, it generates a log file (in the current directory, by default) which tracks the server startup and status of requests made to the server.

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
| logon-defaultuser | A string value of "user:pass" describing the default credential to apply when there is no user database |
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

### Directives
There are a few compiler directives that can be used in service programs that are executed by the
server. These allow for more declarative code.

`@authenticated type`
This requires that the caller of the service be authenticated, and specifies the type of the authentication
to be performed. This should be at the start of the service code; if the caller is not authenticated then the
rest of the services does not run.  Valid types are:

| Type | Description |
| --- | --- |
| any | User can be authenticated by username or token |
| token | User must be authenticated by token only |
| user | User must be authenticated with username/password only |
| admin | The user (regardless of authentication) must have root privileges |
| tokenadmin | The user must authenticated by token and have root privilieges |

&nbsp;
&nbsp;

`@status n`
This sets the REST return status code to the given integer value. By default, the status value is 200
for "success" but can be set to any valid integer HTTP status code. For example, 404 means "not found"
and 403 means "forbidden".

`@response v`
This adds the contents of the expression value `v` to the result set returned to the caller. You
can have multiple `@response` directives, they are accumulated in the order executed. A primary 
value of this is also that it automatically detects if the media type is meant to specify JSON
results; in this case the value is automatically converted to a JSON string before being added
to the response.

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
