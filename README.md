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
| ego.logon.defaultuser | A string value of "user:pass" describing the default credential to apply when there is no user database |
| ego.logon.userdata | the path to the JSON file containing the user data |
| ego.token.expiration | the default duration a token is considered value. The default is "15m" for 15 minutes |
| ego.token.key | A string used to encrypt tokens. This can be any string value |
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

### Server Directives
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
