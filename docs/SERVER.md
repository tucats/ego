
# Table of Contents

1. [Introduction](#intro)
2. [Server Commands](#commands)
    1. [Starting and Stopping](#startstop)
    2. [Credentials Management](#credentials)
    3. [Profile Settings](#profile)
3. [Writing a Service](#services)
    1. [Request Parameter](#request)
    2. [Response Parameter](#response)
    3. [Server Directives](#directives)
    4. [Server Functions](#functions)
4. [Sample Service](#sample)

&nbsp

```text
     ____
    / ___|    ___   _ __  __   __   ___   _ __
    \___ \   / _ \ | '__| \ \ / /  / _ \ | '__|
     ___) | |  __/ | |     \ V /  |  __/ | |
    |____/   \___| |_|      \_/    \___| |_|
```

&nbsp;
&nbsp;

# Ego Web Server <a name="intro"></a>

This documents using the _Ego_ web server capability. You can start Ego as a REST server
with a specified port on which to listen for input (the default is 8080). Web service
requests are handled by the server for administrative functions like logging in or
managing user credentials, and by _Ego_ programs for other service functions that
represent the actual web services features.
&nbsp;
&nbsp;

## Server subcommands <a nanme="commands"></a>

The `ego server` command has subcommands that describe the operations you can perform. The
commands that start or stop a rest server or evaluate its status are run on the same
computer that the server itself is running on. For each of the commands below, you can
specify the option `--port n` to indicate that you want to control the server listening
on the given port number, where `n` is an integer value for a publically available port
number.

| Subcommand      | Description |
|:----------------|:------------|
| start           | Start a server. You can start multiple servers as long as they each have a different --port number assigned to them. |
| stop            | Stop the server that is listening on the named port. If the port is not specified, then the default port is assumed. |
| restart         | Stop the current server and restart it with the options. This can be used to restart a server that has run out of memory, or when upgrading the version of ego being used. |
| status          | Report on the status of the server. |
| logging         | Enable or disable logging on the server |
| users set       | Create or update a user in the server database |
| users delete    | Remove a user from the server database |
| users list      | List users in the server database |
| caches list     | List the endpoints currently in the service cache |
| caches flush    | Flush the service cache on the server |
| caches set-size | Set the number of service endpoints the cache can hold |

&nbsp;
&nbsp;

The commands that start and stop a server only require native operating system
permissions to start or stop a process. The commands that affect user credentials
in the server can only be executed when logged into the server with a credential
that has `root` privileges, as defined by the credentials database in that server.

When a server is running, it generates a log file (in the current directory, by
default) which tracks the server startup and status of requests made to the server.

### Starting and Stopping the Server<a name="startstop"></a>

The `ego server start` command accepts command line options to describe the port
on which to  listen,  whether or not to use secure HTTPS, and options that control
how authentication is handled. The  `ego server stop` command stops a running
server. The `ego server restart` stops  and restarts a server using the options
it was used to start up originally.

You can also run the server from the shell in the current process (instead of
detaching it as a separate process) using the `ego server run` command option.
This accepts the same options as `ego server start` and runs the code directly
in the current shell, sending logging to stdout.

When a server is started, a file is created (by default in ~/.ego) that
describes the server status and command-line options. This information is re-read
when issuing a `ego server status` command to display server information. It is also
read by the `ego server restart`  command to determine the command-line options
to use with the restarted server.

When a server is stopped via `ego server stop`, the server status file is deleted.

Below is additional information about the options that can be used for the `start`
and `run` commands.

#### Caching

You can specify a cache size, which controls how many service programs are held in
memory and not recompiled each time they are invoked by a REST API call. This can
be a significant performance benefit. When an  endpoint call is made, the server
checks to see if the cache already contains the compiled code for that function
along with it's package definitions. If so, it is reused to execute the current
service request.

If the service program was not in the cache, it will be added to the cache.  When
the cache becomes full (has met the limit on the number of programs to cache) then
the least-recently-used service program based on timestamp of the last REST call)
is removed from the cache.

The default cache size is 10 items.

#### /code Endpoint

By default, the server will only run services already stored in the services
directory tree (more on that below). When you start the web service, you can
optionally enable the `/code` endpoint. This accepts a text body and runs it
as a program directly. This can be used for debugging purposes or diagnosing
issues with a server. This should **NOT** be left enabled by default, as it
exposes the server to security risks.

#### Logging

By default, the server generates a log file (named "ego-server-_timestamp_.log"
in the  current directory where the `server start` command is issued. This
contains entries describing server operations (like records of endpoints called,
and HTTP status returned). It also contains a periodic display of memory
consumption by the server. By default, the log file is closed and a new one
opened (with a new timestamp) every night at midnight. This means that, in
general, there is a single log file representing each day's activity.

You can override the location of the log file using the `--log` command line
option, and specifying the location and file name where the log file is to
be written. The log will continue to be written to as long as the server is
running. Note that the first line of the log file contains the UUID of the
server session, so you can correlate a log to a running instance of the server.

You can specify `--no-log` if you wish to suppress logging.

#### Port and Security

Specify the `--port` option to indicate the integer port number that _Ego_
should use to listen for REST requests. If not specified, the default is port
8080. You can have multiple _Ego_ servers running at one time, as long as
they each use a different port number. The port number is also used in other
commands like `server status` to report on the status of a particular
instance of the server on the current computer.

By default, _Ego_ servers assume HTTPS communication. This requires that
you have specified a suitable trusted certificate store in the default
location on your system that _Ego_ can use to verify server trust

If you wish to run in insecure mode, you can use the "--not-secure" option
direct the server  to listen for HTTP requests that are not encrypted. You
must not use this mode in a production environment, since it is possible
for users to snoop for username/password  pairs and authentication tokens.
It is useful to debug issues where you are attempting  to isolate whether
your are having an issue with trust certificates or not.

#### Authentication

An _Ego_ web server can serve endpoints that require authentication or not,
and whether the authentication is done by username/password versus an
authentication token. Server command options control where the credentails
are stored, the default  "super user" account, and the security "realm"
used for password challenges to web clients.

* Use the `--users` command line option to specify either the file system path and
  file name to use for local JSON data that contains the credentials information, or
  a database URL expression (with scheme "postgres://" or "sqlite://") that
  indicates the Postgres or sqlite database used to store the credentials (in
  a schema named "ego-server" that is created if needed).
* Use the `--superuser` option to specify a "username:password" string indicating
  the default superuser. This is only needed when the credentials store is first
  initialized; it creates a user with the given username and password and gives that
  user the "ROOT" privilege which makes them able to perform all secured operations.
  **IMPORTANT:** After the server is configured, this option should not be used as
  it can be visible in process listings such as generated by the "ps" or "top" commands.
* Use the "--realm" option to specify a string that is sent back to web clients when
  a username/password is required but was not provided. For web clients that are
  browsers, this string is typically displayed in the username/password prompt from
  the browser.
  
&nbsp;
&nbsp;

## Credentials Management <a name="credentials"></a>

Use the `ego logon` command to logon to the server you have started, using a username and
password that has root/admin privileges. This communicates with the web server and asks it
to issue a token that is used for all subsequent administraiton operations. This token is
valid for 24 hours by default; after 24 hours you must log in again using the username and
password.

Once you have logged in, you can issue additional `ego server` commands to manage the
credentials database used by the web server, and manage the service cache used to
reduce re-compilation times for services used frequently.

&nbsp;
&nbsp;

## Profile items <a name="profile"></a>

The REST server can be easily controlled by persistent items in the current profile,
which are set with the `ego config set` command or via program operation using the
`profile` package.

| Configuration Item           | Description |
|:-----------------------------|:------------|
| ego.logon.defaultuser        | A string value of "user:pass" describing the default credential to apply when there is no user database |
| ego.logon.userdata           | the path to the JSON file or database containing the user data |
| ego.server.default.logging   | A list of the default loggers to start when running a server |
| ego.server.insecure          | Set to true if SSL validation is to be disabled |
| ego.server.piddir            | The location in the local file system where the PID file is stored |
| ego.server.reetain.log.count | The number of previous log files to retain when starting a new server instance |
| ego.server.token.expiration  | the default duration a token is considered valid. The default is "15m" for 15 minutes |
| ego.server.token.key         | A string used to encrypt tokens. This can be any string value |

&nbsp;
&nbsp;

# Writing a Service <a name="services"></a>

This section covers details of writing a service. The service handler function is called automatically
by the Ego web server when a request comes in with an endpoint URL that matches the service
file location. Information about the request is provided with information about the requst,
the caller, headers, and parameters via a `Request` parameter. The service's `handler()`
function is responsible for formulating a response using the handler's second argument, which
must be a `Response` object. This object allows the handler to set the status, and write
a response body payload (either as text or JSON).

Server startup scans the `services/` directory below the Ego path to find the Ego programs
that offer endpoint support. This directory structure will map to the endpoints that the
server responds to.  For example, a service program named `foo` in the `services/` directory
will be referenced with an endoint like `http://host:port/services/foo`

It is the responsibility of each endpoint to do whatever validation is requireed for
the endpoint. To help support this, a number of global variables are set up for the
endpoint service program which describe  information about the rest call and the
credentials (if any) of the caller.

## Request Parameter <a name="#request"></a>

The first parameter of the service's `handler()` function must be of type `Request`, which
describes all the information known about the request made by the caller. It is a `struct`
data type, with the following fields:

| Name           | Type    | Description                                              |
|:---------------|---------|:---------------------------------------------------------|
| Authentication | string  | The kind of authentication, "none", "basic", or "token"  |
| Body           | string  | The request body if this was a POST operation            |
| Endpoint       | string  | The endpoint for this request                            |
| Headers        | map     | A `map[string][]string` containing all the headers         |
| Media          | string  | "text" or "json" based on the Accept header value        |
| Method         | string  | The request method, "GET", "POST", "DELETE", etc.        |
| Parameters     | map     | A `map[string][]string` containing the parameters          |
| Url            | string  | The full URL used to make the request.                   |
| Username       | string  | If authenitcated, the username of the requestor          |

## Response Parameter <a name="#response"></a>

The second paraameter of the service's `handler()` function must be of type `Response` and
is used to send responses back to the caller. This item has no fields, but does have methods
you can call.

| Name        | Parameter  | Description |
|:------------|:-----------|:------------|
| WriteStatus | integer    | Set the HTTP response status code |
| Write       | string     | Add the string to the response body |
| WriteJSON   | any        | Add a JSON representation of the paraemter to the body |

&nbsp;
&nbsp;

## Server Directives <a name="#directives"></a>

There are a few compiler directives that can be used in service programs that are executed
by the server. These allow for more declarative code.

### @authenticated type

This requires that the caller of the service be authenticated, and specifies the type of the
authentication to be performed. This should be at the start of the service code; if the caller
is not authenticated then the rest of the services does not run.  Valid types are:

| Type       | Description |
|:-----------|:------------|
| any        | User can be authenticated by username or token |
| token      | User must be authenticated by token only |
| user       | User must be authenticated with username/password only |
| admin      | The user (regardless of authentication) must have root privileges |
| admintoken | The user must authenticated by token and have root privilieges |

&nbsp;
&nbsp;
{% raw %}

### @json {}

The body of the code in the `{}` are executed if the current request supports JSON as the
result type. If the caller does not accept JSON, then the body is not executed.

### @text {}

The body of the code in the `{}` is executed if the current request supports TEXT as the
result type. If the caller does not accept text, then the body is not executed.

### @url "/pattern"

The given pattern is applied to the current URL. The pattern can include literal values
or symbols enclosed in `{{` and `}}` characters. The part of the URL represented by those
symbols will be stored in a local variable of the given name. For example,

```go
    @url "/catalog/{{item}}/names"
```

If the request was called with a URL of "/catalog/1551/names" then the value of `item` is
set to the string "1551". If the URL does not include the constant values of the pattern,
then the handler ends and sends a 400 "Bad Request" response to the caller.

### @log server "string"

This adds logging messages to the server log. The "string" value is any string expression;
it is written to the server log if the server log is active (by default, this is always
active when running in server mode).

## Functions <a name="functions"></a>

There are additional functions made available to the Ego programs run as services. These are
generally used to support writing services for administrative or privileged functions. For example,
a service that updates a password probably would use all of the following functions.

| Function | Description |
|:---------|:------------|
| u := getuser(name) | Get the user data for a given user
| call setuser(u) | Update or create a user with the given user data
| f := authenticated(user,pass) | Boolean if the username and password are valid

&nbsp;
&nbsp;

## Sample Service <a name="sample"></a>

This section describes the source for a simple service. This can also be found in
the lib/services directory.

```go
// Sample service. This illustrates using a collection-style URI
// path, of the form:
//
//    services/sample/users/{{name}}/{{field}}
//
//  If name and field are omitted, it lists the possible users.
//  If field is omitted, it lists all info about a specific user.
//  If field is given, it lists the specific field for the specific user.

func handler( req Request, resp Response) {

    // Define the required URI syntax. This ends the request in an error
    // if the syntax is not valid in the URL we received.
    @url "/users/{{name}}/{{item}}"

    // Construct some sample data.
    type person struct {
        age    int 
        gender string 
    }

    names := map[string]person{
            "tom": {age: 51, gender:"M"},
            "mary": {age:47, gender:"F"},
    }

    // If the users collection name was not present, we can do nothing.
    if !users {
        resp.WriteStatus(400)
        resp.Write("Incomplete URL")
    }

    // If the name wasn't part of the path, the Request
    // is for all names.
    if name == "" {
        for k, _ := range names {
            resp.Write(fmt.Sprintf("%s", k))
        }

        return
    }

    // If is for a specific name, so get that information. If it doesn't
    // exist then complain.
    info, found := names[name]
    if !found {
        resp.WriteStatus(404)
        resp.Write("No such name as " + name)
    } else {
        // Based on the item name, return the desired info.
        switch item {
        case "":
            resp.Write(fmt.Sprintf("%v", info))

        case "age":
            resp.Write(fmt.Sprintf("%v", info.age))

        case "gender":
            resp.Write(fmt.Sprintf("%v", info.gender))
        
        default:
            resp.WriteStatus(400)
            resp.Write("Invalid field selector " + item)
        }
    }
}
```

{% endraw %}
