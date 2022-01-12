
# Table of Contents

1. [Introduction](#intro)

1. [Authentication](#auth)

1. [Administration](#admin)

2. [Tables](#tables)

3. [Services](#services)

&nbsp;
&nbsp;

# Introduction <a name="intro"></a>
This document describes the REST Application Programming Interfaces (APIs) 
supported by an _Ego_ server instance. This covers how to authenticate to the 
server, administration functions that can be performed by a suitably 
privileged user, APIs for directly accessing database tables and their data, 
and APIs for accessing user-written services (implemented as _Ego_ programs)


&nbsp;
&nbsp;

# Authentication <a name="auth"></a>
Other than determining if a server is running or not, all operations performed
against an Ego server must be done with an authenticated user. The normal pattern
for this is for the client to "log in" the user, and receive an encrypted token
in return. This token can be presented to any other API to authenticate the user
for the functions of that API.

To authenticate, use a "GET" method to the endpoint "/services/admin/logon". The
request must:

* Use BASIC authentication to pass the username and password to the servce. 
* Specify that the reply type accepted is "application/json"

The result is a JSON payload indicating if there was an error or not, and
the resulting secure token if the credntials were valid. The credentials must
match the username and password stored in the _Ego_ server credentials data 
store; you can use `ego` CLI commands to view and modify this store, as well
as `/admin` endpoints described below to read, update, or delete entries in
the credentials store.

The resulting JSON payload is a struct with the following fields:

| Field     | Description |
| :-------- | ----------- |
| expires   | A string containing the timestamp of when the token expires |
| issuer    | A UUID of the _Ego_ server instance that created the token |
| token     | A string containing the token text itself. |
| identity  | The username encoded within the token. |

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

# Administration <a name="admin"> </a>


&nbsp;
&nbsp;

# Tables <a name="tables"> </a>

The _Ego_ server includes a REST API for communicating with a PostgreSQL database configured
for use by the server. The API can be used to manage database tables and read, write, update,
and delete rows from tables.

## Table API

This section covers APIs to:
* Create a new table 
* List existing tables
* Show the column names and types for a table
* Delete an entire table
* Execute arbitrary SQL statements on the server

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

### GET /tables

A GET call to the /tables endpoint will return a list of the tables. This is a JSON payload
containing an array of strings, each of which is a table name that the current user has
read access to.

### GET /tables/_table_

If you specify a specific table with the GET operation, it returns JSON payload containing
an array of structure, each of which defines the column name, type, size, and nullability.

### DELETE /tables/_table_

A DELETE operation to a specific table will delete that table and it's contents from the
database, if the current user has `delete` privilege for that table.

### PUT /tables/_table_

A PUT to a named table will create the table. The payload must be a JSON specification that
is an array of columns, each with a `name` and `type` field. The table is created using
the given columns. Additionally, if you do not specify a column with name `_row_id_`, then
a column of that name is added to the table definition. This column will contain a UUID
that uniquely identifies the row across all tables.

The valid types that you can specify in the array of column structure definitions are:

| Type    | Description |
|:------- | ----------- |
| string  | Varying length character string |
| int     | Integer value |
| float32 | Real floating point value |
| float64 | Double precision floating point value |
| bool    | Boolean value (can only be `true` or `false`)


### PUT /tables/@sql

This is a variation of the previous operation; it allows execution of an arbitrary SQL
statement, if the current user has `admin` privileges. The SQL text to execute must be
passed as a JSON-encode string in the body of the request. The reply will either be a
rowset or a rowcount object, depending on whether the statement was a `select` operation
(which returns a row set), versus any other statement which just returns a count of the
number of rows affected.

## Row API

The API for accessing row data in a table uses the /tables/_table_/rows endpoint name. The
result is either a row set (for GET of table rows) or a row count for any other operation
that indicates how many rows were affected by the operation. This includes PUT (write rows),
PATCH (update rows), and DELETE (delete rows)

### GET /tables/_table_/rows

This reads the rows from a table. If no parameters are given, then all columns of all rows
are returned, in an unpredictable order. The result is a JSON payload containing an array
of structures, each of which is a set of fields for each column name, and the value of that
field in that row.

You can specify the sort order of the results set by naming one or more columns on which the
data is sorted before it is retuned to you. Use the `sort` parameter, with a value which is 
a comma-separated list of columns. The first column named is the primary sort key, the second
column (if any) is the secondary sort key, etc. You can prefix the column name with a tilde ("`")
character to make the sort order descending instead of ascending.

You can specify the columns that are to be returned using the `columns` parameter, with a value
that is a comma-separate list of column names. Only those columns are returned in the payload.

You can filter the rows returned using the `filter` parameter, which contains a filter 
expression. This consists of an operator, followed by one or two operands in parenthesis. The
operands can themselves be filter expressions to create complex expressions. The operators
are:

| Operator | Example | Description |
| -------- | ------- | ----------- |
| EQ       | EQ(id,101)   | Match rows where the named column has the given value |
| LT       | LT(age, 65)  | Match rows where the named column's value is less than the given value. |
| LE       | LE(size,12)  | Match rows where the named column's value is less than or equal to the given value. |
| GT       | LT(age, 65)  | Match rows where the named column's value is greater than the given value. |
| GE       | LE(size,12)  | Match rows where the named column's value is greater than or equal to the given value. |
| AND      | AND(EQ(id,1),EQ(id,2)) | Both operands must be true |
| OR       | OR(EQ(id,1),EQ(id,2)) | Either operands must be true |
| NOT      | NOT(EQ(id,101)) | Match rows where the operand expression is not true |


### DELETE /tables/_table_/rows

This deletes rows from a table. If no parameters are given, then all rows are deleted. You can
optionall specify the `filter` parameter to indicate which row(s) are to be deleted from the
table. The `filter` parameter contains a filter expression, of the same form as the GET
operation.


&nbsp;
&nbsp;
## Permissions
A permissions table is managed by the _Ego_ server that controls whether a given use can read,
update, or delete a given table.  By default, a user can only set these attributes on tables
that they own. An administrator (a user account with "root" privilege) can change the attributes
of any table for any user.

You can read the entire list of permissions if you are an admin user.

### GET /tables/@permissions

This command specifies the pseudo table name `@permissions` to indicate that the request is to
read all the permissions data for all tables. The result is a JSON payload with an array for
each permissions object stored in the database, listing the user, schema, table, and a string
array of permission names.

### GET /tables/_table_/permissions

This command returns a permissions object for the given table and the current user.  This includes
the user, schema, table, and a string array of permission names.

### PUT /tables/_tables_/permissions

This command will update the permissions for the given table and the current user. The body of
the request must contain a JSON payload with a string array of permission names. The name can
start with a "+" in which case the permission is added to the existing data. If the name starts
with a "-" then the permission is removed. It is not an error to remove a permission that does
not exist.

If there is no existing permission data for the user, a new permissions object is created in the
security database for the current user, initialized with the permissions provided.

&nbsp;
&nbsp;

# Services <a name="services"> </a>