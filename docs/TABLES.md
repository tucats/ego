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
data type for that column. The valid types that you can specify for a table are:

| Type | Description |
| :------- | :----------- |
| string | Varying length character string |
| int | Integer value |
| int16 | Integer value expressed in 16-bits instead of 64 |
| int32 | Integer value expressed in 32-bits instead of 64 |
| float32 | Real floating point value |
| float64 | Double precision floating point value |
| bool | Boolean value (can only be `true` or `false`) |
| timestamp | Date and time of day. See [Timestamp values](#timestamp-values) |
| date | Calendar date. See [Timestamp values](#timestamp-values) |
| time | Time of day. See [Timestamp values](#timestamp-values) |

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

### Timestamp values

**Supply timestamps in RFC 3339 format.** That is the documented contract for
`timestamp`, `date`, and `time` columns, whether the value arrives from the
`table insert`/`table update` commands or from a REST client's JSON payload:

```text
2024-06-15T12:00:00Z          a moment, stated in UTC
2024-06-15T12:00:00-05:00     the same kind of statement, five hours behind UTC
2024-06-15                    a date, read as midnight UTC
```

The defining property is that the value either states its offset from UTC
numerically or states none at all. Values are normalized to UTC before being
stored, and read back in the same format, so a timestamp written this way makes
the same round trip on every machine.

Other formats are still accepted — a value is parsed by format detection, not
by a fixed layout, so `June 15, 2024 12:00pm` and a Unix epoch value like
`1718452800` both work. What is _not_ accepted is a value whose timezone is
given only as a bare abbreviation that cannot be resolved:

```sh
    ego table insert bog.events title="launch" when="December 7, 1959 10:35am EST"
```

An abbreviation like `EST` carries no numeric offset, and the abbreviations are
not unique across the world — `CST` is US Central Standard Time, China Standard
Time, and Cuba Standard Time. Ego resolves such an abbreviation by looking it up
in the zone table of the location named by the `ego.runtime.timezone`
configuration setting (see [CONFIG.md](CONFIG.md)). If that location does not
use the abbreviation, the value is rejected:

```text
ambiguous timezone abbreviation; use a numeric offset such as -05:00: when
```

The request fails and no row is written. This is deliberate: a stored timestamp
is normalized to a UTC instant, so guessing the wrong offset would not produce a
visibly odd value — it would produce a plausible one, several hours from what
was meant, that reads back cleanly forever afterwards. Rejecting the value
leaves the caller able to correct it; accepting a guess does not.

Note that the reference zone is a property of the _server_ that stores the row,
not of the client that sent it. Two servers configured for different timezones
will resolve the same abbreviation differently. Stating the offset numerically
avoids the question entirely, which is why RFC 3339 is the recommendation.

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
a null/empty value. For example, here is a display of the privileges table
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
column. The command will report how many rows were deleted in the table if the command is
successful. You cannot delete rows from a table that you do not have administrator privileges
or `delete` privilege for that table.

&nbsp;

### table sql

The `sql` command (`ego sql`) sends SQL text you supply directly to the database server,
rather than building it from a structured command the way `table create`/`insert`/
`update`/`delete` do. It requires administrator privileges, or the `ego.sql` permission
on the DSN.

```sh
    user@Macbook ~ % ./ego sql --dsn payroll "SELECT * FROM employees"
```

The `--dsn` option identifies the data source name to run against, the same as the other
`table` commands. The SQL text itself is the command's final argument, or can be supplied
from a file with `--sql-file`. If the statement is a query, the results are displayed the
same way `table read` displays them; otherwise the number of rows affected is reported.

This same mechanism — raw, caller-supplied SQL text — is also reachable directly over
REST, as the `@sql` pseudo-table (`POST /dsns/{dsn}/tables/@sql`, taking a single
statement or a JSON array of statements run as one transaction) and as the `sql` opcode
inside a `@transaction` request. The `sql` CLI command is a thin wrapper over the same
`@sql` endpoint. Since this is the one place a caller writes SQL syntax directly rather
than having Ego generate it, it is also the one place where the two supported database
providers' differing SQL dialects can show through — see the next section.

&nbsp;

## SQL Dialect Translation

The structured commands documented above (`table create`, `table insert`, `table update`,
`table delete`, ...) always generate SQL that is already correct for whichever provider
the DSN uses — there is nothing to translate. This section is about `table sql` (and the
`@sql`/`@transaction` REST endpoints behind it) instead, where you supply the SQL text
yourself.

SQLite and PostgreSQL disagree on the syntax for a handful of common things — most
notably, how a column generates its own value on insert. Rather than requiring you to
know or track which provider a given DSN actually is, Ego rewrites a small, well-defined
set of constructs to match the DSN's real provider before running your SQL. You can write
either dialect's idioms and expect them to work against either kind of DSN.

This rewriting is separate from, and in addition to, the ordinary reformatting Ego always
applies to SQL text before running it (upper-casing keywords, quoting identifiers the way
the target provider requires). That reformatting never changes what a statement means.
The translations below sometimes do — they change _how_ something is spelled so the same
outcome still happens on the provider actually running it.

### What translates automatically

**Generated (auto-incrementing) primary keys.** Write it either way; Ego rewrites to
match the DSN:

- Toward SQLite: PostgreSQL's `SERIAL`, `BIGSERIAL`, and `SMALLSERIAL` pseudo-types, and
  both forms of `GENERATED ... AS IDENTITY`, all become
  `INTEGER PRIMARY KEY AUTOINCREMENT`.
- Toward PostgreSQL: SQLite's `INTEGER PRIMARY KEY AUTOINCREMENT` becomes
  `INTEGER GENERATED BY DEFAULT AS IDENTITY` — the modern, SQL-standard replacement for
  `SERIAL`, chosen because it matches `AUTOINCREMENT`'s own behavior (an explicit inserted
  value is honored, not rejected).
- A `PRIMARY KEY` written as its own table-level constraint, rather than attached
  directly to the column, is still recognized. Targeting SQLite, it is moved down onto
  the column itself, since that is the only place `AUTOINCREMENT` is legal there.
- `GENERATED ... AS IDENTITY`'s optional parenthesized sequence-option list (for example
  `(START WITH 1 INCREMENT BY 1)`) is accepted and discarded; SQLite has no equivalent to
  translate it to.

**SQLite's `WITHOUT ROWID` table option.** Dropped when targeting PostgreSQL, which has
no rowid concept and no use for the hint. Left untouched targeting SQLite.

**SQLite's `INSERT OR ...` conflict-resolution shorthand**, when targeting PostgreSQL
(which has no such syntax at all):

- `INSERT OR IGNORE` becomes `INSERT ... ON CONFLICT DO NOTHING`.
- `INSERT OR REPLACE` becomes `INSERT ... ON CONFLICT (<key>) DO UPDATE SET ...`, using
  the target table's actual primary key or a single-column unique index as the conflict
  target, and updating every column named in the statement's own column list.
- `INSERT OR ABORT` is simply dropped — aborting on a constraint violation is already
  PostgreSQL's own default behavior for a plain `INSERT`, so there is nothing to add.

**PostgreSQL-style `INSERT ... ON CONFLICT (...) DO UPDATE` / `DO NOTHING` upserts.**
These need no translation in either direction — SQLite has accepted this exact syntax
since version 3.24, so it is left untouched whether the DSN is SQLite or PostgreSQL.

### What cannot be translated

A handful of constructs have no equivalent in the other dialect. Rather than guess or
silently drop the behavior you asked for, Ego rejects these with a `400` error whose
message begins `cannot translate SQL for the target database provider`:

| Construct | Why it fails |
| :--------- | :------------- |
| A generated-key column that is not also the table's own `PRIMARY KEY`, targeting SQLite | PostgreSQL allows an identity column that is not the primary key; SQLite has no way to auto-increment a column independent of it being the primary key |
| A generated-key column that is part of a composite (multi-column) `PRIMARY KEY`, targeting SQLite | SQLite's `AUTOINCREMENT` only works on a single-column `INTEGER PRIMARY KEY` |
| `INSERT OR REPLACE` with no explicit column list, targeting PostgreSQL | PostgreSQL's `ON CONFLICT ... DO UPDATE` needs to know which columns to overwrite, and there is nothing to build that list from without one |
| `INSERT OR REPLACE` against a table with no primary key and no single-column unique index, targeting PostgreSQL | There is no valid target for `ON CONFLICT` to name |
| `INSERT OR FAIL` or `INSERT OR ROLLBACK`, targeting PostgreSQL | Neither has a PostgreSQL equivalent: a failed statement inside a PostgreSQL transaction always poisons the whole transaction, with no per-statement "keep what already succeeded" option |

### One documented exception: `GENERATED ALWAYS AS IDENTITY` toward SQLite

This one _does_ translate, but not perfectly, and it is worth knowing about. PostgreSQL's
`GENERATED ALWAYS AS IDENTITY` rejects an explicit value on insert; SQLite's
`AUTOINCREMENT` does not enforce that — an explicit value is honored. Since SQLite has no
way to express "always, no override," a column declared `GENERATED ALWAYS AS IDENTITY`
and targeting SQLite still becomes ordinary `AUTOINCREMENT`, the closest available
behavior. This is a deliberate choice, not an oversight: the schema still works, at the
cost of losing the `ALWAYS` enforcement once the underlying DSN is SQLite.

### Seeing what actually ran

Rewriting is silent to the caller — the SQL you sent runs, whichever provider is behind
the DSN, with no error or warning about the rewrite itself. When SQL-class server logging
is enabled, both a short note of what changed (and why) and the final, rewritten statement
that was actually executed are written to the server log, so you can always see exactly
what ran against the database.

&nbsp;
&nbsp;
