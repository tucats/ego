
     _____           _       _
    |_   _|   __ _  | |__   | |   ___   ___
      | |    / _` | | '_ \  | |  / _ \ / __|
      | |   | (_| | | |_) | | | |  __/ \__ \
      |_|    \__,_| |_.__/  |_|  \___| |___/



# Ego Table Services

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
       create                    Create a new table
       delete                    Delete rows from a table   
       drop                      Delete a table             
       help                      Display help text          
       insert                    Insert a row to a table    
       list                      List tables      
       permissions               Show all table permissions (required admin privileges)
       read                      Show contents of a table   
       show-permissions          Show table permissions          
       show-table                Show table metadata    
       sql                       Execute arbitrary SQL (requires admin privileges)    
       update                    Update rows to a table     

The following sections detail each command.

## create
The `create` command creates a new table, specified as the first parameter of the
command line. This must be followed by one or more column specifications. A column
specification consists of the column name, a `:` (colon) character, and the _Ego_
data type for that column. If the column is also nullable, you can specify ",nullable"
after the data type. If the column specification contains spaces, the entire column
specification must be in quotes.

    
    ego table create employees id:int last:string "first:string, nullable"

This creates a new table with three user-defined columns. The third specification is
in quotes because there is a space after the comma. This could be expressed wtihout
the quotes by removing the space character from the command.


## list

The `list` command lists all tables that the current user has access to. Note that there
may be tables in the database that are not included in the list, if the user does not have
admin privilege and there is not a corresponding entry in the permissions table for that
user and table with the `read` permission specified.

The data is printed to the console as a list of the table names. For example,

    user@Macbook % ./ego tables list
    Name          Schema     Columns  Rows
    ==========    =========  =======  ====
    members       admin            5   127
    simple        admin            1     8
    test1         admin            4     0

This shows a listing of three tables that the current user can read. The table "test1"
has no rows in it, so the row count reported is zero.

You can omit the row counts (which can take a while for very very large tables) using
the `--no-row-counts` option on the `list` command.


## show-table

The `show-table` command is used to display the column information for a given table.
You must specify the name of the table as the command parameter. The output
includes the column name, type, size, and whether it is allowed to contain
a null/empty value.  For example, here is a display of the privilges table
discussed in an earlier section, assuming the current user has logged into the
session as the `admin` user:

    user@Macbook  % ./ego tables show-table privileges
    Name           Type      Size    Nullable    
    ===========    ======    ====    ========    
    permissions    string      -5    false       
    tablename      string      -5    false       
    username       string      -5    false   

This shows the three column names, the type (in this case, always string values),
the size (-5 applys to `char varying` types), and none of the fields are allowed
to have null values.


## read

The `read` command (which can also be expressed as `contents` or `select`) reads
rows from a table and displays the values on the console. You must specify the name
of the table as the command parameter.

    user@Macbook ~ % ./ego table read simple
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

   user@Macbook ~ % ./ego table read simple --order-by id
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

   user@Macbook ~ % ./ego table read simple filter='id < 200' --order-by id
    id     name    
    ===    ====    
    101    Tom     
    102    Mary    
    103    Chelsea    
    104    Sarah  

This limites the output to only rows where the `id` column is less than the value
200. You can specify multiple filters separated by commas if needed:

   user@Macbook ~ % ./ego table read simple filter='id < 200','name="Tom" --order-by id
    id     name    
    ===    ====    
    101    Tom     

The filters are comma-separated items, where each filter must be enclosed in quotes. There 
cannot be a space outside the quotes in the filter expression, including after the comma.

Finally, you can choose to only display specific column(s) in the output, using the `--column`
command option:

    user@Macbook ~ % ./ego table read simple filter='id = 101','name="Tom" --column=name
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
