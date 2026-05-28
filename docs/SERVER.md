# Ego as a REST or Web Server

This documents using the _Ego_ web server capability. The server can respond to arbitrary
HTTPS or HTTP requests, and execute _Ego_ code or built-in functions.

## Table of Contents

1. [Introduction](#intro)
2. [Server Commands](#commands)
    1. [Starting and Stopping](#startStop)
    2. [Credentials Management](#credentials)
    3. [Profile Settings](#profile)
3. [Static Redirections](#redirects)
4. [Resource Management](#resources)
5. [Writing a Service](#services)
    1. [Request Parameter](#request)
    2. [Response Parameter](#response)
    3. [Server Directives](#directives)
    4. [Server Functions](#functions)
6. [Sample Service](#sample)
7. [OAuth2 Authentication](#oauth)
    1. [Why OAuth2?](#oauthWhy)
    2. [Ego as an Authorization Server](#oauthAS)
    3. [Ego as a Resource Server](#oauthRS)
    4. [Running AS and RS Together](#oauthBoth)
    5. [CLI Login with OAuth2](#oauthCLI)
8. [Clustering](#clustering)

&nbsp;
&nbsp;

## Introduction <a name="intro"></a>

You can start Ego as a REST server
with a specified port on which to listen for input (the default is 443). Web service
requests are handled by the server for administrative functions like logging in or
managing user credentials, and by _Ego_ programs for other service functions that
represent the actual web services features.
&nbsp;
&nbsp;

### HTTP versus HTTPS

If you start the server as an insecure (HTTP) server by specifying the insecure port
number 80 and/or using the --not-secure command line option, the server will only
listen on that port for requests, and will not use TLS security. If you start the
server on another port (by default, port 443 is used) the server will start listening
for secured connections via TLS on that port.

In addition, if a secure server is started, then a second listener is started on the
insecure port (80, by default) which redirects all requests to the secure port. This
allows a client to connect to the host name and it will work with either HTTPS or
HTTP connections (which are redirected to the HTTPS connection).

You can disable this automatic redirect of insecure connections by using the --insecure-port
option, and setting the port number to zero. A zero value means that the redirect service
should not be started when the server runs.

When running in HTTPS mode, the server will need to access a certificate and a key file.
By default, these are named "https-server.crt" and "https-server.key" and are found in
the `lib` directory of the deployment. You can override these locations using environment
variables "EGO_CERT_FILE" and "EGO_KEY_FILE" if you need to use an alternate location.

If these files do not exist, are not readable, or have an invalid format, the server will
not start, and will put errors in the server log file.

### Server subcommands <a name="commands"></a>

The `ego server` command has subcommands that describe the operations you can perform. The
commands that start or stop a rest server or evaluate its status are run on the same
computer that the server itself is running on. For each of the commands below, you can
specify the option `--port n` to indicate that you want to control the server listening
on the given port number, where `n` is an integer value for a publicly available port
number.

| Subcommand | Description |
| :-------------- | :---------- |
| start | Start a server. You can start multiple servers as long as they each have a different --port number assigned to them. |
| stop | Stop the server that is listening on the named port. If the port is not specified, then the default port is assumed. |
| restart | Stop the current server and restart it with the options. This can be used to restart a server that has run out of memory, or when upgrading the version of ego being used. |
| status | Report on the status of the server. |
| logging | Enable or disable logging on the server |
| users set | Create or update a user in the server database |
| users delete | Remove a user from the server database |
| users list | List users in the server database |
| caches list | List the endpoints currently in the service cache |
| caches flush | Flush the service cache on the server |
| caches set-size | Set the number of service endpoints the cache can hold |

&nbsp;
&nbsp;

The commands that start and stop a server only require native operating system
permissions to start or stop a process. The commands that affect user credentials
in the server can only be executed when logged into the server with a credential
that has `ego.root` privileges, as defined by the credentials database in that server.

When a server is running, it generates a log file (in the current directory, by
default) which tracks the server startup and status of requests made to the server.

#### Starting and Stopping the Server<a name="startStop"></a>

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

##### Caching

You can specify a cache size, which controls how many service programs are held in
memory and not recompiled each time they are invoked by a REST API call. This can
be a significant performance benefit. When an  endpoint call is made, the server
checks to see if the cache already contains the compiled code for that function
along with it's package definitions. If so, it is reused to execute the current
service request.

If the service program was not in the cache, it will be added to the cache. When
the cache becomes full (has met the limit on the number of programs to cache) then
the least-recently-used service program based on timestamp of the last REST call)
is removed from the cache.

The default cache size is 10 items.

##### Logging

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

##### Port and Security

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
for users to snoop for username/password pairs and authentication tokens.
It is useful to debug issues where you are attempting  to isolate whether
your are having an issue with trust certificates or not.

##### Authentication

An _Ego_ web server can serve endpoints that require authentication or not,
and whether the authentication is done by username/password versus an
authentication token. Server command options control where the credentials
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
  user the "ego.root" privilege which makes them able to perform all secured operations.
  **IMPORTANT:** After the server is configured, this option should not be used as
  it can be visible in process listings such as generated by the "ps" or "top" commands.
* Use the "--realm" option to specify a string that is sent back to web clients when
  a username/password is required but was not provided. For web clients that are
  browsers, this string is typically displayed in the username/password prompt from
  the browser.
  
&nbsp;
&nbsp;

### Credentials Management <a name="credentials"></a>

Use the `ego logon` command to logon to the server you have started, using a username and
password that has "ego.root" administrator privileges. This communicates with the web server
and asks it to issue a token that is used for all subsequent administration operations. This
token is valid for 24 hours by default; after 24 hours you must log in again using the username
and password.

Once you have logged in, you can issue additional `ego server` commands to manage the
credentials database used by the web server, and manage the service cache used to
reduce re-compilation times for services used frequently.

#### ego server users list

The command `ego server users list` (which can be abbreviated as `ego server users`) will
list all the user ids in the authentication/authorization database. To see the UUID value
for each user, add the `--id` option to the end of the command.

The command output consists of a table of the user names and their associated permissions.
Permissions are represented as a comma-separate list of keyword tokens, such as "ego.root"
or "ego.logon". Permission names are determined by the application, with only a few being used
directly by the server itself:

| Permission | Description |
| ---------- | ----------- |
| ego.root | The user is an administrator with all privileges granted |
| ego.logon | The user is allowed to logon to the server. |
| ego.table.admin | The user is allowed to administer the tables server. |
| ego.table.read | The user is allowed to read tables. |
| ego.table.modify | The user is allowed to modify or delete tables. |
| ego.server.admin | The user is allowed to administer server settings |
| ego.dsns.admin | The user is allowed to administer data source names |

Note that in addition to the table_* privileges above, individual tables may have
additional privileges associated with them, controlled by the tables service administrator.

#### ego server users create

The `ego server users create` command is used to create a new authorization and authentication
record. The user must specify the username as a parameter on the command line. Additionally,
the user can specify `--password` to define the logon password for the user, and `--permissions`
which is followed by a comma-separated list of permission names, enclosed in quotations marks.
For example,

```sh
ego server users create monica --password "flavor55#" --permissions "logon, table_read, payroll"
```

This creates a new user named "monica", with the associated password and three permissions
(`logon`, `table_read`, and `payroll`). The UUID of the user is assigned by the server when
the user is created.

#### ego server users update

The `ego server users update` command allows the administrator to update a user record in
the authentication and authorization database. The username must be specified as the
parameter to the command.

You can optionally update the user's password by specifying the `--password` option. Any
previous password is replaced.

You can add or remove permissions as well using the `--permissions` option. This is a list
of option names, each preceded by a "+" or "-" character. The entire list of permissions is
comma-separated, and must be enclosed in quotation marks. If the "+" or "-" is missing from
the permission name, then adding the permission is assumed. If the user has a permission that
is not listed in the `--permissions` list, then that permission is not affected by the `update`
command.

```sh
ego server users update monica --permissions "-payroll, +payroll_update"
```

This command removes the "payroll" permission from user "monica", and adds the "payroll_update"
permission. The other user permissions ("logon" and "table_read") are not affected by this
command.

#### ego server users delete

The `ego server users delete` command is used to delete a user record entirely from the
authorization and authentication database. The username must be supplied as the parameter
to the command. There are no additional options to this command.

&nbsp;
&nbsp;

### Profile items <a name="profile"></a>

The REST server can be easily controlled by persistent items in the current profile,
which are set with the `ego config set` command or via program operation using the
`profile` package.

| Configuration Item | Description |
| :--------------------------- | :---------- |
| ego.logon.defaultuser | A string value of "user:pass" describing the default credential to apply when there is no user database |
| ego.logon.userdata | the path to the JSON file or database containing the user authentication and authorization data |
| ego.server.default.logging | A list of the default loggers to start when running a server |
| ego.server.insecure | Set to true if SSL validation is to be disabled |
| ego.server.piddir | The location in the local file system where the PID file is stored |
| ego.server.retain.log.count | The number of previous log files to retain when starting a new server instance |
| ego.server.token.expiration | the default duration a token is considered valid. The default is "15m" for 15 minutes |
| ego.server.token.key | A string used to encrypt tokens. This can be any string value |

&nbsp;
&nbsp;

## Static Redirections <a name="redirects"></a>

In addition to user-written services, the server supports static redirections of
URL references. This can be used to support convenience URLs for HTML code, or
to allow flexible redirects for user-accessible items. The default redirects (shown
below in the sample file) can be used to access the root level of the user
documentation and the language guide.

The redirects are in a JSON file that must be located in the root of the library
location (note this is the `lib` directory within the Ego path location). This
file allows any line that starts with "//" or "#" to be treated as a comment to
support documenting the file.

Here is a sample of the file and it's JSON dictionary. The primary key value in
the dictionary is the name of the local URL; by default it is relative to the server
itself, though it can be a fully relative URL. This points to a dictionary for each
possible HTTP method, and the URL to which the client is redirected. Note that the
redirection is implemented using HTTP standards; the client receives a 301 error
and must retrieve the redirect location from the HTTP response and make the request
again to the new location (this is the standard for web browsers, `curl`, etc.)

```json
{
    "/docs": {
        "GET": "https://tucats.github.io/ego/"
    },
    "/docs/language": {
        "GET": "https://tucats.github.io/ego/LANGUAGE.html"
    }
}
```

Note the `redirects.json` file is read only once during system startup; if you need
to change the redirects you must restart the server. If there is a syntax error in the
JSON file, then the server will not start. If the file does not exist, then there is
no error and the server starts without any redirects.

&nbsp;
&nbsp;

## Resource Management <a name="resources"></a>

By default, Ego will launch a thread in the main server to execute the code for each
service request. This is the fastest way to run services, but has some risks and
limitations:

* Each service is competing with the resources of a single process with all other services.
* If a service crashes or otherwise has a catastrophic failure, it can take down the entire server.
* There is no mechanism for limiting the rate at which services are processed.
* Many services using a lot of memory can strain the memory management of the server.

The server can be run in "child services" mode. In this mode, each new REST request
launches a new child process, which executes a single service request. When the request
completes, the child process exits and all its resources are reclaimed. This resolves
most of the above issues, at the cost of slower execution. A service that runs in a few
milliseconds in a single thread may take 20-30 milliseconds for a given service request
to complete in the child service model. That is, overall performance for a given service
is slower, with the benefit that the server system overall is more easily managed and
will be less vulnerable to resource constraint failures.

There are a number of configuration options that are used to control this feature:

| Option | Description |
| ------ | ----------- |
| ego.server.child.services | If "true", run in child services mode |
| ego.server.child.services.limit | If greater than zero, limits the number of simultaneous services running |
| ego.server.child.services.timeout | If present, specifies duration ("60s") a child service waits for an execution slot |
| ego.server.child.services.dir | If present, location where service request temp files are written |
| ego.server.child.services.retain | If "true", the service request temp files are retained for debugging |

&nbsp;

If the services directory is not specified, it defaults to "/tmp" on Mac or Linux, and "c:\Temp\" on Windows.
By default, the service request temp files are deleted when the service complete execution. You can retain
them if debugging and issue where looking at the details of the request sent to the child process is helpful.

The services limit value allows you to limit the number of simultaneous child service processes are
running at one time. If this value is not set, or has a value less than 1, then there are no limits. This
means that if the server receives 100 service requests, they will all be launched as separate processes
(creating 100 child processes). Each process will exist as long as the service runs, and it is up to the
operating system to schedule these processes. Note that if there are no limits then it is possible that
the host system would consume too many system resources creating too many simultaneous processes.

If you set the limit to a number that is equal to about two times the number of CPUs available to processes,
the server will limit the number of concurrent subprocesses to this value. When all available slots are
in use, the next thread processing a request will wait, checking the number of active subprocesses once
every 100 milliseconds, until an available slot can be used. The advantage of setting the value to two
times the number of slots you have is that it allows the operating system to do scheduling of the active
child processes (accounting for waits for I/O, network activity, etc.) without consuming system resources
by starting too many processes all at once.

&nbsp;
&nbsp;

## Writing a Service <a name="services"></a>

This section covers details of writing a service. The service handler function is called automatically
by the Ego web server when a request comes in with an endpoint URL that matches the service
file location. Information about the request is provided with information about the request,
the caller, headers, and parameters via a `Request` parameter. The service's `handler()`
function is responsible for formulating a response using the handler's second argument, which
must be a `ResponseWriter` object. This object allows the handler to set the status, and write
a response body payload (either as text or JSON).

Server startup scans the `services/` directory below the Ego path to find the Ego programs
that offer endpoint support. This directory structure will map to the endpoints that the
server responds to. For example, a service program named `foo` in the `services/` directory
will be referenced with an endpoint like `http://host:port/services/foo`

### Request Parameter <a name="#request"></a>

The first parameter of the service's `handler()` function must be of type `Request`, which
describes all the information known about the request made by the caller. It is a `struct`
data type, with the following fields:

| Name | Type | Description |
| :--- | ---- | :---------- |
| Authentication | string | The kind of authentication, "none", "basic", or "token" |
| Body | string | The request body if this was a POST operation |
| Endpoint | string | The endpoint for this request |
| Headers | map | A `map[string][]string` containing all the headers |
| Media | string | "text" or "json" based on the Accept header value |
| Method | string | The request method, "GET", "POST", "DELETE", etc. |
| Parameters | map | A `map[string][]string` containing the parameters |
| Url | string | The full URL used to make the request. |
| IsJSON | bool | True if this service can return JSON data |
| IsText | bool | True if this service can return text data |
| Username | string | If authenticated, the username of the requestor |

### Response Parameter <a name="#response"></a>

The second parameter of the service's `handler()` function must be of type `Response` and
is used to send responses back to the caller. This item has no fields, but does have methods
you can call.

| Name | Parameter | Description |
| :---------- | :--------- | :---------- |
| WriteStatus | integer | Set the HTTP response status code |
| Write | any | Add the item to the response body |
| WriteJSON | any | Add a JSON representation of the parameter to the body |

&nbsp;
&nbsp;

### Server Directives <a name="#directives"></a>

There are a few compiler directives that can be used in service programs that are executed
by the server. These allow for more declarative code.

#### @endpoint "path"

This specifies the endpoint this service provides, and includes any pattern information
about how elements of the URL can be converted into local variables within the handler
being run. If used, this directive must be the first line of code in the service file.

The "path" string is an expression of the URL path, using substitution values for URL elements that are variable. The specific path for a given request is stored in the
http.Request field `URL.Path` which is a string. Additionally, `URL.Parts` is a map
for each field in the URL, indicating if the value was present in the actual request,
and if so the value provided.

If the service does not have an `@endpoint` directive, then the URL path is assumed
to be identical to the service handler program path, with no additional user elements.

#### @authenticated type

This requires that the caller of the service be authenticated, and specifies the type
of the authentication to be performed. This should be at the start of the service code;
if the caller is not authenticated then the rest of the services does not run. Valid
types are:

| Type | Description |
| :--------- | :---------- |
| none | User authentication not required for this service |
| user | User must be authenticated by username or token |
| admin | The user (regardless of authentication) must have ego.root privileges |

&nbsp;
&nbsp;

#### @json {}

The body of the code in the `{}` are executed if the current request supports JSON as the
result type. If the caller does not accept JSON, then the body is not executed.

#### @text {}

The body of the code in the `{}` is executed if the current request supports TEXT as the
result type. If the caller does not accept text, then the body is not executed.

#### @log server "string"

This adds logging messages to the server log. The "string" value is any string expression;
it is written to the server log if the server log is active (by default, this is always
active when running in server mode).

&nbsp;
&nbsp;

### Sample Service <a name="sample"></a>

This section describes the source for a simple service. This can also be found in
the lib/services directory.

```go
// Sample service. This illustrates using a collection-style URL
// endpoint, of the form specified in the @endpoint directive below.
// Note that @endpoint must be the first statement in the source
// file.
//
//  * If name and field are omitted, it lists the possible users.
//  * If field is omitted, it lists all info about a specific user.
//  * If field is given, it lists the specific field for the specific user.
@endpoint "GET services/sample/users/{{name}}/{{field}}"

import "http"

func handler(req http.Request, w http.ResponseWriter) {
    // Construct some sample data.
    type person struct {
        age    int 
        gender string 
    }

    names := map[string]person{
            "tom": {age: 51, gender:"M"},
            "mary": {age:47, gender:"F"},
    }

    // If the name wasn't part of the path, the Request
    // is for all names.
    name := r.URL.Parts["name"]
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
        resp.WriteHeader(http.StatusNotFound)
        resp.Write("No such name as " + name)
    } else {
        // Based on the item name, return the desired info.
        switch item := r.URL.Parts["field"] {
        case "":
            resp.Write(fmt.Sprintf("%v", info))

        case "age":
            resp.Write(fmt.Sprintf("%v", info.age))

        case "gender":
            resp.Write(fmt.Sprintf("%v", info.gender))
        
        // No item given, so return the whole object value
        default:
            resp.WriteHeader(http.StatusBadRequest)
            resp.Write("Invalid field selector " + item)
        }
    }
}
```

&nbsp;
&nbsp;

## OAuth2 Authentication <a name="oauth"></a>

### Why OAuth2? <a name="oauthWhy"></a>

Ego's default authentication model stores usernames, hashed passwords, and permissions in its own user database, and issues short-lived tokens after a successful logon. This works well for self-contained deployments, but becomes a burden when your organization already has centralized identity management.

**OAuth2** (RFC 6749) is the industry-standard framework for delegated authorization. Rather than requiring every application to manage its own user database, OAuth2 lets your organization rely on a dedicated **Identity Provider** (IdP) — such as Okta, Microsoft Entra ID, Auth0, Keycloak, or Google Workspace — to handle authentication centrally. Users authenticate once with the IdP, and the resulting credential can be accepted by many services, including Ego, without Ego ever seeing a password.

**OpenID Connect (OIDC)** is a thin identity layer built on top of OAuth2 that standardizes user identity claims (name, email, etc.) in a machine-readable format. All major modern IdPs support OIDC. When this document refers to an "OAuth2 provider," assume it also speaks OIDC.

#### Tokens: JWT vs. Ego's native format

Ego's native tokens are symmetrically-encrypted opaque strings that only the issuing server can read. OAuth2 uses a different format called a **JWT** (JSON Web Token, RFC 7519): a Base64-encoded JSON structure in three parts separated by dots. A JWT is not encrypted — it carries a *payload* (the "claims") and a *cryptographic signature* produced by the IdP's private key. Anyone with the IdP's corresponding public key can verify the signature without contacting the IdP on every request.

A typical JWT payload looks like:

```json
{
  "sub": "alice@example.com",
  "name": "Alice Example",
  "iss": "https://login.example.com/",
  "aud": "ego-service",
  "exp": 1716239022,
  "scope": "openid profile ego:read"
}
```

`sub` is the subject (the username), `iss` is the issuer, `aud` is the intended audience, `exp` is the expiration time (Unix timestamp), and `scope` lists what the token permits.

The IdP publishes its public signing keys at a standard URL called a **JWKS endpoint** (JSON Web Key Set). Ego fetches these keys at startup, caches them, and uses them to verify JWT signatures locally — making per-request validation a fast, local operation with no network round-trip to the IdP.

#### OAuth2 flows

OAuth2 defines several grant types for different use cases:

| Flow | Use case |
| ---- | -------- |
| **Authorization Code + PKCE** | Browser or CLI; the user logs in at the IdP's page and a short-lived code is exchanged for a token. PKCE (Proof Key for Code Exchange, RFC 7636) prevents interception attacks. |
| **Client Credentials** | Machine-to-machine; a service presents its client ID and secret directly for a token — no user login page involved. |
| **Refresh Token** | Silently renew an expired access token without prompting the user again. |

Ego's `ego logon --oauth` command uses the Authorization Code + PKCE flow. Machine-to-machine API clients can use Client Credentials directly against the token endpoint.

#### Backward compatibility

OAuth2 is an *addition* to Ego's authentication, not a replacement. In `hybrid` mode (the default when OAuth2 is enabled), Ego accepts both its native tokens and JWTs in the same `Authorization: Bearer` header. Existing clients using `ego logon` with a username and password continue to work without any changes.

#### Migration path

Ego's built-in Authorization Server role is ideal for development and testing. When you are ready for production, switching to a corporate IdP is a single configuration change — the same principle as switching from SQLite to PostgreSQL. The rest of the server configuration is unchanged.

&nbsp;

Ego has two independent but complementary OAuth2 roles:

| Role | When to use |
| ---- | ----------- |
| **Authorization Server (AS)** | You want Ego itself to issue JWTs — suitable for local development and testing. |
| **Resource Server (RS)** | You want Ego to accept JWTs issued by an external identity provider (Okta, Entra ID, Keycloak, etc.). |

Both roles can be active at the same time on the same server. They are configured independently, with all settings under the `ego.server.oauth.*` prefix.

&nbsp;

### Ego as an OAuth2 Authorization Server <a name="oauthAS"></a>

When `ego.server.oauth.as.enabled` is `true`, Ego registers the standard OIDC/OAuth2 endpoints at startup and can issue signed JWT access tokens to any registered OAuth2 client — including the Ego CLI itself.

> **Intended use:** development, automated testing, and demos. For production, use a dedicated identity provider such as Okta, Entra ID, or Keycloak.

#### Minimum configuration

```sh
ego config set ego.server.oauth.as.enabled=true
ego config set ego.server.oauth.as.issuer=http://localhost:4040
```

The issuer URL must be the publicly reachable base URL of the Ego server (scheme + host + port, no trailing slash). Ego uses it as the `iss` JWT claim and to build all OIDC discovery URLs.

#### Signing key

Ego automatically generates a P-256 ECDSA key pair the first time the AS starts and saves the private key to:

```text
{EGO_PATH}/lib/oauth/oauth-signing.pem
```

The key file and its directory are created with owner-only permissions (`0600`/`0700`). The corresponding public key is published at `/.well-known/jwks.json` for token consumers to verify signatures.

You can override the key file location:

```sh
ego config set ego.server.oauth.as.key.file=/etc/ego/signing.pem
```

#### Client registry

Registered OAuth2 clients are listed in a JSON file at `{EGO_PATH}/lib/oauth/oauth-clients.json`. The file is created automatically if it does not exist, and the directory is secured at `0700`.

A minimal client entry:

```json
[
  {
    "client_id": "myapp",
    "client_secret": "supersecret",
    "redirect_uris": ["https://myapp.example.com/callback"],
    "grant_types": ["authorization_code", "client_credentials", "refresh_token"],
    "scopes": ["openid", "profile", "ego:read"]
  }
]
```

Plaintext `client_secret` values are bcrypt-hashed in memory on load and never kept as plaintext in RAM. You may pre-hash secrets and use `client_secret_hash` instead.

> **Built-in `ego-cli` client:** The AS automatically pre-registers a public client named `"ego-cli"` if one is not already present in the client file. This is the client used by `ego logon --oauth`. Because it is a public client (no secret), it relies on PKCE (RFC 7636) instead of a shared secret. You can override the built-in registration by adding your own `"ego-cli"` entry to the client file.

#### Token lifetimes

| Setting | Default | Description |
| ------- | ------- | ----------- |
| `ego.server.oauth.as.token.expiration` | `1h` | Access token lifetime |
| `ego.server.oauth.as.refresh.expiration` | `24h` | Refresh token lifetime |
| `ego.server.oauth.as.code.expiration` | `5m` | Authorization code lifetime |

All values must be Go duration strings (`"30m"`, `"2h"`, `"24h"`, etc.).

#### Registered OIDC/OAuth2 endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/.well-known/openid-configuration` | OIDC discovery document |
| GET | `/.well-known/jwks.json` | Public signing key (JWKS) |
| GET | `/oauth2/authorize` | Displays the login form |
| POST | `/oauth2/authorize` | Processes login; issues authorization code |
| POST | `/oauth2/token` | Token endpoint (all grant types) |
| GET | `/oauth2/userinfo` | OIDC UserInfo (requires Bearer token) |
| POST | `/oauth2/revoke` | Token revocation (RFC 7009) |

#### Scope → Ego permission mapping

JWTs issued by the AS carry scope strings. Ego maps them to internal permissions as follows:

| Scope | Ego permission |
| ----- | -------------- |
| `openid`, `profile`, `email` | (identity claims only, no server permission) |
| `ego:read` | `ego.logon` |
| `ego:write` | table write access |
| `ego:admin` | `ego.root` |
| `ego:code` | `ego.code` |

#### Quick start

```sh
# 1. Configure and start the AS on port 4040 (insecure for local testing)
ego config set ego.server.oauth.as.enabled=true
ego config set ego.server.oauth.as.issuer=http://localhost:4040
ego server start -k --port 4040

# 2. Verify the discovery document
curl http://localhost:4040/.well-known/openid-configuration

# 3. Log in from the CLI using OAuth2
ego logon --oauth --logon-server http://localhost:4040

# 4. Issue a client_credentials token (machine-to-machine)
curl -X POST http://localhost:4040/oauth2/token \
  -d "grant_type=client_credentials&client_id=myapp&client_secret=supersecret&scope=openid ego:read"
```

After `ego logon --oauth`, all subsequent `ego server` and `ego` commands use the obtained JWT transparently — no other changes needed.

&nbsp;

### Ego as an OAuth2 Resource Server <a name="oauthRS"></a>

When `ego.server.oauth.provider` is set to the base URL of an external identity provider, Ego acts as a resource server: it accepts JWT Bearer tokens in API requests and validates them by fetching the provider's OIDC discovery document and JWKS public keys.

#### Minimum RS configuration

```sh
ego config set ego.server.oauth.provider=https://dev-12345.okta.com/oauth2/default
ego config set ego.server.oauth.client.id=my-ego-client-id
```

#### Integration modes

Three modes are available via `ego.server.oauth.mode`:

| Mode | Behavior |
| ---- | -------- |
| `resource-server` | Accept JWT Bearer tokens only. Clients must obtain JWTs from the IdP directly. |
| `proxy` | Redirect browser logins to the IdP; exchange the resulting code for a native Ego token. Existing clients that expect Ego tokens continue to work. |
| `hybrid` _(default)_ | Accept both JWT Bearer tokens and native Ego tokens. Also supports proxy logins. |

Set the mode:

```sh
ego config set ego.server.oauth.mode=hybrid
```

#### Full RS configuration reference

| Setting | Description |
| ------- | ----------- |
| `ego.server.oauth.provider` | IdP base URL (required — activates RS role) |
| `ego.server.oauth.client.id` | Application (client) ID assigned by the IdP |
| `ego.server.oauth.client.secret` | Client secret (also via `EGO_OAUTH_CLIENT_SECRET` env var) |
| `ego.server.oauth.scopes` | Space-separated scopes to request (e.g. `"openid profile ego:read"`) |
| `ego.server.oauth.redirect.uri` | Callback URL registered with the IdP (required for `proxy`/`hybrid` modes) |
| `ego.server.oauth.user.claim` | JWT claim used as the Ego username (default: `"sub"`) |
| `ego.server.oauth.permission.claim` | JWT claim used for role/scope mapping (default: `"scope"`) |
| `ego.server.oauth.audience` | Expected `"aud"` claim value (optional; skip validation when empty) |
| `ego.server.oauth.mode` | Integration mode: `"resource-server"`, `"proxy"`, or `"hybrid"` |
| `ego.server.oauth.jwks.cache.ttl` | How long to cache the IdP's public keys (default: `"1h"`) |
| `ego.server.oauth.permission.map` | Comma-separated `scope=permission` pairs (see below) |

#### Mapping IdP scopes to Ego permissions

Use `ego.server.oauth.permission.map` to translate IdP scope strings to Ego permission names. The value is a comma-separated list of `scope=permission` pairs:

```sh
ego config set ego.server.oauth.permission.map="ego:admin=root,ego:write=tables,ego:read=logon,ego:code=code_run"
```

Any scope not listed in the map is ignored. When this setting is empty, a built-in default mapping is used (see the table in the AS section above).

#### Quick start (external IdP)

```sh
# 1. Configure the RS
ego config set ego.server.oauth.provider=https://dev-12345.okta.com/oauth2/default
ego config set ego.server.oauth.client.id=my-ego-client-id
ego config set ego.server.oauth.client.secret=my-client-secret
ego config set ego.server.oauth.mode=hybrid
ego config set ego.server.oauth.redirect.uri=https://ego.example.com/services/admin/oauth/callback

# 2. Start the server
ego server start

# 3. Log in from the CLI using the external IdP
ego logon --oauth --logon-server https://ego.example.com \
                  --oauth-server https://dev-12345.okta.com/oauth2/default
```

The `--oauth-server` flag tells the CLI which OIDC issuer to target for the browser login flow. The `--logon-server` flag sets the Ego API server URL for subsequent commands.

&nbsp;

### Running AS and RS together <a name="oauthBoth"></a>

Both roles can be enabled on the same server. A common development setup uses Ego's own AS to issue tokens _and_ accepts them as a resource server:

```sh
ego config set ego.server.oauth.as.enabled=true
ego config set ego.server.oauth.as.issuer=http://localhost:4040
ego config set ego.server.oauth.provider=http://localhost:4040
ego config set ego.server.oauth.mode=hybrid
ego server start -k --port 4040

ego logon --oauth --logon-server http://localhost:4040
```

To run AS and RS on separate ports (for integration testing), use named profiles:

```sh
# Authorization Server on port 4040
ego -p as config set ego.server.oauth.as.enabled=true
ego -p as config set ego.server.oauth.as.issuer=http://localhost:4040
ego -p as server start -k --port 4040

# Resource Server on port 8080, trusting the AS above
ego config set ego.server.oauth.provider=http://localhost:4040
ego config set ego.server.oauth.mode=hybrid
ego server start -k --port 8080

# CLI logs into the AS, then uses the token against the RS
ego logon --oauth --logon-server http://localhost:8080 \
                  --oauth-server http://localhost:4040
```

&nbsp;

### CLI OAuth2 login (`ego logon --oauth`) <a name="oauthCLI"></a>

The `--oauth` flag switches `ego logon` from username/password to an OAuth2 Authorization Code + PKCE flow (RFC 7636, RFC 8252). The entire flow runs in Go; the only external dependency is a system browser.

#### How it works

1. The CLI determines the OAuth2 issuer URL (see priority order below).
2. It fetches the OIDC discovery document and locates the authorization and token endpoints.
3. It generates a random PKCE verifier and S256 challenge.
4. It starts a temporary HTTP listener on a random localhost port.
5. It prints the authorization URL to the console and attempts to open it in the default browser.
6. The user logs in; the AS redirects to `http://localhost:{port}/callback` with an authorization code.
7. The CLI exchanges the code + verifier for an access token (and optional refresh token).
8. The tokens are stored in the active profile; all subsequent commands use the JWT transparently.

On headless systems (SSH, no GUI), the browser open silently fails; the user copies the printed URL into a browser on any machine and completes the login there. The CLI waits up to 2 minutes for the callback.

#### Issuer URL resolution (priority order)

1. `--oauth-server URL` — explicit CLI flag
2. `ego.logon.oauth.server` — config setting
3. `ego.server.oauth.as.issuer` — Ego's own AS
4. `ego.server.oauth.provider` — external IdP (RS config)

#### CLI OAuth2 settings

| Setting | Default | Description |
| ------- | ------- | ----------- |
| `ego.logon.oauth.server` | *(auto-detect)* | OAuth2 issuer URL override for CLI login |
| `ego.logon.oauth.client.id` | `ego-cli` | OAuth2 client_id presented by the CLI |
| `ego.logon.oauth.scopes` | `openid profile` | Scopes requested; `openid` is always appended if missing |

#### Refresh tokens

If the AS issues a refresh token, the CLI stores it automatically. On the next `ego logon --oauth`, the CLI attempts a silent token refresh without opening a browser. If the refresh token has expired or been revoked, it falls back to the full browser flow.

&nbsp;

## Clustering <a name="clustering"></a>

Multiple Ego server instances can be grouped into a **named cluster** so they share system state
and keep each other's in-memory caches consistent.  Clustering requires a shared file system that
all nodes can read and write — typically a network volume or, in simple two-node cases, a local
filesystem path that both nodes access over the same host.

### When to use clustering

- You run two or more Ego servers behind a load balancer and need user credentials, DSN definitions,
  and table-schema cache to be consistent across nodes.
- You want graceful rolling restarts where in-flight tokens remain valid regardless of which node
  handles the next request.

### Starting a clustered node

Add the `--cluster <name>` flag when starting the server:

```sh
ego server start --cluster myCluster -k --port 4040
ego server start --cluster myCluster -k --port 4041
```

Every node that shares the same `--cluster` name and can access the same system database file (set
by `--users` or `ego.server.userdata`) automatically discovers its peers and begins exchanging
cache-invalidation notices.

> **Shared database requirement:** all nodes in a cluster must point to the same SQLite file path
> (or the same PostgreSQL database). The SQLite file must be on a shared, POSIX-compatible
> filesystem so that concurrent reads and WAL-based writes work correctly.

### Cluster membership commands

After starting clustered nodes, use the following commands to inspect and manage them:

#### ego server cluster show

Displays the current membership table — node IDs, hostnames, ports, states, and join/last-seen timestamps.

```sh
ego server cluster show
```

The equivalent verb-object form is `ego cluster server show` (or just `ego cluster server`).

#### ego server cluster stop

Sends a graceful shutdown request to one or all active nodes.

```sh
ego server cluster stop --node <node-id>   # stop one node
ego server cluster stop --all              # stop every active node
```

The command retrieves the current membership list, then POSTs a shutdown request directly to each
target node's URL.  Admin credentials (obtained via `ego server logon`) are used to authenticate
the request.

#### ego server cluster remove

Forcibly evicts a non-responsive node from the membership table without contacting it.  Use this
when a node has crashed and you do not want to wait for the automatic 90-second health-check eviction.

```sh
ego server cluster remove --node <node-id>
```

### Health checking and automatic eviction

Each node runs a background health-checker goroutine that pings every other active peer every 30 s
(configurable via `ego.cluster.ping.interval`).  A peer that fails to respond three consecutive
times (90 s total) is automatically evicted from the membership table.

You can tune both intervals in the profile:

```sh
ego config set ego.cluster.ping.interval=45s
ego config set ego.cluster.ping.timeout=10s
```

### Cache invalidation

Whenever a write operation modifies shared state on one node (user records, DSN definitions, table
schemas, blacklisted tokens), that node purges its local in-memory cache and broadcasts a flush
request to every active peer.  Peers discard the named cache and reload fresh data from the shared
database on the next request.

This is transparent to application code — no changes to service endpoints are required.

### Security model

Node-to-node cache-flush requests are authenticated with an HMAC-SHA256 token derived from
`ego.server.token.key`.  Because all nodes share the same configuration (and therefore the same
key), any node can verify the token without a separate secret exchange.  The derived token begins
with `"cluster-"` and is never a valid user session token.

CLI management commands (`cluster stop`, `cluster remove`) use standard admin credentials stored
by `ego server logon`.
