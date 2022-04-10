     ____    _____   ____    _____        _      ____    ___
    |  _ \  | ____| / ___|  |_   _|      / \    |  _ \  |_ _|  ___
    | |_) | |  _|   \___ \    | |       / _ \   | |_) |  | |  / __|
    |  _ <  | |___   ___) |   | |      / ___ \  |  __/   | |  \__ \
    |_| \_\ |_____| |____/    |_|     /_/   \_\ |_|     |___| |___/


# Table of Contents

1. [Introduction](#intro)

2. [Authentication](#auth)

3. [Administration](#admin)

4. [Tables](#tables)

5. [Services](#services)

&nbsp;
&nbsp;

# Introduction <a name="intro"></a>
This document describes the REST Application Programming Interfaces (APIs) 
supported by an _Ego_ server instance. This covers how to authenticate to the 
server, administration functions that can be performed by a suitably 
privileged user, APIs for directly accessing database tables and their data, 
and APIs for accessing user-written services (implemented as _Ego_ programs)

All REST API responses should return a server information object as a field in
the payload named "server". This contains the following informmation:

| Field     | Description |
| :-------- |:----------- |
| api       | An integer indicating the API version. Currently always 1. |
| name      | The short host name of the machine running the server. |
| id        | The UUID of this instance of the server. |
| session   | The session correlator for the server (matches request log entries). |

The server name can be used to locate where the log files are found. The server id
helps identify when a server is restarted, as it is assigned a new UUID each time 
and this information is in the server log. Finally, each logging entry from a
REST client session includes a session number. This integer value can be used to
find the specific log entries for this request.

&nbsp;
&nbsp;

# Authentication <a name="auth"></a>
Other than determining if a server is running or not, all operations performed
against an Ego server must be done with an authenticated user. The normal pattern
for this is for the client to "log in" the user, and receive an encrypted token
in return. This token can be presented to any other API to authenticate the user
for the functions of that API.

To authenticate, you can use a "GET" or a "POST method to the endpoint "/services/admin/logon". 

## GET /services/admin/logon
Use GET when you wish to use Basic authentication in the HTTP header to send the
username and password for logon. The request must:

* Use BASIC authentication to pass the username and password to the service. 
* Specify that the reply type accepted is "application/json"

The rest status will either be a status of 403 indicating that the credentials 
are invalid, or 200 indicating that the credentials were valid.

The response payload is a JSON object with the resulting secure token if 
the credntials were valid. The credentials must match the username and 
password stored in the _Ego_ server credentials data store; you can use 
`ego` CLI commands to view and modify this store, as well as `/admin` 
endpoints described below to read, update, or delete entries in
the credentials store.

The resulting JSON payload is an object with the following fields:

| Field     | Description |
| :-------- |:----------- |
| server    | The server information object for this response. |
| expires   | A string containing the timestamp of when the token expires |
| token     | A variable-length string containing the token text itself. |
| identity  | The username encoded within the token. |

Here is an example response payload:
&nbsp;

    {
      "server": {
          "api": 1,
          "name": "appserver.abc.com",
          "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
          "session": 151
      },
      "expires": "Fri Jan 21 13:12:25 EST 2022",
      "identity": "joesmith",
      "token": "220de9776c7c517f84c1d4b94aadcb6e50849abed4eb6b26b9d16e3365e3a014b5fdefac5b107"
    }

&nbsp;

It is the responsibility of the application to extract the `token` field from
the resulting payload and store it away to use for subsequent REST API 
operations. When using this token, it should be used as a Bearer token in 
subsequent REST operations as the `Authentication: Bearer` header.

A less secure mechanism can be used to authenticate, by providing username
and password credentials using the `Authentication: Basic` header, followed
by a Base64 encoding of the "username:password" string. This can be used for
initial development and debugging, but an authenticated token is the preferred
way to interact with the table services. In the future, the `Basic` authentication
support may be removed, requiring a `Bearer` token authentication.

&nbsp;
&nbsp;

## POST /services/admin/logon
Alternatively, if you do not want to use an authentication header in this initial
communication, you can use a "POST" method to the same endpoint with a JSON
payload with an object containing two field. For TLS/SSL-based communication, this
embeds the credentials inside the encrypted body of the request which can be more
secure. The request body must contain the following two fields:

&nbsp;

| Field     | Description |
| :-------- |:----------- |
| username   | A string containing username of the credentials |
| password   | A string containing password of the credentials |

Here is an example request payload for the logon operation, with a string for 
the username and a string for the password:

    {
        "username": "joesmith",
        "password": "3h97q-k35Z5"
    }
    

&nbsp;

The REST call will result in either be a status of 403 indicating that 
the credentials are invalid, or 200 indicating that the credentials were 
valid.

The response payload is a JSON object with the resulting secure token if 
the credntials were valid. The credentials must match the username and 
password stored in the _Ego_ server credentials data store; you can use 
`ego` CLI commands to view and modify this store, as well as `/admin` 
endpoints described below to read, update, or delete entries in
the credentials store.

The resulting JSON payload is an object with the following fields:

| Field     | Description |
| :-------- |:----------- |
| server    | The server information object for this response. |
| expires   | A string containing the timestamp of when the token expires. |
| token     | A variable-length string containing the token text itself. |
| identity  | The username encoded within the token. |

Here is an example response payload:
&nbsp;

    {
      "server": {
         "api": 1,
         "name": payroll,
         "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
         "session": 482
      },
      "expires": "Fri Jan 21 13:12:25 EST 2022",
      "identity": "joesmith",
      "token": "220de9776c7c517f84c1d4b94aadcb6e50849abed4eb6b26b9d16e3365e3a014b5fdefac5b107"
    }

&nbsp;

It is the responsibility of the application to extract the `token` field from
the resulting payload and store it away to use for subsequent REST API 
operations. When using this token, it should be used as a Bearer token in 
subsequent REST operations as the `Authentication: Bearer` header.

A less secure mechanism can be used to authenticate, by providing username
and password credentials using the `Authentication: Basic` header, followed
by a Base64 encoding of the "username:password" string. This can be used for
initial development and debugging, but an authenticated token is the preferred
way to interact with the table services. In the future, the `Basic` authentication
support may be removed, requiring a `Bearer` token authentication.

&nbsp;
&nbsp;


# Administrative Functions <a name="admin"></a>
Administrative functions are REST APIS used to support managing the REST server, including the
status and state of the server, the database of valid user credentials and permissions, and support
for caching and logging functions on the server.

* [View, flush, or set size of runtime caches](#caches)
* [Check if server is active/responding](#hearbeat)
* [View or configure logging classes on the server](#loggers)
* [Manage user credentials and permissions](#users)
* [Access HTML assets (images, etc.) used in HTML pages](#assets)

&nbsp;
&nbsp;

## Heartbeat <a name="heartbeat"></a>
The `heartbeat` endpoint is the simplest and fasted way to determine if an Ego server
is running and responding to requests. It does not require authentication of any kind,
and returns a 200 success code if the server is available. Any other return code
indicates that the server is not running or there is a network/gateway problem between
the REST client code and the Ego server.

This endpoint only supports the GET method, and returns no response body.
&nbsp;
&nbsp;

## Caches <a name="caches"></a>
The _Ego_ server maintains caches to make repeated use of the server more efficient
by saving operations in memory instead of having to re-load and re-compile services,
reload assets, etc.

You can examine what is in the server cache, direct the server to flush (i.e. remove
from memory) any cached items, and you can set the size of the services cache using
REST calls to the `admin/caches` endpoint.

When a service is invoked by a user to execute _Ego_ code written by the developer(s)
running the _Ego_ server, the server first searches the cache to see if it has already
executed this code before. When this is the case, the server does not have to reload
the service source code from disk or compile it again. Instead, it uses the previous
results of the compile to execute the service again on behalf of the client.

When the service cannot be found in the cache, it is loaded from disk and compiled
before it can be executed. The service just compiled is placed in the cache. If the
cache is too large (based on the limit on the number of items the server is configured
to allow) the oldest (least recently used) item in the cache is discarded before 
storing the newly-compiled service in the cache. For example, if the cache limit is
set to 10, then the cache will contain the ten most-recently used services. The
premise is that the cache should be set large enough to hold the most commonly used
services, so they are available for most users most of the time without recompiling.

Similarly, an "asset" cache is managed by the server. When a REST call is made to 
the server to the `/asset` endpoint, the remainder of the path represents the location
in the _Ego_ server's disk storage where assets are found. When a request is made for
an item, the server first checks to see if it is in memory already, and if so will
just return the contents of the cached item. If it was not found, then the item is
loaded from disk and also stored in the cache. The assets are typically image
files or similar HTML assets that might be used by a browser-based application.
As such, the asset cache is limited by the number of bytes of storaget that it can
consume in memory, regardless of the number of assets. By default, the cache is
one megabyte in size.

Note that a side effect of having a non-zero asset cache size is that if a server
has already cached a value in memory, then changing the disk copy will not result
in the updated information being used as the cached value. For example, if an HTML
web page requests an image from the `/assets` path, and then after that the
developer changes the image on disk using an image editing program, subsequent
calls to the server will still return the old copy. You must flush the server
cache to cause it to discard all the asset storage and start again reading from
disk to satisfy asset requests.

&nbsp;
&nbsp;
### GET /admin/caches
This gets information about the caching status in the server. This API requires that
the user have "admin" privileges. The result is a JSON payload with the following
fields:
&nbsp;

| Field     | Description |
|:--------- |:----------- |
| server    | The server information object for this response |
| host      | A string containing the name of the computer running the _Ego_ server |
| id        | A string containing the UUID of the server instance |
| count     | The number of items in the services cache |
| limit     | The maximum number of items in the services cache |
| items     | An array of strings with the names of the cached services |
| assets    | The number of assets stored in the in-memory cached |
| assetSize | The maximum size in bytes of the asset cache |

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### DELETE /admin/caches
The `DELETE` REST method tells the _Ego_ server to flush the caches; i.e. all the
copies of service compilations and asset objects are deleted from memory. Subsequent
REST calls will require that the server reload the item(s) from the disk store and
also then store them in the cache for future use.

You must have "admin" privileges to execute this REST call.
&nbsp;
&nbsp;
### PUT /admin/caches
You can set the size of the caches using the `PUT` method. The JSON payload for
this operation is a structure with one or both of the following fields:

| Field     | Description |
|:--------- |:----------- |
| limit     | The maximum number of items in the services cache |
| assetSize | The maximum size in bytes of the asset cache |

&nbsp;
&nbsp;

You must be an "admin" user to execute this call.
&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

## Loggers <a name="loggers"></a>
You can use the loggers endpoint to get information about the current state of logging on the
server, enable or disable specific loggers, and retrive the text of the log.
&nbsp;
&nbsp;

### GET /admin/loggers
This retrieves the current state of logging on the server. The response is a JSON payload
that indicates the host name where the server is running, it's unique instance UUID, the
name of the text file on the server where the log is being written, and an structure 
that indicates if each logger is enabled or disabled.

This service requires authentication with credentials for a user with administrative
privileges.

Here is an example response payload from this request:
&nbsp;

    {
        "server": {
            "api": 1,
            "name": "appserver.abc.com",
            "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
            "session": 6385
        },
        "file": "/Users/tom/ego/ego-server_2022-01-20-000000.log",
        "loggers": {
            "APP": false,
            "AUTH": true,
            "BYTECODE": false,
            "CLI": false,
            "COMPILER": false,
            "DB": false,
            "DEBUG": false,
            "INFO": false,
            "REST": false,
            "SERVER": true,
            "SYMBOLS": false,
            "TABLES": true,
            "TRACE": false,
            "USER": false
        }
    }

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### GET /services/admin/log

This path will return the text of the log file itself. If you specify that the REST call
accepts JSON, it will be returned as an array of strings. If you specify that it accepts
TEXT, the text is returned as-is.

If you add the parameter `?tail=n` where `n` is a number of lines of text, the GET operation
will return the last lines from the log. If you specify a value of zero, then all
lines are returned, otherwise the result is limited to the last `n` lines of the log

Here is an example output with a `tail` value of 5:
&nbsp;

    {
      "server": {
          "api": 1,
          "name": "appserver.abc.com",
          "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
          "session": 91103
      },
      "lines": [
        "[2022-01-20 13:20:18] 155   SERVER : [8] enable info(7) logger",
        "[2022-01-20 13:20:39] 156   SERVER : Requests in last 60 seconds: admin(1)  service(6)  asset(4)  code(0)  heartbeat(4)  tables(8)",
        "[2022-01-20 13:22:38] 157   SERVER : Memory: Allocated(   0.452mb) Total(   9.563mb) System(  14.253mb) GC(6) ",
        "[2022-01-20 13:24:56] 158   SERVER : [9] GET /services/admin/log/ from [::1]:56303",
        "[2022-01-20 13:24:56] 164   AUTH   : [9] Auth using token 254c9d366d..., user admin, root privilege user"
      ]
    }

&nbsp;
&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### PUT /admin/loggers
This call is used to modify the state of logging on the server. The payload must contain
the `loggers` structure that tells which loggers are to change state. Note that any logger
not mentioned in the payload does not have it's state changed.

This service requires authentication with credentials for a user with administrative
privileges. The response is the same as the GET operation; a summar of the current state
of logging.

Here is a sample request body, that enables the INFO logger and disables the TRACE logger.
Note that the names of the loggers are not case-sensitive.

&nbsp;

    {
    "loggers": {
        "INFO": true,
        "TRACE": false
      }
    }

&nbsp;
&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

## Users <a name="users"></a>
The users interface allows an administrative user to create and delete user credentials, set
user passwords, and update the permissions list for a given user.

&nbsp;
&nbsp;

### GET /admin/users/

This call returns the list of users that are in the credentials store. The result is a JSON
structure with the following fields:
&nbsp;

| Field  | Description |
|:------ |:----------- |
| server | The server information object for this response |
| start  | This value is always zero |
| count  | The number of items returned |
| items  | An array of user objects, described in the next table |

&nbsp;

The field "items" contains an array of user objects. The array of user objects
has the following fields:
&nbsp;

| Field | Description |
|:----- |:----------- |
| name | The name of the user |
| id   | A unique UUID for the user |
| permissions | an array of strings containing permissions names |

&nbsp;

Here is example output from a request to this endpoint:
&nbsp;

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
                    "root",
                    "logon"
                ]
            },
            {
                "name": "iphoneUser",
                "id": "360565a1-f038-4478-88f3-abd9cc38d47f",
                "permissions": [
                    "logon",
                    "table_create"
                ]
            }
        ]
    }

&nbsp;

In this example, there are two users defined. The user "admin" has the `root` privilege in their
list, which makes them an administrative user. The user "iphoneUser" has `logon` and `table_create`
privileges, which enable this user to connect to the server and have permission to create tables
using the /tables API discussed below.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

## Assets <a name="heartbeat"></a>
The _Ego_ server has the ability to serve up arbitrary file contents to a REST caller. These
are referred to as "assets" and are typically things like image files, javascript payloads, 
etc. that are created by the administrator of an instance of an _Ego_ web server, to support
services written in _Ego_.

The only supported method is "GET"

### GET /assets/_path_

The "GET" operation reads an asset from the disk in the library part of the _Ego_ path. The
root of this location is typically _EGO_PATH_/lib/services/assets/ followed by the path
expressed in the REST URL call. You can only GET items, you cannot modify them or list
them.

Note that when an asset is read, it is also cached in memory (see the documentation on
[caching](#caches) for more information). You can also see an example of an assset being
read in the service located at _EGO_PATH_/lib/services/templates/memory.html which references
an asset in HTML for a graphical image.

     <!-- The asset must have a root path of /assets to be located properly --> 
     <img src="/assets/logo.png" alt="Ego logo" style="width:300px;height:150px;">
     

&nbsp;
&nbsp; 
# Tables <a name="tables"> </a>

The _Ego_ server includes a REST API for communicating with a PostgreSQL database configured
for use by the server. The API can be used to manage database tables and read, write, update,
and delete rows from tables.

This API is divided into two sets,

* [Manipulating tables](#tablesapi)
* [Manipulating rows in a table](#rows)

&nbsp;
&nbsp;

## Table API <a name="tablesapi"></a>

This section covers APIs to:
* [List existing tables](#listtables)
* [Create a new table](#createtable)
* [Show the column names and types for a table](#metadata)
* [Delete an entire table](#deletetable)
* [Execute arbitrary SQL statements on the server](#sql)

All tables operations return either a rowset or a rowcount response. A rowset contains an array of
structure definitions where each column in the row is the field name, and the value of the column
in that row is the field value. There will be one object for each column in the requested table
query. A rowcount contains a struct withi a field called "count" which is the number of rows that
are affected by the operation performed. For update or delete operations, this is the number of 
rows that were updated or deleted. This value is zero for other operations (like deleting a table).

Finally, rowsets and rowcounts will also include a `status` field which is the HTTP status
of the operation, which is normally 200 for a successful operation. A value other than 200 means
something happened with the request that may not be the desired result, so an additional field
`message` contains the text of any error message genrated (for example, attempting to read a 
table column that doesn't exist, or not having permissions for the requested operation).

&nbsp;
&nbsp;
### GET /tables <a name="listtables"></a>

A GET call to the /tables endpoint will return a list of the tables. This is a JSON payload
containing an array of objects, each of which describes a table that the current user has
read access to.

Because the list of tables might be quite long, you can specify URL parameters that limit
the result set:

| Parameter | Example    | Description |
|:--------- |:---------- |:----------- |
| limit     | ?limit=10  | Return at most this many rows from the result set |
| start     | ?start=100 | Specify the first row of the result set (1-based) |
| rowcounts     | ?rowcounts=false | Do not return row counts in the result |

&nbsp;
&nbsp;

The `rowcounts` parameter defaults to `true`; the tables list operation will include the
number of rows in the table in the result set. However, for very large tables this may
become a performance problem, so the caller can request that the server not get the row
count. When `rowcounts` is set to false, then the row count is always zero.

The result of the call is an object with two fields, `count` and `tables`. The `count`
is the number of tables returned in this REST call. The `tables` are an array of table
objects, with the following fields:


| Parameter | Type  | Description |
|:--------- |:----- |:----------- |
| name     | string | The name of the table |
| schema   | string | The username for the database schema  |
| columns  | int    | Count of columns in the table |
| rows     | int    | Count of rows in the table |

&nbsp;
&nbsp;

Here is an example of the result data when the call is made by the user "smith", returning three
available tables of info:

&nbsp;

    {
      "server": {
          "api": 1,
          "name": "appserver.abc.com",
          "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
          "session": 44622
      },
      "tables": [
            {
                "name": "Accounts",
                "schema": "smith",
                "columns": 2,
                "rows": 8
            },
            {
                "name": "simple",
                "schema": "smith",
                "columns": 2,
                "rows": 1053
            },
            {
                "name": "test5",
                "schema": "smith",
                "columns": 1,
                "rows": 23
            }
        ],
        "count": 3
    }

&nbsp;
&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;


### PUT /tables/_table_  <a name="createtable"></a>

A PUT to a named table will create the table. The payload must be a JSON specification that
is an array of columns, each with a `name` and `type` field. The table is created using
the given columns. Additionally, if you do not specify a column with name `_row_id_`, then
a column of that name is added to the table definition. This column will contain a UUID
that uniquely identifies the row across all tables.

The valid types that you can specify in the array of column structure definitions are:

| Type    | Description |
|:------- |:----------- |
| string  | Varying length character string |
| int     | Integer value |
| float32 | Real floating point value |
| float64 | Double precision floating point value |
| bool    | Boolean value (can only be `true` or `false`)

&nbsp;


The request payload must be a JSON representation of the columns to be created. As an
example, this payload creates a table with three columns. 

    {
        "columns": [
            {
                "name": "first",
                "type": "string",
                "nullable": true
            },
            {
                "name": "id",
                "type": "int",
                "unique": true
            },
            {
                "name": "last",
                "type": "string"
            }
        ]
    }

The first column is allowed to have a null value, and the second column must contain
unique values.

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### GET /tables/_table_  <a name="netadata"></a>

If you specify a specific table with the GET operation, it returns JSON payload containing
an array of structure, each of which defines the column name, type, size, and nullability.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### DELETE /tables/_table_  <a name="deletetable"></a>

A DELETE operation to a specific table will delete that table and it's contents from the
database, if the current user has `delete` privilege for that table.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### PUT /tables/@sql  <a name="sql"></a>

This is a variation of the previous operation; it allows execution of an arbitrary SQL
statement, if the current user has `admin` privileges. The SQL text to execute must be
passed as a JSON-encode string in the body of the request. The reply will either be a
rowset or a rowcount object, depending on whether the statement was a `select` operation
(which returns a row set), versus any other statement which just returns a count of the
number of rows affected.

For example, here is a request payload that joins two tables and returns a result. Because
this is a SQL `select` statement, the _Ego_ server knows to reeturn a rowset as the result.
Otherwise, it returns a rowcount as the result.

    "select people.name, surname.name 
         from \"mary\".\"people\" 
         join \"mary\".\"surname\"
            on people.id == surname.id"

Note that the string must be properly escaped as a JSON string.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;


### PUT /tables/@transaction  <a name="tx"></a>

This operation allows you to specify an _atomic_ list of operations that must all be
successfully performed for the change to occur. That is, it specifies a list of
update or delete operations to arbitrary tables, and the changes will be made _only_
if all the changes can be made successfully. This allows the caller to create a
_transaction_ of multiple operations. Other users of the same table will only ever
see the state of the database before the transaction, or after it is entirely complete.
No other user of the database will see a partial version of the update while it is in
progress. This is particularly helpful if the client logic requires updates or deletes
to multiple tables to reflect a single logical operation, and all the operations must
happen together.

The payload for a transaction is an array of tasks. Each task has the following
members:

| Task Item   | Description |
| ----------- | ------------|
| operation   | The operation to be performed for this particular task. Can be "DELETE", "INSERT", or "UPDATE" |
| table       | The name of the table on which to perform the operation. |
| filters     | If filters are used for this operation, this is an array of filter specificiations |
| columns     | If subset of the data is to be used, this is an array of the column names to be affected |
| data        | A representation of a single row, where the object field name is the column name and teh object field value is the column value. |

If the operation requires multiple filters, those can be individually specified in the `filters` array; each filter is
impplicity joined to the others by an AND() operation, so that all the filters specifiec must be true for the filter
to match a row. For operations that do not specify a filter (such as "INSERT"), the `filters` list can be empty. 
For operations are intended to use all the fields of the "data" element, the `columns` list can be empty. 
For "DELETE" or "DROP" operations, the `data` element can be empty or omitted from the payload.

Here is a sample payload with three transactions:

    [
        {
            "operation": "insert",
            "table": "x6",
            "data": {
                "address": "123 Elm St",
                "description" : "tx row",
                "first": "Elmer",
                "last": "Fudd",
                "role": "tester"
            }
        },
        {
            "operation": "insert",
            "table": "x6",
            "data": {
                "address": "125 Elm St",
                "description" : "tx row",
                "first": "Daffy",
                "last": "Duck",
                "role": "tester"
            }
        },
        {
            "operation": "update",
            "table":"x6",
            "filters":[
                "EQ(description,'tx row')"
            ],
            "columns": [
                "address"
            ],
            "data": {
                "address":"666 Scary Drive"
                "description" : "tx row",
                "first": "Daffy",
                "last": "Duck",
                "role": "tester"
            }
        }
    ]


The first and second tasks insert new data into the table "x6". The third task updates the address
of any row that matches the filter of a "description" field equal to "tx row". Note that the third
task also explicitly specifies a `columns` list. This means that even though the `data` item contains
many fields, the only field that will be updated is "address" from the data object.

If the insert fails (perhaps due to a constraint violation, etc.) then no data will be inserted. If
the inserts succeed but the upddate fails (perhaps there is a syntax error in the filter list), then
no inserts or updates will occur.  If any error occurs, the resulting message indicates how many
tasks were processed before the error was encountered, and what the error was.

A successful transaction will return a rowcount object, which has a field "count" which contains
the number of rows affected by _all_ the transactions processed.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;


## Rows API <a name="rows"></a>

This section covers API functions to
* [Read rows from a table](#readrows)
* [Insert new rows into a table](#insertrows)
* [Update existing rows in a table](#updaterows)
* [Delete rows from a table](#deleterows)

The API for accessing row data in a table uses the /tables/_table_/rows endpoint name. The
result is either a row set (for GET of table rows) or a row count for any other operation
that indicates how many rows were affected by the operation. This includes PUT (write rows),
PATCH (update rows), and DELETE (delete rows)
&nbsp;
&nbsp;

### GET /tables/_table_/rows <a name="readrows"></a>

This reads the rows from a table. If no parameters are given, then all columns of all rows
are returned, in an unpredictable order. The result is a JSON payload containing an array
of structures, each of which is a set of fields for each column name, and the value of that
field in that row.

The Rows API supports the following parameters on the URL that affect the result set. 
Additional information about the parameters follows this table:

| Parameter | Example                | Description |
|:--------- |:---------------------- |:----------- |
| columns   | ?columns=id,name       | Specify the columns to return (if not specified, all columns are returned) |
| filter    | ?filter=EQ(name,"TOM") | Only return rows that match the filter |
| limit     | ?limit=10              | Return at most this many rows from the result set |
| sort      | ?sort=id               | Sort the result set by the named column |
| start     | ?start=100             | Specify the first row of the result set (1-based) |

&nbsp;

You can specify the sort order of the results set by naming one or more columns on which the
data is sorted before it is retuned to you. Use the `sort` parameter, with a value which is 
a comma-separated list of columns. The first column named is the primary sort key, the second
column (if any) is the secondary sort key, etc. You can prefix the column name with a tilde ("~")
character to make the sort order descending instead of ascending.

You can specify the columns that are to be returned using the `columns` parameter, with a value
that is a comma-separate list of column names. Only those columns are returned in the payload.

You can filter the rows returned using the `filter` parameter, which contains a filter 
expression. This consists of an operator, followed by one or two operands in parenthesis. The
operands can themselves be filter expressions to create complex expressions. The operators
are:

&nbsp;

| Operator | Example      | Description |
|:-------- |:------------ |:----------- |
| EQ       | EQ(id,101)   | Match rows where the named column has the given value |
| LT       | LT(age, 65)  | Match rows where the named column's value is less than the given value. |
| LE       | LE(size,12)  | Match rows where the named column's value is less than or equal to the given value. |
| GT       | LT(age, 65)  | Match rows where the named column's value is greater than the given value. |
| GE       | LE(size,12)  | Match rows where the named column's value is greater than or equal to the given value. |
| AND      | AND(EQ(id,1),EQ(id,2)) | Both operands must be true |
| OR       | OR(EQ(id,1),EQ(id,2)) | Either operands must be true |
| NOT      | NOT(EQ(id,101)) | Match rows where the operand expression is not true |
| HAS      | HAS(foo, 'YES', 'NO') | Match rows where character column `foo` contains "YES" _or_ "NO" |
| HASALL   | HASALL(foo, 'YES', 'NO') | Match rows where character column `foo` contains "YES" _and_ "NO" |

&nbsp;

The AND() and OR() operators can contain a list of two or more values. If you specify multiple values, then
in the case of AND() the filter is active if _all_ of the sub-expressions are true and in the case of OR()
the filter is active if _any_ of teh sub-expressions are true. 

For the HAS() operator, the first item must be the column name and this is followed by one or more 
substrings that might be found in the column name; the filter is true if _any_ of the values are 
present in the column string value. For HASALL(), the parameters are the same as HAS() but the 
condition is true only if _all_ of the values represented are found in the column string. The HAS() 
and HASALL() operations are case-sensitive.

Note that in these examples, the value usually being tested is an integer. You can also specify a string value 
in double quotes, or a floating point value (such as 123.45).

&nbsp;

The result is called a "rowset" and consists of an object with two values.

| Field | Description |
|:----- |:----------- |
| rows  | An array of JSON objects, representing a row. The field names are the column names, and the field value are the row values |
| count | An integer value that indicates how many items were returned in the rows array |

&nbsp;

Here is an example output from the call to read rows. This table has two columns defined
by the user, called `Number` and `Name`. There are two rows in the result set. Note that
the result set also includes the synthetic column name `_row_id_` which contains a unique
identifier for each row in the database.

&nbsp;

    {
      "server": {
          "api": 1,
          "name": "appserver.abc.com",
          "id": "2ef21c8f-cc4f-4a83-9e62-b7b7561c64ce",
          "session": 1525
      },
      "rows": [
        {
            "Name": "Tom",
            "Number": 101,
            "_row_id_": "76d3e219-1015-49c8-9e77-decb750ad13e"
        },
        {
            "Name": "Mary",
            "Number": 102,
            "_row_id_": "a974019e-f9e7-4554-adb4-2004b6f65c03"
        }
      ],
    "count": 2
    }

&nbsp;


In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;


### PUT /tables/_table_/rows <a name="insertrows"></a>
The PUT method inserts new rows into the table. The payload is either a row 
descriptor which is a JSON object describing the values of each column in 
the row to be added, or a rowset which consists of an object with an array
of row descriptors. The latter allows an insert of multiple rows at one time.
If a column is not specified in the body of the request, the corresponding
value in the table is null/zero.

You do not need to specify a `_row_id_` item; this will be set for you when the
new row is created.

&nbsp;

Here is an example payload that can be sent to the server to insert a single
new row for account number 103 wtih name "Susan".

&nbsp;

    {
        "Name": "Susan",
        "Number": 103
    }

&nbsp;

If the row is successfully inserted, the result is a JSON object with a single field,
`count` which should contain the number 1. _In the future, it will be possible to
insert multiple rows in a single call, in which case this value will reflect the number
of rows inserted._

You can also send a list of rows that are to be inserted using a rowset. Here is a 
sample payload that inserts three rows as a single operation:
&nbsp;

    {
      "rows" :[
        {
            "Name": "Susan",
            "Number": 103
        },
        {
            "Name": "Timmy",
            "Number": 104
        },
        {
            "Name": "Mike",
            "Number": 105
        }
      ],
      "count": 3
    }

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### PATCH /tables/_table_/rows <a name="updaterows"></a>
The PATCH method updates existing rows in the table. The payload is a row descriptor (the same
as the PUT method) but does not have to specify all the values in the row. Only the values
specified in the request body are updated; the other values are left unchanged.

Use the `filter` parameter to select which row(s) are to be updated:

| Parameter | Example                | Description |
|:--------- |:---------------------- |:----------- |
| filter    | ?filter=EQ(name,"TOM") | Only return rows that match the filter |
| columns   | ?columns=Id,Name       | Only update the named columns from the request payload |

Note that the use of `columns` is present to support the case where the client needs to present
a model of the entire object represetned by the table row, but only wants to update specific
values in that model (this can be important for performance when updating an index value, for
example). If `columns` is not specified, then all fields in the request payload are updated.

&nbsp;

If a `_row_id_` field exists in the row representation, then *only* the matching row
in the table with that exact ID will be update. If not specified, all rows will be updated 
using the same value. In addition to a `_row_id_` you can use the `filter`
option to select specific rows that are to be updated. You can reference the column values 
if they are sufficiently unique. 

A common usage is to perform a GET operation on the row(s) you wish to update so you have
a rowset with all the IDs alreaady in them. You can then update the value(s) you wish in 
the rowset, and then pass the rowset back for the PATCH operation to update the values. The
presence of the `_row_id_` column in the rowset guarantees that only the rows in the rowset
are updaed, and you may not need any further filtering. Note that in this case, you can still
use the ?columns parameter to specify the columns in the rowset that are to be used for the
update, and any other column info in the rowset is ignored.

&nbsp;

Here is an example payload that can be sent to the server to update the row
for account number 101 to change the name to "Bob". The account number (and
the synthetic row ID) are not modified by this operation. This assumes that
only one row in the table has the given account number of 101.

&nbsp;

    {
        "Name": "Bob",
    },

&nbsp;

The url request formed would be something like:

    PATCH http://localhost:8080/tables/Accounts/rows?filter=EQ(Number,101)

This specifies that the row is to be updated (a `PATCH` method call) and the
only row(s) to be updated are those where the `Number` field is equal to 101.
You can also use the `_row_id_` variable to specify a specific row, which is
alwasy guaranteed to be unique.

When this call runs successfully, the resulting payload is an object with a
field `count` which contains the number of rows that were changed by this
operation. A value of zero means the filter did not allow any rows to be
modified. You can also use this to verify that you updated one and only one
row if you needed the update to be unique to a particular row.

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### DELETE /tables/_table_/rows <a name="deleterows"></a>

This deletes rows from a table. By default, all rows are deleted. You can use the following
parameter to specify which rows are to be deleted:

&nbsp;

| Parameter | Example                | Description |
|:--------- |:---------------------- |:----------- |
| filter    | ?filter=EQ(name,"TOM") | Only delete rows that match the filter |

&nbsp;

If no parameters are given, then all rows are deleted. You can
specify the `filter` parameter to indicate which row(s) are to be deleted from the
table. The `filter` parameter contains a filter expression, of the same form as the GET
operation.


&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

## Permissions
A permissions table is managed by the _Ego_ server that controls whether a given use can read,
update, or delete a given table.  By default, a user can only set these attributes on tables
that they own. An administrator (a user account with "root" privilege) can change the attributes
of any table for any user.

* [Read all permittions](#allperms)
* [Read permissions for a specific table](#tableperms)
* [Set permissions for a specific table](#setperms)

You can read the entire list of permissions if you are an admin user.

&nbsp;
&nbsp;

### GET /tables/@permissions <a name="allperms"></a>

This command specifies the pseudo table name `@permissions` to indicate that the request is to
read all the permissions data for all tables. The result is a JSON payload with an array for
each permissions object stored in the database, listing the user, schema, table, and a string
array of permission names.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### GET /tables/_table_/permissions  <a name="tableperms"></a>

This command returns a permissions object for the given table and the current user.  This includes
the user, schema, table, and a string array of permission names.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

### PUT /tables/_tables_/permissions  <a name="setperms"></a>

This command will update the permissions for the given table and the current user. The body of
the request must contain a JSON payload with a string array of permission names. The name can
start with a "+" in which case the permission is added to the existing data. If the name starts
with a "-" then the permission is removed. It is not an error to remove a permission that does
not exist.

If there is no existing permission data for the user, a new permissions object is created in the
security database for the current user, initialized with the permissions provided.

&nbsp;

In the event that the REST call returns a non-success status code, the response payload
will contain the following diagnostic fields as a JSON payload:

| Field     | Description |
|:--------- |:----------- |
| status    | The HTTP status message (integer other than 200) |
| msg       | A string with the text of the status message |

&nbsp;
&nbsp;

# Services <a name="services"> </a>

All remaining REST endpoints are provided under the `/services` path point. Use of this path
means that the server will load, compile, and run an Ego program that will respond to the given
REST API. This is the mechanism by which the developer can extend the server functionality
specific to the needs of the end users.

There are a number of /services endpoints provided in the default installation, and at least
one (the /services/admin/logon endpoint) is required for a secure, authenticated server. This
section will describe the endpoints provided in the default deployment.

By convention, the _EGO_PATH_/services directory includes a number of subdirectories to support
functions of an _Ego_ web server.

| Subdirectory | Description |
|:------------ |:----------- |
| admin        | Contains services to support administrative and debugging services, such as logon |
| assets       | Contains any static resources that are served via the /assets endpoint, such as images |
| templates    | Contains static template files (usually) HTML that are used by services. |

You can see examples of this by examining the /services/admin/memory endpoint. 

* The code that is loaded and run is in the admin/memory.ego file. This is the primary endpoint name.
* The code uses a template located in the /templates directory that forms the HTML component of the result
* The template includes references to read a PNG image from the /assets directory in forming the web page

## Example Service Code
Here is the full _Ego_ code for the /services/admin/memory service, found in the "memory.ego" file:


    import "http"

    func mb(f float64) string {
        return fmt.Sprintf("%3.2fmb", f)
    }

    func handler( req http.Request, resp http.Response ) {

        // Prepare the data to be used by the page.
        m := util.Memory()
        pageData := { 
            Allocated: mb(m.current),
            Total: mb(m.total),
            System: mb(m.system),
            GC: m.gc,
            ID: _server_instance,
            Date: time.Now().String(),
            Host: os.Hostname(),
        }

        // Given a path to the template asset, write the page using the
        // dynamically generated data
        resp.WriteTemplate("lib/services/templates/memory.html", pageData)

    }

The service always calls the `handler` entrypoint, and always passes in the request
and response objects. The services uses the built-in `util.Memory()` function to get
information about the system memory usage. It then extracts the data it wants to
send to the user, including using the local `mb()` function defined in the service
file to format bytes as megabytes.

Finally, the code uses the `resp.WriteTemplate()` function to reference a template
file, and provide the values that are to be plugged into the template file. The
template processor reads the template file, performs any substitutions in the template
from the supplied data structure (so that a reference to `{{.Total}}` in the template
is replaced with the value of `pageData.Total` from the supplied data structure).

## Example Template File

The template contains the actual HTML text that will be sent back as the response to
the query (via `resp.WriteTemplate()` in the service code). 

The template contains
both static text, and substitution operators, which are identified by being enclosed
in double-braces, such as `{{.Total}}` which is a substutiton operator for a field
named `Total` in the data structure supplied with the template. This allows the
template to contain the design/formatting code needed to present the desired page,
while variable values can be injected as part of the template processing.

Here is the associated template file, located in lib/services/templates/memory.html:

    <!DOCTYPE html>
    <!-- Demo web page dynamically rendered by a service. -->
    <html>
        <head>
            <title>Ego Memory ({{.Host}})</title>
            <style>
                table,
                td,
                th {
                    border: none;
                    width: 400px;
                    border-collapse: collapse;
                }
            </style>
        </head>

        <body>
            <table style="border: none;width: 440px;border-collapse:collapse">
                <tr>
                    <td>
                        <!-- The asset must have a root path of /assets to be located properly -->
                        <img src="/assets/logo.png" alt="Ego logo" style="width:300px;height:150px;">
                    </td>
                    <td>
                        <h1>&nbsp;&nbsp; Memory Statistics</h1>
                    </td>
                </tr>
            </table>
            <br> <br>{{.Date}}
            <p>
                <table>
                    <tr>
                        <td>Currently allocated</td>
                        <td style="text-align: right">{{.Allocated}}</td>
                    </tr>
                    <tr>
                        <td>Total allocated</td>
                        <td style="text-align: right">{{.Total}}</td>
                    </tr>
                    <tr>
                        <td>System memory</td>
                        <td style="text-align: right">{{.System}}</td>
                    </tr>
                    <tr>
                        <td>Garbage collections</td>
                        <td style="text-align: right">{{.GC}}</td>
                    </tr>
                </table>
                <br> Server {{.Host}}, session {{.ID}}
                <br>
        </body>
    </html>


Note the references to substitution operators throughout the page, showing where the
text of the service data structure items are injected into the HTML page that is
sent back to the caller.

Also note that there is a reference to an image via an img src="..." tag. This 
will case the web brower presenting the HTML to make a second call to the _Ego_
web server to retrieve the image from the assets directory on the web server.




