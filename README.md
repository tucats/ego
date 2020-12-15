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

### Functions
There are additional functions made available to the Ego programs run as services. These are generally used to support writing
services for administrative or privileged functions. For example, a service that updates a password would use all of the following
functions.

| Function | Description | 
|----------|------------|
| u := getuser(name) | Get the user data for a given user
| call setuser(u) | Update or create a user with the given user data
| f := authenticated(user,pass) | Boolean if the username and password are valid

A sample program might look like this:

    // Assume u is username
    //        p is password
    //        n is a new password to assign to the user
    
    if authenticated(u, p) {
        d := getuser(u)
        d.password = newpass
        call setuser(d)
    }
