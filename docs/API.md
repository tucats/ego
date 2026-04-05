# Server API Documentation

This document describes the REST Application Programming Interfaces (APIs)
supported by an _Ego_ server instance. This covers how to authenticate to the
server, administration functions that can be performed by a suitably
privileged user, APIs for directly accessing database tables and their data
via named data source names (DSNs), and APIs for accessing user-written
services (implemented as _Ego_ programs).

## Table of Contents

1. [Introduction](#intro)

2. [Authentication](#auth)

3. [Administration](#admin)

4. [Data Sources and Tables](#dsn)

5. [Services](#services)

&nbsp;
&nbsp;

## Introduction <a name="intro"></a>

This section covers basic information about using the API functions built into
the _Ego_ server.

All REST API responses should return a server information object as a field in
the payload named "server". This contains the following information:

| Field | Description |
| :-------- | :----------- |
| api | An integer indicating the API version. Currently always 1. |
| name | The short host name of the machine running the server. |
| id | The UUID of this instance of the server. |
| session | The session correlator for the server (matches request log entries). |

The server name can be used to locate where the log files are found. The server id
helps identify when a server is restarted, as it is assigned a new UUID each time
and this information is in the server log. Finally, each logging entry from a
REST client session includes a session number. This integer value can be used to
find the specific log entries for this request.

&nbsp;
&nbsp;

## Authentication <a name="auth"></a>

Other than determining if a server is running or not, all operations performed
against an Ego server must be done with an authenticated user. The normal pattern
for this is for the client to "log in" the user, and receive an encrypted token
in return. This token can be presented to any other API to authenticate the user
for the functions of that API.

### POST /services/admin/logon

Send a `POST` request to `/services/admin/logon` to authenticate and receive a
token. There are two ways to supply credentials:

**Option 1 — JSON body.** Include a JSON payload with `username` and `password`
fields. For TLS/SSL-based communication this embeds the credentials inside the
encrypted body of the request:

| Field | Description |
| :-------- | :---------- |
| username | A string containing the username |
| password | A string containing the password |

Example request body:

```json
{
    "username": "joesmith",
    "password": "3h97q-k35Z5"
}
```

**Option 2 — Basic authentication header.** Supply credentials as an
`Authorization: Basic <base64(username:password)>` header instead of a JSON
body. This is acceptable for initial development and debugging, but the JSON
body approach is preferred for production use.

The REST status will be 403 if the credentials are invalid, or 200 if valid.

The resulting JSON payload contains the following fields:

| Field | Description |
| :-------- | :---------- |
| server | The server information object for this response. |
| expires | A string containing the timestamp of when the token expires. |
| token | A variable-length string containing the token text itself. |
| identity | The username encoded within the token. |

Example response:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 482
    },
    "expires": "Fri Jan 21 13:12:25 EST 2022",
    "identity": "joesmith",
    "token": "220de9776c7c517f84c1d4b94aadcb6e50849abed4eb6b26b9d16e3365e3a014b5fdefac5b107"
}
```

Extract the `token` field and store it for subsequent REST API calls. Pass it as
a Bearer token in the `Authorization: Bearer <token>` header.

&nbsp;
&nbsp;

### GET /services/admin/authenticate

Returns information about the current bearer token. Requires a valid bearer token
in the `Authorization` header.

The response is a JSON object describing the token identity and expiration.

&nbsp;
&nbsp;

## Administrative Functions <a name="admin"></a>

Administrative functions are REST APIs used to support managing the REST server,
including the status and state of the server, the database of valid user
credentials and permissions, and support for caching, logging, and other
server configuration functions.

* [Check if server is active/responding](#heartbeat)
* [View, flush, or set size of runtime caches](#caches)
* [View or configure logging classes on the server](#loggers)
* [Manage user credentials and permissions](#users)
* [Access HTML assets (images, etc.) used in HTML pages](#assets)
* [Read server configuration values](#config)
* [Query server memory usage](#memory)
* [Run Ego code from the dashboard](#run)
* [View the request validation dictionary](#validation)
* [Manage the token revocation list](#tokens)
* [Graceful server shutdown](#down)

&nbsp;
&nbsp;

### Heartbeat <a name="heartbeat"></a>

#### GET /admin/heartbeat

The heartbeat endpoint is the simplest and fastest way to determine if an Ego
server is running and responding to requests. It does not require authentication
of any kind and returns a 200 success code if the server is available. Any other
return code indicates that the server is not running or there is a
network/gateway problem between the REST client code and the Ego server.

This endpoint only supports the GET method, and returns no response body. It is
also not logged (it does not appear in the request log), so it can be called
frequently without polluting the log.

&nbsp;
&nbsp;

### Caches <a name="caches"></a>

The _Ego_ server maintains caches to make repeated use of the server more efficient
by saving operations in memory instead of having to re-load and re-compile services,
reload assets, etc.

You can examine what is in the server cache, direct the server to flush (i.e. evict
from memory) any cached items, and you can set the size of the services cache using
REST calls to the `/admin/caches` endpoint.

When a service is invoked by a user to execute _Ego_ code written by the developer(s)
running the _Ego_ server, the server first searches the cache to see if it has already
executed this code before. When this is the case, the server does not have to reload
the service source code from disk or compile it again. Instead, it uses the previous
results of the compile to execute the service again on behalf of the client.

When the service cannot be found in the cache, it is loaded from disk and compiled
before it can be executed. The service just compiled is placed in the cache. If the
cache is too large (based on the limit on the number of items the server is configured
to allow) the oldest (least recently used) item in the cache is discarded before
storing the newly-compiled service in the cache.

Similarly, an "asset" cache is managed by the server. When a REST call is made to
the server to the `/assets` endpoint, the remainder of the path represents the location
in the _Ego_ server's disk storage where assets are found. The asset cache is limited
by the number of bytes of storage that it can consume in memory, regardless of the
number of assets. By default, the cache is one megabyte in size.

Note that if a server has already cached a value in memory, then changing the disk copy
will not result in the updated information being used as the cached value. You must flush
the server cache to cause it to discard all cached storage and start again reading from
disk.

&nbsp;
&nbsp;

#### GET /admin/caches

Gets information about the caching status in the server. Requires admin privileges.
The result is a JSON payload with the following fields:

| Field | Description |
| :----------- | :---------- |
| server | The server information object for this response |
| host | A string containing the name of the computer running the _Ego_ server |
| id | A string containing the UUID of the server instance |
| serviceCount | The number of items in the services cache |
| serviceSize | The maximum number of items in the services cache |
| items | An array of objects for each cached item |
| assetCount | The number of assets stored in the in-memory cache |
| assetSize | The maximum size in bytes of the asset cache |

The `items` array contains an object for each item in the cache:

| Field | Description |
| :----------- | :---------- |
| name | The object name (the endpoint path used to locate it) |
| last | A string representation of the last time the item was used |
| class | Either "asset" or "service" |
| count | The number of times the item has been accessed |

Supports the `?order-by=` parameter to specify the sort order of the items array.

&nbsp;
&nbsp;

#### DELETE /admin/caches

Tells the _Ego_ server to flush the caches; all copies of service compilations and
asset objects are deleted from memory. Subsequent REST calls will require that the
server reload the item(s) from the disk store.

You must have admin privileges to execute this REST call.

Supports the `?class=` parameter to limit the flush to a specific class of cached
items (e.g. `?class=asset` or `?class=service`).

&nbsp;
&nbsp;

#### POST /admin/caches

Sets the size of the caches. You must be an admin user to execute this call.
The JSON payload may contain one or both of the following fields:

| Field | Description |
| :-------- | :---------- |
| limit | The maximum number of items in the services cache |
| assetSize | The maximum size in bytes of the asset cache |

&nbsp;
&nbsp;

### Loggers <a name="loggers"></a>

You can use the loggers endpoint to get information about the current state of
logging on the server, enable or disable specific loggers, purge old log files,
and retrieve the text of the log.

The following logger names are recognized by the server:

| Logger | Description |
| :----- | :---------- |
| APP | General application-level messages |
| ASSET | Logs asset file reads and cache operations for the `/assets` endpoint |
| AUTH | Shows authentication operations (token validation, credential checks) |
| BYTECODE | Shows disassembly of the pseudo-instructions that execute Ego programs |
| CACHE | Logs service and asset cache activity (hits, misses, evictions) |
| CHILD | Logs child process or sub-task lifecycle events |
| CLI | Logs information about command-line processing |
| COMPILER | Logs actions taken by the compiler (imports, source reads, code generation) |
| DB | Logs information about active database connections |
| DEBUG | General-purpose debug messages |
| GOROUTINE | Logs goroutine creation and lifecycle events |
| INFO | Informational messages such as server startup details and log rollover |
| INTERNAL | Internal runtime messages; enabled by default |
| OPTIMIZER | Logs bytecode optimization passes |
| PACKAGES | Logs package loading and resolution activity |
| RESOURCES | Logs access to server resource files |
| REST | Shows REST server request and response details |
| ROUTE | Logs route registration and matching decisions |
| SERVER | Logs high-level server lifecycle events (start, stop, request summary) |
| SERVICES | Logs service loading, compilation, and execution |
| SQL | Logs the SQL statements sent to the database |
| STATS | Logs server request statistics |
| SYMBOLS | Logs symbol table and symbol name operations |
| TABLES | Shows detailed table operations for the `/dsns` table endpoints |
| TOKENIZER | Logs lexer/tokenizer activity during compilation |
| TRACE | Logs execution of pseudo-instructions as they run |
| USER | Logs messages generated by `@LOG` directives in Ego programs |
| VALID | Logs request validation checks against the validation dictionary |

&nbsp;
&nbsp;

#### GET /admin/loggers/

Retrieves the current state of logging on the server. The response is a JSON
payload that indicates the host name where the server is running, its unique
instance UUID, the name of the text file on the server where the log is being
written, and a structure that indicates if each logger is enabled or disabled.

This service requires authentication with credentials for a user with
administrative privileges.

Example response:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 6385
    },
    "file": "/Users/tom/ego/ego-server_2022-01-20-000000.log",
    "loggers": {
        "AUTH": true,
        "BYTECODE": false,
        "CLI": false,
        "COMPILER": false,
        "DB": false,
        "REST": true,
        "SERVER": true,
        "SYMBOLS": false,
        "TABLES": true,
        "TRACE": false,
        "USER": false
    }
}
```

&nbsp;
&nbsp;

#### POST /admin/loggers/

Modifies the state of logging on the server. The payload must contain the
`loggers` structure that tells which loggers are to change state. Any logger
not mentioned in the payload does not have its state changed.

This service requires authentication with credentials for a user with
administrative privileges. The response is the same as the GET operation — a
summary of the current state of logging.

Example request body (enables the AUTH logger, disables TRACE):

```json
{
    "loggers": {
        "AUTH": true,
        "TRACE": false
    }
}
```

&nbsp;
&nbsp;

#### DELETE /admin/loggers/

Purges old log files from the server. Requires admin privileges.

Supports the `?keep=n` parameter to specify the number of most-recent log files
to retain. All older log files are deleted.

&nbsp;
&nbsp;

#### GET /services/admin/log

Returns the text of the server log file. If you specify that the REST call accepts
`application/json`, the log is returned as an array of strings each containing a
JSON-encoded log line. If you specify `text/plain`, the log file is localized on
the server side and returned as raw lines of text.

Supports the following URL parameters:

| Parameter | Description |
| :-------- | :---------- |
| tail | Return only the last `n` lines of the log. A value of zero returns all lines. |
| session | Return only log lines from the given session number. |

Example response with `?tail=3`:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 91103
    },
    "lines": [
        "{\"time\":\"2025-12-29 10:57:55\",\"id\":\"e0ca934f...\",\"seq\":482,\"session\":81,\"class\":\"server\",\"msg\":\"log.server.request\",\"args\":{...}}",
        "{\"time\":\"2025-12-29 10:57:55\",\"id\":\"e0ca934f...\",\"seq\":483,\"session\":82,\"class\":\"server\",\"msg\":\"log.server.request\",\"args\":{...}}",
        "{\"time\":\"2025-12-29 10:57:55\",\"id\":\"e0ca934f...\",\"seq\":484,\"session\":83,\"class\":\"server\",\"msg\":\"log.server.request\",\"args\":{...}}"
    ]
}
```

The `lines` array contains escaped strings, each holding a single JSON object
for one log line. The complete set of fields in a log message object:

| Field Name | Description |
| ---------- | ----------- |
| time | The timestamp of when the log entry was generated |
| id | The UUID of the server that generated the message |
| seq | An integer sequence number that orders messages for a given id |
| session | The REST session number that generated this log message |
| class | The Ego logging message class ("server", "rest", "auth", etc.) |
| msg | A text string uniquely identifying each message |
| args | An object with named arguments that accompany the message |

&nbsp;
&nbsp;

### Users <a name="users"></a>

The users interface allows an administrative user to create and delete user
credentials, set user passwords, and update the permissions list for a given
user. All user endpoints require admin privileges.

&nbsp;
&nbsp;

#### GET /admin/users/

Returns the list of users in the credentials store. The result is a JSON
structure with the following fields:

| Field | Description |
| :----- | :---------- |
| server | The server information object for this response |
| start | The starting offset of this result set (always zero) |
| count | The number of items returned |
| items | An array of user objects |

Each user object in the `items` array has the following fields:

| Field | Description |
| :---- | :---------- |
| name | The name of the user |
| id | A unique UUID for the user |
| permissions | An array of strings containing permission names |

Example response:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 155
    },
    "start": 0,
    "count": 2,
    "items": [
        {
            "name": "admin",
            "id": "0b77ac93-44b3-4f43-b1d3-9fa0dc7a4039",
            "permissions": [
                "ego.root",
                "ego.logon"
            ]
        },
        {
            "name": "iphoneUser",
            "id": "360565a1-f038-4478-88f3-abd9cc38d47f",
            "permissions": [
                "ego.logon",
                "ego.table.admin"
            ]
        }
    ]
}
```

In this example, the user "admin" has the `ego.root` privilege, which makes them
an administrative user.

&nbsp;
&nbsp;

#### GET /admin/users/_name_

Returns the user record for a single named user. The response payload is the
same structure as a single element from the `items` array described above.

&nbsp;
&nbsp;

#### POST /admin/users/

Creates a new user in the credentials store. The request body must be a JSON
object with the following fields:

| Field | Description |
| :---- | :---------- |
| name | The username to create |
| password | The initial password for the user |
| permissions | An optional array of permission name strings to assign |

&nbsp;
&nbsp;

#### PATCH /admin/users/_name_

Updates an existing user. The request body is a JSON object containing the
fields to update. Only fields present in the payload are changed; omitted
fields retain their current values. You can update the password, permissions
list, or both.

&nbsp;
&nbsp;

#### DELETE /admin/users/_name_

Deletes the named user from the credentials store.

&nbsp;
&nbsp;

### Assets <a name="assets"></a>

The _Ego_ server has the ability to serve up arbitrary file contents to a REST
caller. These are referred to as "assets" and are typically things like image
files, JavaScript payloads, etc. that are created by the administrator of an
_Ego_ web server to support services written in _Ego_.

#### GET /assets/_path_

Reads an asset from the disk in the library part of the _Ego_ path. The root of
this location is typically _EGO_PATH_/lib/services/assets/ followed by the path
expressed in the REST URL call. You can only GET items; you cannot modify them or
list them.

When an asset is read it is also cached in memory (see the documentation on
[caching](#caches) for more information).

```html
<!-- The asset must have a root path of /assets to be located properly -->
<img src="/assets/logo.png" alt="Ego logo" style="width:300px;height:150px;">
```

&nbsp;
&nbsp;

### Configuration <a name="config"></a>

#### GET /admin/config

Returns all current server configuration values. Requires admin privileges.
The response is a JSON object whose fields are the configuration key names and
whose values are the current settings.

#### POST /admin/config

Returns specific configuration values. Requires admin privileges. The request
body is a JSON array of configuration key name strings; the response contains
only those keys.

&nbsp;
&nbsp;

### Memory <a name="memory"></a>

#### GET /admin/memory

Returns the current memory usage statistics for the server process. Requires
admin privileges and the `server.admin` permission.

The response is a JSON object describing current allocated memory, total
allocated, system memory, and garbage collection count.

&nbsp;
&nbsp;

### Run Code <a name="run"></a>

#### POST /admin/run

Compiles and executes a snippet of Ego code submitted from the dashboard Code
editor tab. Requires admin privileges and both the `server.admin` and `code.run`
permissions.

The request body is a JSON object containing the Ego source code to run. The
response contains the output produced by the code.

&nbsp;
&nbsp;

### Validation Dictionary <a name="validation"></a>

#### GET /admin/validation/

Returns the server's request validation dictionary, which describes the expected
parameters and media types for each registered endpoint. Requires admin
privileges and the `server.admin` permission.

Supports the following mutually exclusive URL parameters:

| Parameter | Description |
| :-------- | :---------- |
| method | Filter results to entries for the given HTTP method |
| path | Filter results to entries for the given path pattern |
| entry | Return a single named validation entry |

&nbsp;
&nbsp;

### Token Revocation List <a name="tokens"></a>

The _Ego_ server maintains a revocation list of bearer tokens that have been
explicitly invalidated before their natural expiration. All token endpoints
require admin privileges and the `server.admin` permission.

#### GET /admin/tokens/

Returns the list of currently revoked token IDs.

#### PUT /admin/tokens/

Adds a token ID to the revocation list (revokes the token). The request body
must contain the token ID to revoke.

#### DELETE /admin/tokens/_id_

Removes a single token ID from the revocation list, reinstating the token.

#### DELETE /admin/tokens/

Flushes the entire token revocation list.

&nbsp;
&nbsp;

### Server Shutdown <a name="down"></a>

#### POST /services/admin/down/

Initiates a graceful server shutdown. Requires admin authentication. The server
will stop accepting new connections and exit cleanly after completing any
in-progress requests.

&nbsp;
&nbsp;

## Data Sources and Tables <a name="dsn"></a>

The _Ego_ server provides a REST API for communicating with databases configured
as Data Source Names (DSNs). These APIs are used to manage database tables and to
read, write, update, and delete rows.

All table data access is scoped under a DSN. The URL path is `/dsns/` followed by
the DSN name, followed by `/tables/`. For example, to read the rows of a table
named `foo` in a DSN named `bar`, the endpoint is:

```http
GET /dsns/bar/tables/foo/rows
```

The DSN endpoints also support transaction control (begin, commit, rollback) and
an admin-only raw SQL execution endpoint.

&nbsp;
&nbsp;

### Managing Data Source Names

All DSN management endpoints require admin privileges and the `dsn.admin`
permission.

#### GET /dsns/

Lists the available data source names. For each DSN, the type of database is
given along with any information needed to construct a valid connection string.
Passwords, if provided, are not returned.

Supports the `?start=` and `?limit=` parameters to page through long lists.

#### POST /dsns/

Creates a new data source name. The DSN definition is provided in the request
body as a JSON object. If a password is provided it will be encrypted in the
back-end store.

#### GET /dsns/_dsn_/

Retrieves the stored definition for the named DSN.

#### DELETE /dsns/_dsn_/

Deletes the named DSN and its configuration.

#### POST /dsns/@permissions

Grants or revokes permissions on a DSN. The JSON payload defines one or more
grant or revoke operations for specified users.

#### GET /dsns/_dsn_/@permissions

Lists the current permissions for the named DSN.

&nbsp;
&nbsp;

### Table Operations <a name="tablesAPI"></a>

This section covers APIs to:

* [List existing tables](#listTables)
* [Create a new table](#createTable)
* [Show the column names and types for a table](#metadata)
* [Delete an entire table](#deleteTable)
* [Execute arbitrary SQL statements on the server](#sql)
* [Execute multiple row operations as a transaction](#tx)
* [Data chaining in transactions](#chaining)

All table operations return either a rowset or a rowcount response. A rowset
contains an array of structure definitions where each column in the row is a
field name and the value of the column in that row is the field value. A rowcount
contains a struct with a field `count` which is the number of rows affected by
the operation.

Rowsets and rowcounts also include a `status` field (normally 200) and, on error,
a `msg` field containing the error text.

It is recommended that you read the [Rows API](#rows) documentation before
attempting to use the transaction function, as each task in a transaction
corresponds to an individual row operation.

&nbsp;
&nbsp;

#### GET /dsns/_dsn_/tables/ <a name="listTables"></a>

Returns a list of the tables in the named DSN. The current user must have at
least read access. The response is a JSON payload containing an array of table
descriptor objects.

Supports the following URL parameters:

| Parameter | Description |
| :-------- | :---------- |
| limit | Return at most this many tables |
| start | Specify the first table of the result set (1-based) |
| rowcounts | When `false`, skip counting rows (default `true`) |
| user | Return tables owned by this user (admin only) |

The `rowcounts` parameter defaults to `true`; the tables list will include the
number of rows in each table. For very large tables this may be a performance
concern, so callers can set `rowcounts=false` to skip row counting.

The response has the following fields:

| Field | Type | Description |
| :---- | :--- | :---------- |
| count | int | The number of tables returned |
| tables | array | An array of table descriptor objects |

Each table object has:

| Field | Type | Description |
| :---- | :--- | :---------- |
| name | string | The name of the table |
| schema | string | The username for the database schema |
| columns | int | Count of columns in the table |
| rows | int | Count of rows in the table |

Example response:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 44622
    },
    "tables": [
        { "name": "Accounts", "schema": "smith", "columns": 2, "rows": 8 },
        { "name": "simple",   "schema": "smith", "columns": 2, "rows": 1053 },
        { "name": "test5",    "schema": "smith", "columns": 1, "rows": 23 }
    ],
    "count": 3
}
```

&nbsp;
&nbsp;

#### PUT /dsns/_dsn_/tables/_table_ <a name="createTable"></a>

Creates a new table. The payload must be a JSON object with a `columns` array.
Each element in the array has a `name` and `type` field. If you do not specify
a column named `_row_id_`, one is added automatically; it contains a UUID that
uniquely identifies each row.

Valid column types:

| Type | Description |
| :------ | :---------- |
| string | Varying-length character string |
| int | Integer value |
| float32 | Single-precision floating point |
| float64 | Double-precision floating point |
| bool | Boolean value (`true` or `false`) |

Optional column attributes:

| Attribute | Description |
| :-------- | :---------- |
| nullable | When `true`, the column may contain a null value |
| unique | When `true`, the column must contain unique values |

Example request payload:

```json
{
    "columns": [
        { "name": "first", "type": "string", "nullable": true },
        { "name": "id",    "type": "int",    "unique": true },
        { "name": "last",  "type": "string" }
    ]
}
```

&nbsp;
&nbsp;

#### GET /dsns/_dsn_/tables/_table_ <a name="metadata"></a>

Returns the column metadata for the named table. The response is a JSON payload
containing an array of structures, each of which defines the column name, type,
size, and nullability.

&nbsp;
&nbsp;

#### DELETE /dsns/_dsn_/tables/_table_ <a name="deleteTable"></a>

Deletes the named table and all its contents from the database. The current user
must have `table.update` permission for the table.

&nbsp;
&nbsp;

#### PUT /dsns/_dsn_/tables/@sql <a name="sql"></a>

Executes an arbitrary SQL statement. The current user must have admin privileges.
The SQL text to execute must be passed as a JSON-encoded string in the body of
the request. The reply will be either a rowset (for `SELECT` statements) or a
rowcount (for any other statement).

For example, this request payload joins two tables:

```json
"select people.name, surname.name from \"mary\".\"people\" join \"mary\".\"surname\" on people.id == surname.id"
```

You can execute multiple statements in a single operation by formatting the
payload as an array of strings:

```json
[
    "delete from people where name='Jones'",
    "insert into people(name) values ('Smith')"
]
```

All statements in the array are executed as a single transaction. If any
statement fails, none take effect. If a `SELECT` is included it must be the
last statement in the array.

&nbsp;
&nbsp;

#### POST /dsns/_dsn_/tables/@transaction <a name="tx"></a>

Executes an atomic list of operations — all must succeed for any change to be
committed. This is the primary mechanism for creating multi-step transactions that
span multiple tables.

The following task operations can be performed as part of a transaction:

| Operation | Description |
| :-------- | :---------- |
| delete | Delete rows from a table. Specify filters to indicate which rows. |
| insert | Insert a row into a table. Specify values for each column. |
| readrows | Read multiple rows from a table; return as the transaction result. |
| select | Read a single row from a table; set symbol values for chaining. |
| symbols | Set symbol values for this transaction. |
| sql | Execute an arbitrary native SQL statement. |
| update | Update one or more rows with new values. Specify filters to indicate which rows. |

The payload is an array of task objects. Each task has the following members:

| Task Item | Description |
| :---------- | :----------- |
| operation | The operation to perform for this task. |
| table | The table on which to perform the operation. Not used for `sql` or `readrows`. |
| filters | An array of filter specification strings (ANDed together). |
| columns | An array of column names to use for this operation. |
| emptyError | When `true`, the transaction fails if the step finds or modifies no rows. |
| data | A row object: field names are column names, field values are column values. |
| sql | Native SQL string, used for `readrows` and `sql` operations only. |

The `emptyError` flag is only relevant for `select`, `delete`, and `update`
operations. By default, a task that affects no rows allows the transaction to
continue. When `emptyError` is `true`, an empty result causes the transaction
to stop with an HTTP 404 error.

Example payload with two inserts and an update:

```json
[
    {
        "operation": "insert",
        "table": "x6",
        "data": { "first": "Elmer", "last": "Fudd", "role": "tester" }
    },
    {
        "operation": "insert",
        "table": "x6",
        "data": { "first": "Daffy", "last": "Duck", "role": "tester" }
    },
    {
        "operation": "update",
        "table": "x6",
        "filters": [ "EQ(role,'tester')" ],
        "columns": [ "address" ],
        "emptyError": true,
        "data": { "address": "666 Scary Drive" }
    }
]
```

A successful transaction returns a rowcount object with a `count` field
containing the total number of rows affected across all tasks.

The `?filter=` URL parameter can be used to supply an additional filter applied
to all row-level operations in the transaction.

&nbsp;
&nbsp;

#### Data Chaining in Transactions <a name="chaining"></a>

Two task types support passing values between steps:

| Operation | Description |
| :-------- | :---------- |
| select | Reads a single row from a table and stores its column values in the substitution dictionary. |
| symbols | Explicitly sets named values in the substitution dictionary. |

There is a _substitution dictionary_ for each transaction REST API call — a set
of key/value pairs that persist across all tasks in a single call. A name
enclosed in double braces (`{{name}}`) in a filter, column list, or data value
is replaced at run time with the corresponding dictionary value.

For example, a read of a customer UUID can be used as the filter in a subsequent
update:

```json
[
    {
        "operation": "select",
        "table": "table1",
        "columns": [ "customer" ],
        "filters": [ "EQ(key,1)" ]
    },
    {
        "operation": "insert",
        "table": "table2",
        "data": {
            "sender": "toby",
            "recipient": "{{customer}}"
        }
    }
]
```

The `select` operation always reads at most one row. When `columns` is omitted,
all columns from the retrieved row are stored as symbols.

When you need to read the same table twice and preserve both results, use the
`symbols` operation to rename the value between reads:

```json
[
    {
        "operation": "select",
        "table": "table1",
        "columns": [ "customer" ],
        "filters": [ "EQ(key,1)" ]
    },
    {
        "operation": "symbols",
        "data": { "sending_customer": "{{customer}}" }
    },
    {
        "operation": "select",
        "table": "table1",
        "columns": [ "customer" ],
        "filters": [ "EQ(key,2)" ]
    },
    {
        "operation": "symbols",
        "data": { "receiving_customer": "{{customer}}" }
    },
    {
        "operation": "insert",
        "table": "table2",
        "data": {
            "sender": "{{sending_customer}}",
            "recipient": "{{receiving_customer}}"
        }
    }
]
```

&nbsp;
&nbsp;

### Transaction Control

The transaction control endpoints allow a client to explicitly manage a database
transaction that spans multiple individual REST calls.

#### GET /dsns/_dsn_/begin

Starts a new transaction on the named DSN. Returns a transaction ID that must
be passed to subsequent requests as the `?transaction=` parameter.

Supports `?expires=<duration>` (e.g. `?expires=5m`) to set a timeout after
which the transaction is automatically rolled back.

#### GET /dsns/_dsn_/commit

Commits the transaction identified by `?transaction=<id>`.

#### GET /dsns/_dsn_/rollback

Rolls back the transaction identified by `?transaction=<id>`.

&nbsp;
&nbsp;

### Rows API <a name="rows"></a>

This section covers API functions to:

* [Read rows from a table](#readrows)
* [Insert new rows into a table](#insertRows)
* [Update existing rows in a table](#updateRows)
* [Delete rows from a table](#deleteRows)

The row data APIs use the `/dsns/_dsn_/tables/_table_/rows` endpoint. The result
is either a rowset (for GET) or a rowcount indicating how many rows were affected
(for PUT, PATCH, and DELETE).

&nbsp;
&nbsp;

#### GET /dsns/_dsn_/tables/_table_/rows <a name="readrows"></a>

Reads rows from a table. If no parameters are given, all columns of all rows are
returned in an unpredictable order.

Supported URL parameters:

| Parameter | Example | Description |
| :-------- | :-------- | :---------- |
| columns | ?columns=id,name | Columns to return (default: all) |
| filter | ?filter=EQ(name,"TOM") | Only return rows matching the filter |
| limit | ?limit=10 | Return at most this many rows |
| sort | ?sort=id | Sort the result by the named column(s) |
| start | ?start=100 | First row of the result set (1-based) |
| abstract | ?abstract=true | Return results in the abstract row format |
| user | ?user=smith | Return rows belonging to the given user (admin only) |

You can prefix a column name in the `sort` parameter with `~` to sort in
descending order.

**Filter expressions** consist of an operator followed by one or two operands
in parentheses. Operands can themselves be filter expressions, allowing
arbitrarily complex expressions.

| Operator | Example | Description |
| :------- | :------- | :---------- |
| EQ | EQ(id,101) | Column equals value |
| LT | LT(age,65) | Column is less than value |
| LE | LE(size,12) | Column is less than or equal to value |
| GT | GT(age,65) | Column is greater than value |
| GE | GE(size,12) | Column is greater than or equal to value |
| AND | AND(EQ(id,1),EQ(id,2)) | Both sub-expressions must be true |
| OR | OR(EQ(id,1),EQ(id,2)) | Either sub-expression must be true |
| NOT | NOT(EQ(id,101)) | Sub-expression must be false |
| HAS | HAS(foo,'YES','NO') | String column contains any of the given substrings |
| HASALL | HASALL(foo,'YES','NO') | String column contains all of the given substrings |

AND() and OR() accept two or more sub-expressions. HAS() and HASALL() are
case-sensitive. String values may be specified in double or single quotes;
floating-point values are written as e.g. `123.45`.

The response is a rowset object:

| Field | Description |
| :---- | :---------- |
| rows | An array of row objects (field names = column names) |
| count | How many items are in the rows array |

Example response:

```json
{
    "server": {
        "api": 1,
        "name": "appserver.abc.com",
        "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
        "session": 1525
    },
    "rows": [
        { "Name": "Tom",  "Number": 101, "_row_id_": "76d3e219-1015-49c8-9e77-decb750ad13e" },
        { "Name": "Mary", "Number": 102, "_row_id_": "a974019e-f9e7-4554-adb4-2004b6f65c03" }
    ],
    "count": 2
}
```

&nbsp;
&nbsp;

#### PUT /dsns/_dsn_/tables/_table_/rows <a name="insertRows"></a>

Inserts one or more new rows into the table. The request body must have the
`table.update` permission. You do not need to specify `_row_id_`; it is assigned
automatically.

The payload may be either a single row object or a rowset (an object with a
`rows` array):

Single row:

```json
{ "Name": "Susan", "Number": 103 }
```

Multiple rows:

```json
{
    "rows": [
        { "Name": "Susan", "Number": 103 },
        { "Name": "Timmy", "Number": 104 },
        { "Name": "Mike",  "Number": 105 }
    ],
    "count": 3
}
```

Supports the `?upsert` parameter for conditional insert-or-update behavior.
When `?upsert` is present with no value, the upsert key is the `_row_id_`
column. When `?upsert=col1,col2` is specified, those columns are used as the
match key. If a row with the same key already exists it is updated; otherwise
a new row is inserted.

Supports the `?abstract=true` and `?user=` parameters.

&nbsp;
&nbsp;

#### PATCH /dsns/_dsn_/tables/_table_/rows <a name="updateRows"></a>

Updates existing rows in the table. Only the values specified in the request body
are updated; other values are left unchanged.

Supported URL parameters:

| Parameter | Example | Description |
| :-------- | :------- | :---------- |
| filter | ?filter=EQ(name,"TOM") | Select which rows to update |
| columns | ?columns=Id,Name | Only update these columns from the payload |

If a `_row_id_` field is present in the row payload, only that specific row is
updated. Otherwise, all rows matching the `filter` are updated.

A common pattern is to GET the rows you want to update (which returns `_row_id_`
values), modify the desired fields, then PATCH the modified rowset back.

A successful response is a rowcount object with a `count` field indicating how
many rows were changed.

Example request and URL:

```text
PATCH /dsns/bar/tables/Accounts/rows?filter=EQ(Number,101)
```

```json
{ "Name": "Bob" }
```

&nbsp;
&nbsp;

#### DELETE /dsns/_dsn_/tables/_table_/rows <a name="deleteRows"></a>

Deletes rows from a table. By default all rows are deleted. Use the `filter`
parameter to select which rows to delete.

| Parameter | Example | Description |
| :-------- | :------- | :---------- |
| filter | ?filter=EQ(name,"TOM") | Only delete rows matching the filter |
| user | ?user=smith | Delete rows for the given user (admin only) |

A successful response is a rowcount object.

&nbsp;
&nbsp;

### Table Permissions <a name="permissions"></a>

A permissions table managed by the _Ego_ server controls whether a given user can
read, update, or delete a given table. By default, a user can only set
permissions on tables they own. An administrator (a user with `ego.root`
privilege) can change permissions on any table for any user.

&nbsp;
&nbsp;

#### GET /dsns/_dsn_/tables/@permissions <a name="allPermissions"></a>

Returns all permissions data for all tables in the DSN. Requires admin
privileges. The result is a JSON payload with an array of permission objects,
each listing the user, schema, table, and a string array of permission names.

Supports `?user=` to filter by user.

&nbsp;
&nbsp;

#### GET /dsns/_dsn_/tables/_table_/permissions <a name="tablePermissions"></a>

Returns the permissions object for the given table and the current user. Includes
the user, schema, table, and a string array of permission names.

Supports `?user=` to query permissions for a specific user (admin only).

&nbsp;
&nbsp;

#### PUT /dsns/_dsn_/tables/_table_/permissions <a name="setPermissions"></a>

Updates the permissions for the given table and the current user. The body must
contain a JSON array of permission name strings. A name prefixed with `+` adds
the permission; a name prefixed with `-` removes it. It is not an error to
remove a permission that does not exist. If no permissions record exists for the
user, a new one is created.

Supports `?user=` to update permissions for a specific user (admin only).

&nbsp;
&nbsp;

#### DELETE /dsns/_dsn_/tables/_table_/permissions

Revokes all permissions for the given table and the current user.

Supports `?user=` to revoke permissions for a specific user (admin only).

&nbsp;
&nbsp;

## Services <a name="services"></a>

All remaining REST endpoints are provided under the `/services` path. The server
loads, compiles, and runs an Ego program to respond to each such API request.
This is the mechanism by which the developer extends the server's functionality.

By convention, the _EGO_PATH_/services directory includes several subdirectories:

| Subdirectory | Description |
| :----------- | :---------- |
| admin | Services supporting administrative and debugging functions |
| assets | Static resources served via the `/assets` endpoint (images, etc.) |
| templates | Static template files (usually HTML) used by services |

### Dynamic Services

Any `.ego` file placed under the services directory tree becomes an endpoint. The
server maps the file path under the services root to an endpoint path. For
example, `lib/services/factor.ego` is available at `/services/factor`.

A service file uses the `@endpoint` directive to declare a custom endpoint path,
and `@authenticated` to require authentication. Query parameter types can be
declared with `?name=type` annotations. The server calls the `handler` function
in the service file, passing `http.Request` and `http.Response` objects.

Example service:

```go
import "http"

func mb(f float64) string {
    return fmt.Sprintf("%3.2fmb", f)
}

func handler(req http.Request, resp http.Response) {
    m := util.Memory()
    h := os.Hostname()

    pageData := {
        Allocated: mb(m.current),
        Total:     mb(m.total),
        System:    mb(m.system),
        GC:        m.gc,
        ID:        _instance,
        Date:      time.Now().Format(time.RFC1132),
        Host:      h,
    }

    resp.WriteTemplate("lib/services/templates/memory.html", pageData)
}
```

The `resp.WriteTemplate()` function reads an HTML template from disk, substitutes
values from the supplied data structure (using `{{.FieldName}}` syntax), and
writes the result as the HTTP response.

### Example Template File

The associated template uses Go template syntax for substitution operators.
A reference like `{{.Total}}` is replaced with the value of `pageData.Total`.

```html
<!DOCTYPE html>
<html>
    <head>
        <title>Ego Memory ({{.Host}})</title>
    </head>
    <body>
        <p>{{.Date}}</p>
        <table>
            <tr><td>Currently allocated</td><td>{{.Allocated}}</td></tr>
            <tr><td>Total allocated</td>    <td>{{.Total}}</td></tr>
            <tr><td>System memory</td>      <td>{{.System}}</td></tr>
            <tr><td>Garbage collections</td><td>{{.GC}}</td></tr>
        </table>
        <!-- Asset referenced from /assets path -->
        <img src="/assets/logo.png" alt="Ego logo" style="width:300px;height:150px;">
    </body>
</html>
```

Template substitution operators throughout the page show where the service data
structure values are injected into the HTML sent back to the caller. The `img`
tag causes the browser to make a second call to retrieve the image from the
`/assets` path.

&nbsp;
&nbsp;

## API Endpoint Summary

The following table lists every endpoint supported by the _Ego_ server.

| Method | Endpoint | Summary |
| :----- | :------- | :------ |
| POST | /services/admin/logon | Authenticate with username and password; returns a bearer token. |
| GET | /services/admin/authenticate | Returns identity and expiration information for the current bearer token. |
| POST | /services/admin/down/ | Initiates a graceful server shutdown. |
| GET | /services/admin/log | Returns lines from the server log file, optionally filtered by session or tail count. |
| GET | /admin/heartbeat | Lightweight liveness check; returns 200 if the server is running. |
| GET | /admin/config | Returns all current server configuration key/value pairs. |
| POST | /admin/config | Returns the current values for a specified set of configuration keys. |
| GET | /admin/memory | Returns current process memory usage statistics. |
| POST | /admin/run | Compiles and executes a snippet of Ego code submitted from the dashboard. |
| GET | /admin/validation/ | Returns the request validation dictionary, optionally filtered by method, path, or entry. |
| GET | /admin/caches | Returns the current service and asset cache status and item inventory. |
| POST | /admin/caches | Sets the maximum size of the service cache or asset cache. |
| DELETE | /admin/caches | Flushes all items from the service and/or asset cache. |
| GET | /admin/loggers/ | Returns the enabled/disabled state of all server loggers. |
| POST | /admin/loggers/ | Enables or disables one or more named server loggers. |
| DELETE | /admin/loggers/ | Purges old server log files, keeping a specified number of the most recent. |
| GET | /admin/users/ | Lists all users in the credentials store. |
| GET | /admin/users/_name_ | Returns the credential record for a single named user. |
| POST | /admin/users/ | Creates a new user in the credentials store. |
| PATCH | /admin/users/_name_ | Updates the password and/or permissions for an existing user. |
| DELETE | /admin/users/_name_ | Deletes a user from the credentials store. |
| GET | /admin/tokens/ | Returns the list of currently revoked bearer token IDs. |
| PUT | /admin/tokens/ | Adds a bearer token ID to the revocation list. |
| DELETE | /admin/tokens/_id_ | Removes a single token ID from the revocation list. |
| DELETE | /admin/tokens/ | Flushes the entire token revocation list. |
| GET | /assets/_path_ | Serves a static asset file from the server's assets directory. |
| GET | /ui | Serves the web-based administration dashboard. |
| GET | /dsns/ | Lists all configured data source names. |
| POST | /dsns/ | Creates a new data source name. |
| GET | /dsns/_dsn_/ | Returns the configuration for the named DSN. |
| DELETE | /dsns/_dsn_/ | Deletes the named DSN. |
| POST | /dsns/@permissions | Grants or revokes permissions on a DSN for one or more users. |
| GET | /dsns/_dsn_/@permissions | Lists the current permissions for the named DSN. |
| GET | /dsns/_dsn_/begin | Starts a new explicit database transaction; returns a transaction ID. |
| GET | /dsns/_dsn_/commit | Commits the transaction identified by the `transaction` parameter. |
| GET | /dsns/_dsn_/rollback | Rolls back the transaction identified by the `transaction` parameter. |
| GET | /dsns/_dsn_/tables/ | Lists all tables available in the named DSN. |
| PUT | /dsns/_dsn_/tables/_table_ | Creates a new table with the specified column definitions. |
| GET | /dsns/_dsn_/tables/_table_ | Returns the column metadata (name, type, nullability) for the named table. |
| DELETE | /dsns/_dsn_/tables/_table_ | Deletes the named table and all its data. |
| PUT | /dsns/_dsn_/tables/@sql | Executes arbitrary SQL against the DSN (admin only). |
| POST | /dsns/_dsn_/tables/@transaction | Executes an atomic multi-step transaction across one or more tables. |
| GET | /dsns/_dsn_/tables/@permissions | Returns permissions records for all tables in the DSN (admin only). |
| GET | /dsns/_dsn_/tables/_table_/rows | Reads rows from a table, with optional filtering, sorting, and pagination. |
| PUT | /dsns/_dsn_/tables/_table_/rows | Inserts one or more new rows into a table, with optional upsert behavior. |
| PATCH | /dsns/_dsn_/tables/_table_/rows | Updates existing rows in a table matching a filter expression. |
| DELETE | /dsns/_dsn_/tables/_table_/rows | Deletes rows from a table matching a filter expression. |
| GET | /dsns/_dsn_/tables/_table_/permissions | Returns the permissions record for the named table and current user. |
| PUT | /dsns/_dsn_/tables/_table_/permissions | Grants or revokes permissions on the named table for a user. |
| DELETE | /dsns/_dsn_/tables/_table_/permissions | Revokes all permissions on the named table for a user. |
| ANY | /services/_path_ | Loads, compiles, and runs the Ego service file mapped to the given path. |
