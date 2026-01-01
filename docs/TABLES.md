# Ego Table Services

The _Ego_ server can be used as a REST-based database, with standard database
operations like insert, update, delete that are ACID-compliant database
operations. Additionally, the _Ego_ command line interface has a set of
commands (the `tables` commands) that support accessing the database from
a shell environment.

&nbsp;
&nbsp;

## Data Sources

To access a database, an administrator must create a data source name (DSN)
object. This can be done using the `ego` command line or via API access. The
DSN indicates all the information needed by the server to access the data
store. Currently, PostgreSQL and SQLite3 are the only supported data source
types. The DSN information includes the credentials used to connect to
Postgres when that is the specified data source type.

When accessing a table, the DSN is specified, which allows the Ego server to
retrieve the database information and credentials from the DSN store and
access the underlying data base. This prevents the end user from needing to
know the actual database credentials.

In addition to the information needed to access the database, the DSN may
include information that controls what kinds of operations may be done using
the database. A DSN can have no restrictions, but if any username is granted
permissions on the DSN, then any access using the DSN must ber validated
against the DSN authorizations. This determines of a given user can read,
write, or perform administrative functions (like creating or dropping a
table) via the named DSN.

## Data Source Name Commands

The `dsns` command set is used to manipulate the list of data source names (DSNs)
managed by the Ego server. These commands all require authentication using an
administrator account.

```text
Usage:
   ego dsns [command]           Manage data source names

Commands:
   add                          Add a new data source name                              
   delete                       Delete a data source name                               
   grant                        Grant permissions to a user for a data source name      
   list                         List the DSNS known to the server                       
   revoke                       Revoke permissions from a user for a data source name   
   show                         Show permissions for a data source name                 
```

### dsn add

This adds a new DSN. Each DSN name must be unique.

```sh
ego dsn add --name payroll --database payroll --type postgres -u dbuser -p dbpass
```

In this example, a new DSN named `payroll` is created. While not required, it is
a convention that the DSN and the database name be the same when it is a Postgres
DSN. If the type is `sqlite3` instead of `postgres` then the database name is the
full file system path to the Sqlite3 database file.

Because this DSN is of type `postgres` is must include a user and password that
are stored with the DSN information. By default, the Postgres server is assumed
to be on the same host as the Ego server and running on the default port, but
this can be overridden using the `--host` and `--port` command line options.

By default, a DSN is created as unsecured, which means any user can access it.
Use the `--secured` command line flag to indicate that only specific users are
allowed to access the data. When this is the case. the administrator _must_
use the `dsns grant` command to grant permission for a user to access the
data source name.

### dsn delete

The `delete` subcommand is used to remove a data source name from the Ego
server. Any existing connections that are using this DSN are unaffected, but
no additional connections are permitted for the deleted DSN.

```sh
ego dsns delete --name payroll
```

The name of the DSN to remove must be specified in the command.

### dsn grant

The `grant` subcommand gives a user permissions to access a data source
name. The permissions are `read`, `write`, and `admin`. The `read` permission
is used to read data from the database. The `write` permission is used to
modify or delete records from the database. The `admin` permission is required
to create a new table or drop an existing table, or use the native SQL
database interface.

```sh
ego dsns grant --name payroll --user jsmith --permissions read,write
```

This grants the user `jsmith` both read and write permissions on the data
source name `payroll`. This means that "jsmith" can read or write rows in
any table referenced by this data source.

### dsn list

The `list` subcommand lists all the data source names managed by the Ego
server, including the database type, database name, default schema if it
is a Postgres database, and other information indicating the database user
name and whether access to this data source name is restricted or not.

### dsn revoke

The `revoke` subcommand removes user permissions to access a data source
name. The permissions are `read`, `write`, and `admin`. Only the specified
permissions are removed from the user authorizations.

```sh
ego dsns revoke --name payroll --user jsmith --permissions write
```

In this example, user "jsmith" has the `write` permission removed from the
data source name `payroll`. Any other permissions that "jsmith" had for this
DSN are unaffected. So if this command followed the example in the `grant`
subcommand, this user would still retain the `read` permission.

### dsn show

The `show` subcommand indicates the data source name permissions that exist
for each user. The output is a list of users and their permissions.

```sh
ego dsns show --name payroll
```

## Table Commands

The `tables` command set is used to manipulate database tables from a shell by an interactive
user. All commands start with `ego tables` following by the subcommands:

```text
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
```

For each command that specifies a table name, you can specify the `--dsn` option
that specifies the data source name to be used to access that table. If the `--dsn`
option is not given, then the Ego server attempts to get the dsn from a two-part
table name, such as "foo.bar" where "foo" is the data source name, and "bar" is
the table name.

The following sections detail each command.
&nbsp;

### table create

The `create` command creates a new table, specified as the first parameter of the
command line. This must be followed by one or more column specifications. A column
specification consists of the column name, a `:` (colon) character, and the _Ego_
data type for that column.  The valid types that you can specify for a table are:

| Type | Description |
| :------- | :----------- |
| string | Varying length character string |
| int | Integer value |
| int32 | Integer value expressed in 32-bits instead of 64 |
| float32 | Real floating point value |
| float64 | Double precision floating point value |
| bool | Boolean value (can only be `true` or `false`) |

Additionally, you can specify supported attributes of
the column separated by commas after the type name.

| Attribute | Description |
| :--------- | :----------- |
| nullable | The column value is allowed by be a SQL null value |
| unique | The column values must be unique within the table |

&nbsp;

If the column specification contains spaces, the entire column
specification must be in quotes. For example,

```sh
    ego table create employees --dsn payroll id:int first:string "last:string, unique, nullable"
```

 The table `employees` is found in the database accessed via the
 data source name `payroll`. This creates a new table with three
 user-defined columns. The third specification is in quotes because
 there is a space after the comma. This could be expressed without
the quotes by removing the space characters from the specification.

If the `--dsn`
option is not given, then the Ego server attempts to get the dsn from a two-part
table name, such as "foo.bar" where "foo" is the data source name, and "bar" is
the table name.
&nbsp;

### table list

The `list` command lists all tables that the current user has access to. Note that there
may be tables in the database that are not included in the list, if the user does not have
admin privilege and there is not a corresponding entry in the permissions table for that
user and table with the `read` permission specified.

The data is printed to the console as a list of the table names. For example,

```text
    user@Macbook % ./ego tables list --dsn family
    Name          Schema     Columns  Rows
    ==========    =========  =======  ====
    members       admin            5   127
    simple        admin            1     8
    test1         admin            4     0
```

This shows a listing of three tables that the current user can read using the
data source name `family`. In this example, the table "test1" has no rows in
it, so the row count reported is zero.

You can omit the row counts (which can take a while for very very large tables) using
the `--no-row-counts` option on the `list` command.

&nbsp;

### table show

The `show` command is used to display the column information for a given table.
You must specify the name of the table as the command parameter. The output
includes the column name, type, size, and whether it is allowed to contain
a null/empty value.  For example, here is a display of the privileges table
discussed in an earlier section, assuming the current user has logged into the
session as the `admin` user:

```text
    user@Macbook  % ./ego tables show privileges
    Name           Type      Size    Nullable    Unique
    ===========    ======    ====    ========    ======
    permissions    string      -5    true        false
    tablename      string      -5    false       true
    username       string      -5    false       true
```

This shows the three column names, the type (in this case, always string values),
the size (-5 applies to `char varying` types) The `permissions` column is allowed
to have null values, and the `tablename` and `username` columns must be unique.

If the `--dsn`
option is not given, then the Ego server attempts to get the dsn from a two-part
table name, such as "foo.bar" where "foo" is the data source name, and "bar" is
the table name.
&nbsp;

### table read

The `read` command (which can also be expressed as `contents` or `select`) reads
rows from a table and displays the values on the console. You must specify the name
of the table as the command parameter.

```text
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
```

Note that the order of the rows in unpredictable (in practice, it usually is in the
order the items were added or last updated, but this is not guaranteed). You can specify
the order of the output using the `--order-by` command option:

```text
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
```

You can further influence the output by specifying filters that are applied to the
query to select specific rows. For example,

```text
    user@Macbook ~ % ./ego table read simple filter='id < 200' --order-by id
    id     name    
    ===    ====    
    101    Tom     
    102    Mary    
    103    Chelsea    
    104    Sarah  
```

This limits the output to only rows where the `id` column is less than the value
200. You can specify multiple filters separated by commas if needed:

```text
    user@Macbook ~ % ./ego table read simple filter='id < 200','name="Tom" --order-by id
    id     name    
    ===    ====    
    101    Tom     
```

The filters are comma-separated items, where each filter must be enclosed in quotes. There
cannot be a space outside the quotes in the filter expression, including after the comma.

Finally, you can choose to only display specific column(s) in the output, using the `--column`
command option:

```text
    user@Macbook ~ % ./ego table read simple filter='id = 101','name="Tom" --column=name
    name    
    ====    
    Tom     
```

You can specify multiple column names by separating them by commas. The columns are printed
in the order specified in the `--column` option.

If the `--dsn`
option is not given, then the Ego server attempts to get the dsn from a two-part
table name, such as "foo.bar" where "foo" is the data source name, and "bar" is
the table name.
&nbsp;

### table insert

The `insert` command adds a single row to the specified table. The first parameter must be
the name of the table, and this is followed by one or more column value specifications.
For example,

```sh
    user@Macbook ~ % ./ego table insert bog.simple id=301 name="Suzy"
```

This will add a new row to the table "simple" in the DSN "bog". The row will have `id`
set to the value `301` and `name` set to
the value `Suzy`. The command will report that a row was added to the table if it is
successful. You cannot insert into a table that you do not have administrator privileges
or `update` privilege for that table. You must only specify column names that already
exist on the table; otherwise the row is not added and an error is reporting showing the
first column in your command that is not in the named table.

&nbsp;

### table update

The `update` command modifies columns in rows of the specified table. The first
parameter must be the name of the table, and this is followed by one or more column
value specifications. For example,

```sh
    user@Macbook ~ % ./ego table update bog.simple name="Suzy"
```

This will change the value of the column `name` to the value `Suzy` for every row in
the table. A more common case it to update a specific row or set of rows using
an optional `--filter` command line option.

```sh
    user@Macbook ~ % ./ego table update simple name="Suzy" --filter 'id=101'
```

This variation will only update row(s) that also have a value of `101` for the `id`
column. Note that other columns in the row not named in the command are unchanged
by this operation.

The command will report how many rows were modified in the table if the command is
successful. You cannot update columns in a table that you do not have administrator privileges
or `update` privilege for that table. You must only specify column names that already
exist on the table; otherwise no rows are updated and an error is reporting showing the
first column in your command that is not in the named table.

&nbsp;

### table delete

The `delete` command deletes rows from the specified table. The first
parameter must be the name of the table. For example,

```sh
    user@Macbook ~ % ./ego table delete bog.simple 
```

This will delete every row in the table. A more common case it to delete a specific
row or set of rows using an optional `--filter` command line option.

```sh
    user@Macbook ~ % ./ego table delete bog.simple --filter 'id=101'
```

This variation will only delete row(s) that have a value of `101` for the `id`
column.  The command will report how many rows were deleted in the table if the command is
successful. You cannot delete rows from a table that you do not have administrator privileges
or `delete` privilege for that table.

&nbsp;
&nbsp;
