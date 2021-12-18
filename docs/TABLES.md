# Tables

The _Ego_ server can be used as a REST-based database, with standard database
operations like insert, update, delete that are ACID-compliant database 
operations. Additionally, the _Ego_ command line interface has a set of
commands (the `tables` commands) that support accessing the database from
a shell environment.

&nbsp;
&nbsp;

## The Database

Currently, the only supported database backend is PostgreSQL. The _Ego_ server
will make connections to the database server as needed to support database
operations. The default location for the database is on localhost at port 5432.
You can specify a different host using the `ego.server.database.url` profile
value which is a full postgres:// URL specification for the host, credentials,
and database to connect.

You can also use the default database host:port and specify the specific name
of the database using the `ego.server.database.name` profile value. Finally,
you can specify the credentails to be used with the `ego.server.database.credentials`
profile value. If the credentials are not specified, the current username is
assumed, with no password.

The database should be partitioned into schemas, one for each authenticated user
in _Ego_. By default, all database operations are done within the schema for the
_Ego_ username. So for a username of `bob`,

    create schema bob;

will create the schema using the PSQL command line interface to Postgres. At
this time, there is no CLI interface to perform this operation.

Additionally, there should always be a schema of admin, which contains data
needed for the security checks done on behalf of an Ego user.  This should
always include a table named `privileges` which contains three string columns.

   
    create table admin.privileges(username char varying, 
                                  tablename char varying,
                                  permissions char varying);


The columns serve the following functions:

| Column      | Description |
| ----------- | ----        |
| username    | The _Ego_ username being granted permissions |
| tablename   | The database table for which permissions are granted |
| permissions | A comma-separated list of the permissions (read,update,delete) |

An _Ego_ user with the `root` privilege always has permission to do anything to 
the database backend. Other users must have an entry in the `privileges` table
for their username and the name of the database they want to access, and a string
value containing the allowed permissions.

| Permission | Description |
| ---------- | ------------|
| read       | User can see the table in a `list` operation, and read rows from the table |
| update     | User can insert, delete or update rows in the table |
| delete     | User can delete the table |

The permissions are based on the current _Ego_ user who has logged in using the `ego logon`
command, or who has used the REST API to log on to the Ego server and gotten a token. This
token is used as a bearer token for all database operations; you cannot perform any database
operation with a token. The user identity is encoded in the token value, and is used to
validate permissions.

&nbsp;
&nbsp;

# Commands

The `tables` command set is used to manipulate database tables from a shell by an interactive
user. All commands start with `ego tables` following by the subcommands:

    ego table [command]          Operate on database tables

    Commands:
       contents                  Show contents of a table   
       delete                    Delete rows from a table   
       drop                      Delete a table             
       help                      Display help text          
       insert                    Insert a row to a table    
       list                      List tables                
       show                      Show table metadata        
       update                    Update rows to a table     

The following sections detail each command.

## list

The `list` command lists all tables that the current user has access to. Note that there
may be tables in the database that are not included in the list, if the user does not have
admin privilege and there is not a corresponding entry in the permissions table for that
user and table with the `read` permission specified.

The data is printed to the console as a list of the table names. For example,

    user@Macbook % ./ego tables list
    Name          
    ==========    
    members    
    simple        
    test1         

This shows a listing of three tables that the current user can read.


## show

The `show` command is used to display the column information for a given table.
You must specify the name of the table as the command parameter. The output
includes the column name, type, size, and whether it is allowed to contain
a null/empty value.  For example, here is a display of the privilges table
discussed in an earlier section, assuming the current user has logged into the
session as the `admin` user:

    user@Macbook  % ./ego tables show privileges
    Name           Type      Size    Nullable    
    ===========    ======    ====    ========    
    permissions    string      -5    false       
    tablename      string      -5    false       
    username       string      -5    false   

This shows the three column names, the type (in this case, always string values),
the size (-5 applys to `char varying` types), and none of the fields are allowed
to have null values.


## contents

The `contents` command (which can also be expressed as `read` or `select`) reads
rows from a table and displays the values on the console. You must specify the name
of the table as the command parameter.

    user@Macbook ~ % ./ego table contents simple
    id     name    
    ===    ====    
    203    Fred    
    101    Tom     
    201    Tony    
    103    Chelsea    
    102    Mary    
    104    Sarah    
    202    Bob    

Note that the order of the rows in unpredictable (in practice, it usually is in the
order the items were added or last updated, but this is not guaranteed). You can specify
the order of the output using the `--order-by` command option:

   user@Macbook ~ % ./ego table contents simple --order-by id
    id     name    
    ===    ====    
    101    Tom     
    102    Mary    
    103    Chelsea    
    104    Sarah  
    201    Tony    
    202    Bob   
    203    Fred    

You can further influence the output by specifying filters that are applied to the
query to select specific rows. For example,

   user@Macbook ~ % ./ego table contents simple filter='id < 200' --order-by id
    id     name    
    ===    ====    
    101    Tom     
    102    Mary    
    103    Chelsea    
    104    Sarah  

This limites the output to only rows where the `id` column is less than the value
200. You can specify multiple filters separated by commas if needed:

   user@Macbook ~ % ./ego table contents simple filter='id < 200','name="Tom" --order-by id
    id     name    
    ===    ====    
    101    Tom     

The filters are comma-separated items, where each filter must be enclosed in quotes. There 
cannot be a space outside the quotes in the filter expression, including after the comma.

Finally, you can choose to only display specific column(s) in the output, using the `--column`
command option:

    user@Macbook ~ % ./ego table contents simple filter='id = 101','name="Tom" --column=name
    name    
    ====    
    Tom     

You can specify multiple column names by separating them by commas. The columns are printed
in the order specified in the `--column` option.

## insert

The `insert` command adds a single row to the specified table. The first parameter must be
the name of the table, and this is followed by one or more column value specifications.
For example,

    user@Macbook ~ % ./ego table insert simple id=301 name="Suzy"

This will add a new row to the table with `id` set to the value `301` and `name` set to
the value `Suzy`. The command will report that a row was added to the table if it is
successful. You cannot insert into a table that you do not have administrator privileges
or `update` privilege for that table. You must only specify column names that already
exist on the table; otherwise the row is not added and an error is reporting showing the
first column in your command that is not in the named table.


## update

The `update` command modifies columns in rows of the specified table. The first 
parameter must be the name of the table, and this is followed by one or more column 
value specifications. For example,

    user@Macbook ~ % ./ego table update simple name="Suzy"

This will change the value of the column `name` to the value `Suzy` for every row in
the table. A more common case it to update a specific row or set of rows using
an optional `--filter` command line option.

    user@Macbook ~ % ./ego table update simple name="Suzy" --filter 'id=101'
 
This variation will only update row(s) that also have a value of `101` for the `id`
column. Note that other columns in the row not named in the command are unchanged
by this operation.

The command will report how many rows were modified in the table if the command is
successful. You cannot update columns in a table that you do not have administrator privileges
or `update` privilege for that table. You must only specify column names that already
exist on the table; otherwise no rows are updated and an error is reporting showing the
first column in your command that is not in the named table.

## delete

The `delete` command deletes rows from the specified table. The first 
parameter must be the name of the table. For example,

    user@Macbook ~ % ./ego table delete simple 

This will delete every row in the table. A more common case it to delete a specific 
row or set of rows using an optional `--filter` command line option.

    user@Macbook ~ % ./ego table delete simple --filter 'id=101'
 
This variation will only delete row(s) that have a value of `101` for the `id`
column.  The command will report how many rows were deleted in the table if the command is
successful. You cannot delete rows from a table that you do not have administrator privileges
or `delete` privilege for that table. 

&nbsp;
&nbsp;

## REST API
The _Ego_ command line interfaces documented above use an API standard for communicating
with the server. You can use this API directly from your own code that communicates with
rest services.

You must have used the /admin/logon endpoint to log onto the server with a username and
password, and have been issued an authentication token. You use this token as the Bearer
token in the Authentication header. This is required for all table operations.

## Tables

The API can be used to list existing tables, show the column info for a specific table, and
to delete a table.  Note that _currently_ there is not an API to create a table, but that
is coming in the future.

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

## Rows

The API for accessing row data in a table uses the /tables/_table_/rows endpoint name. The
result is either a row set (for GET of table rows) or a row count for any other operation
that indicates how many rows were affected by the operation.

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



