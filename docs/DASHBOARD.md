# Ego Server Dashboard

The _Ego_ server includes a built-in web dashboard that lets you monitor and manage a running
server instance from any modern web browser. No additional software is required; the dashboard
is served directly by the _Ego_ server.

&nbsp;

## Accessing the Dashboard

Point your browser at the server's hostname and port, followed by `/ui`:

```text
http://localhost:8080/ui
```

Replace `localhost:8080` with the actual host and port of your _Ego_ server. If the server
was started with TLS enabled, use `https://` instead.

&nbsp;

## Logging In

When the dashboard first loads it shows a **Sign In** overlay. Enter the username and password
for an account that has been configured on the server.

Once authenticated, the dashboard stores a bearer token in memory for the current browser
session. If the _Remember login_ setting is enabled (see [Settings](#settings) below), the
token is also written to a browser cookie that expires after 24 hours, so the login survives
a page refresh or a new tab opened to the same server.

If you remain idle, the dashboard automatically signs you out and re-displays the login
overlay. The idle timeout is set by the server (the `ego.server.dashboard.inactivity`
setting) and sent to the dashboard as part of a successful login; **15 minutes** is only the
default the dashboard falls back to if that value is ever unavailable, so the real timeout on
a given server may be shorter or longer.

### Passkey Login (FaceID / TouchID)

If the server is configured with `ego.server.allow.passkeys = true` and you are using a
browser that supports platform authenticators (Safari on macOS/iOS, Chrome on macOS, etc.),
the Sign In overlay shows a **Sign in with FaceID/TouchID** button beneath the standard
username/password fields. Click that button to authenticate with a previously registered
passkey instead of typing a password.

After a successful username/password login, if the current browser supports passkeys and you
have not previously dismissed the prompt, you are offered the chance to store a passkey for
future logins. This offer is not conditioned on whether a passkey already exists for your
account anywhere else — it is based only on whether this particular browser has been told to
stop asking. Two different buttons dismiss the prompt with different persistence: **Not Now**
skips it for this session only, while **Don't Ask Again** suppresses it in this browser for 90
days.

&nbsp;

## Header Bar

The top of every page is a simple strip containing just the Ego logo (left) and the hamburger
menu button (right). Server identity — hostname, version, instance UUID, and start time — no
longer lives in the header; it has moved to the **Server Info** sheet, opened from the Status
tab's toolbar (see [Status Tab](#status-tab) below).

&nbsp;

## Hamburger Menu

Click the hamburger button (☰) in the top-right corner of the header to open the menu. When you
are signed in, the dropdown's first line reads **Logging in as _username_**, followed by a
divider and then the menu items:

| Item | Action |
| :--- | :--- |
| **⚙ Help…** | Opens the online documentation in a new browser tab |
| **⚙ Settings** | Opens the [Settings](#settings) sheet |
| **✕ Log Out** | Immediately ends your session and returns to the login overlay |

&nbsp;

## Settings

The Settings sheet slides in from the right when you choose **Settings** from the hamburger
menu. It groups six settings under three headings:

### Appearance

| Setting | Description |
| :--- | :--- |
| **Dark mode** | A three-way switch — **Auto**, **On**, **Off** — rather than a simple toggle. **Auto** follows your browser/OS color-scheme preference and switches live if that preference changes while the dashboard is open. The Code tab always uses its own dark theme regardless of this setting. |
| **Use Text Buttons** | When enabled (the default), toolbar buttons across every tab show an icon plus a text label. Turn it off to show icons only, for a more compact toolbar. |

### Logins

| Setting | Description |
| :--- | :--- |
| **Remember login** | When enabled, the session token is saved to a browser cookie so a page refresh does not require you to sign in again. The cookie expires after 24 hours. |
| **Use passkeys** | Allow passkey (biometric / hardware key) login and registration. Turn off to use passwords only, even if the server supports passkeys. |

### Code

| Setting | Description |
| :--- | :--- |
| **Format** | When enabled, the Code tab's editor is automatically reformatted before every Run or Debug, and the SQL tab's editor before every Submit. Off by default. |
| **Console** | Shows or hides the Code tab's Console (REPL) panel. On by default. |

All settings are stored as browser cookies and persist across sessions.

Click **Close** to dismiss the sheet.

&nbsp;

## Tabs

The dashboard is organized into eight tabs. Click a tab name to switch to it; the last active
tab is remembered between page loads.

| Tab | Description |
| :--- | :--- |
| [Status](#status-tab) | Server metrics and cache summary |
| [Users](#users-tab) | User account management |
| [DSNs](#dsns-tab) | Create, manage, and set permissions on database connections |
| [Tables](#tables-tab) | Browse tables in a DSN |
| [Data](#data-tab) | Browse and edit table rows |
| [SQL](#sql-tab) | Interactive SQL editor and builder |
| [Code](#code-tab) | _Ego_ code editor, debugger, and REPL |
| [Log](#log-tab) | Server log viewer and logger configuration |

Most tabs (Status, Users, DSNs, Tables, Data, and Log) are hidden entirely unless the
logged-in user has the `ego.root` permission. Two tabs are also available individually to a
non-admin user: Code to anyone with the `ego.code` permission, and SQL to anyone with the
`ego.sql` permission. A non-admin lands automatically on whichever of Code or SQL their
permissions unlock after signing in (Code if they hold both), and sees only the tab(s) their
permission(s) grant.

&nbsp;

### Status Tab

The Status tab shows two compact three-column grids: **Metrics** (Go runtime statistics) and
**Cache Status** (server cache summary). Both grids refresh each time you open the tab.

```text
┌────────────────────────────────────────────────────────────────────┐
│  ↺ Refresh   🗑 Flush Caches   ℹ Server Info                       │
├────────────────────────────────────────────────────────────────────┤
│  METRICS                                                           │
│                                                                    │
│  Uptime           2h 28m   Objects in Use      16,698   App Mem   …│
│  Requests Proc.      100   Heap Memory       36.69 MB   Stack Mem …│
│  GC Cycles            90   Goroutines             42                │
├────────────────────────────────────────────────────────────────────┤
│  CACHE STATUS                                                      │
│                                                                    │
│  DSN Entries           0   Cached Services        3   Authorizations│
│  Schema Entries        0   Service Cache Size  20 items   Tokens   │
│  Code Run Sessions     0   Cached Assets          2   Blacklist    │
│  Code Debug Sessions   0   Asset Cache size    24 KB               │
├────────────────────────────────────────────────────────────────────┤
│  Cached Endpoints  │ Class │ Reuse count │ Size │ Last accessed    │
│  …                                                                 │
└────────────────────────────────────────────────────────────────────┘
```

**Metrics fields:**

| Field | Description |
| :--- | :--- |
| Uptime | Time elapsed since the server started |
| Requests Processed | Total HTTP requests handled since startup |
| GC Cycles | Number of garbage-collection cycles completed |
| Objects in Use | Number of live heap objects |
| Heap Memory | Memory currently allocated on the heap |
| Stack Memory | Memory currently in use by goroutine stacks |
| Application Memory | Total memory obtained from the operating system |
| Goroutines | Number of active Go goroutines in the server process |

**Cache Status fields:**

| Field | Description |
| :--- | :--- |
| DSN Entries | Cached database connection descriptors |
| Schema Entries | Cached table-schema descriptions |
| Code Run Sessions | Active code-execution sessions (Admin → Run) |
| Code Debug Sessions | Active debugger sessions |
| Cached Services | Compiled _Ego_ service endpoints held in memory |
| Service Cache Size | Maximum number of service entries the cache holds |
| Cached Assets | Static files (HTML, CSS, JS) held in the asset cache |
| Asset Cache size | Total bytes occupied by the asset cache |
| Authorizations | Cached access-control decisions |
| Tokens | Active bearer tokens in the token cache |
| Blacklist Status | Tokens that have been explicitly invalidated |

Below the summary grids, a **Cached Endpoints** table lists each individual cached item with
its endpoint name, class (service or asset), reuse count, size, and last-access time.

**Toolbar buttons:**

| Button | Action |
| :--- | :--- |
| ↺ Refresh | Reloads metrics and cache data from the server |
| 🗑 Flush Caches | Deletes all cached items, forcing the server to recompile services and reload assets on the next request |
| ℹ Server Info | Opens the [Server Info sheet](#server-info-sheet) |

&nbsp;

#### Server Info Sheet

Click **ℹ Server Info** to open a read-only sheet with three parts:

1. **Server identity** — the block of information that used to sit in the page header:
   Host Name, Ego Version, Server UUID, and Started (with a live-computed uptime alongside
   the timestamp, e.g. `Mon Jan 2 15:04:05 MST 2006 (up 2h 28m)`).
2. **Host machine** — details about the machine the server process is running on: Platform
   (e.g. `macOS 14.5`), Architecture, CPU Cores, Total Memory, and Available Memory. This
   section is best-effort: if the underlying host query fails or is unavailable, it is
   simply omitted and the rest of the sheet still works.
3. **Setting / Value table** — every server configuration key and its current value, sorted
   alphabetically, exactly as in earlier versions of this sheet. Each row is now clickable:
   clicking a setting opens a small popup showing its full name, current value, and a
   description of what it controls.

> **Permission required:** `ego.root`

&nbsp;

### Users Tab

The Users tab lists every user account on the server and lets you create, edit, and delete
accounts.

**User list columns:**

| Column | Description |
| :--- | :--- |
| User | The login name for the account |
| ID | An internal identifier assigned by the server |
| Permissions | Comma-separated list of capabilities granted to this user (e.g. `ego.logon, ego.root`) |
| Passkeys | Number of passkeys registered for this account |
| Last Login | Date and time of the account's most recent successful token issuance, or a dash if it has never logged in |

**Creating a user** — click **New User** to open the creation sheet:

1. Enter a **Username**.
2. Enter a **Password**.
3. Enter one or more **Permissions**, separated by commas.
4. Click **Save**.

**Editing a user** — click any row in the table to open the edit sheet:

* The username is shown but cannot be changed here.
* Enter a **New password** to change the password, or leave the field blank to keep the
  current password.
* Edit the **Permissions** field as needed.
* The sheet also shows the user's **passkey count** and **last login** time (read-only) —
  the same values already visible in the list, repeated here for convenience.
* Click **Save** to apply changes, or **Delete** to remove the account entirely.

When passkeys are enabled, the edit sheet also shows passkey buttons:

| Button | Action |
| :--- | :--- |
| **+ Passkey** | Register a new passkey for this account using the browser's platform authenticator. |
| **- Passkey** | Remove all passkeys stored for this account. Available to the account owner and to administrators. |

Common permission values:

| Permission | Grants |
| :--- | :--- |
| `ego.logon` | Ability to authenticate (required for all interactive use) |
| `ego.root` | Full administrative access: users, loggers, caches, memory stats |
| `ego.code` | Ability to execute arbitrary _Ego_ code in the Code tab |
| `ego.sql` | Ability to run SQL statements in the SQL tab, subject to that user's table_perms grants and (for schema changes) `ego.dsn.admin` |
| `ego.dsn.admin` | Ability to manage data source connections |

> **Permission required:** `ego.root`

&nbsp;

### DSNs Tab

DSN stands for _Data Source Name_ — a named connection descriptor that tells the server how
to connect to a database. The DSNs tab lists every DSN configured on the server, and lets you
create, delete, and manage per-user permissions on them directly from the dashboard.

**Columns:**

| Column | Description |
| :--- | :--- |
| Name | The identifier used to reference this connection in _Ego_ programs and REST requests |
| Provider | Database engine: `sqlite`, `postgres`, etc. |
| Database | Name of the database (or file path for SQLite) |
| Host | Hostname of the database server (blank for SQLite) |
| Port | TCP port (blank for SQLite) |
| User | Database login username |
| Secured | `Yes` if _Ego_'s own per-table permission checks are enforced for this DSN's tables. When `No`, any user who can reach the DSN can perform any operation on any of its tables. |
| Restricted | `Yes` if using this DSN at all requires an explicit per-user grant (see [Permissions](#dsn-permissions) below). When `No`, any authenticated user can use the DSN. |

**Creating a DSN** — click **New DSN** to open the creation sheet:

1. Enter a **Name**.
2. Choose a **Provider**: **Postgres** or **Sqlite3**.
3. Fill in **Host**, **Port**, **Database**, **Schema**, and **User** as appropriate for the
   provider (SQLite connections typically leave Host and Port blank).
4. Check **Secured** to enable per-table permission checks, and/or **Restricted** to require
   an explicit per-user grant before the DSN can be used at all.
5. **Row ID** is checked by default — leave it checked so the server adds its automatic
   `_row_id_` column to tables created through this DSN, which most dashboard features
   (row editing, the SQL Build wizard) rely on to identify individual rows.
6. Click **Save**.

**Viewing and managing a DSN** — click any row in the table to open the detail sheet:

* All of the DSN's attributes are shown in a read-only table.
* If the DSN is **Restricted**, a **Permissions** section lists every user who has been
  granted access, with their comma-separated permission list. Click a user row to edit or
  remove their permissions.
* Click **Show tables…** to jump to the [Tables tab](#tables-tab) with this DSN pre-selected.
* Click **Add permission…** to grant a new user access (see below).
* Click **Delete** to remove the DSN entirely, or **Close** to dismiss the sheet.

<a id="dsn-permissions"></a>**Managing permissions** — from the detail sheet:

* **Add permission…** opens a sheet with a **User** field and a comma-separated
  **Permissions** field; click **Save** to grant them.
* Clicking an existing user row in the Permissions list opens an edit sheet pre-filled with
  their current permissions. Change the **Permissions** field and click **Save**, or click
  **Delete** to revoke all of that user's permissions on this DSN.

> **Permission required:** `ego.root`

&nbsp;

### Tables Tab

The Tables tab lets you browse the database tables available through a DSN.

1. Select a **DSN** from the dropdown at the top of the tab, or click **↺ Refresh** to reload
   the table list for the currently selected DSN.
2. The table list shows the **name**, **schema**, **column count**, and **row count** for
   each table.
3. Click any table row to open a **detail sheet** listing each column's name, data type,
   size, and whether it is nullable or must contain a unique value.
4. From the detail sheet, click **View Data** to jump straight to the [Data tab](#data-tab)
   with this DSN and table pre-selected, or **Close** to dismiss the sheet.

> **Permission required:** access to the selected DSN

&nbsp;

### Data Tab

The Data tab lets you browse and edit the rows stored in a database table.

**Selecting data:**

1. Choose a **DSN** from the first dropdown.
2. Choose a **Table** from the second dropdown (populated automatically when a DSN is
   selected).
3. The rows of the selected table are loaded and displayed. Click **↺ Refresh** at any time
   to reload just the row data without re-selecting the DSN or table.

**Reading the data grid:**

* Each column in the table becomes a column in the grid.
* If the table's unique key is the server's internal `_row_id_` column, the grid adds its own
  **Row ID** column for it, since `_row_id_` is otherwise excluded from the regular data
  columns.
* Numeric columns (`int`, `float`, and related types) are right-aligned.
* Fields that contain no value are shown as `null` in italic grey text.
* Float values always display a decimal point (e.g. `42.0`) to distinguish them from
  integers.
* A **row count** summary is shown below the grid.

**Choosing visible columns** — click **Columns…** to open a picker sheet:

* Toggle individual columns on or off using the checkboxes.
* Click **Select all** to make every column visible again.
* The selection resets automatically when you switch to a different DSN or table.

**Opening the data as SQL** — click **SQL…** to switch to the [SQL tab](#sql-tab) with a
`SELECT` statement pre-filled for the current table, scoped to whichever columns are
currently visible (or `SELECT *` if every column is shown).

**Editing a row** — click any row in the grid to open an edit sheet (titled **Edit Row**, or
**Row Contents** if the row has no usable key):

* All fields for that row are shown. The column that uniquely identifies the row (its unique
  key, or `_row_id_` if the table has one) is marked with 🔑 and cannot be edited.
* Each editable field has its own **Null** button beside it, to explicitly set that field to
  SQL `NULL` rather than an empty string.
* Modify field values and click **Save** to write changes back to the database. The **Save**
  button stays disabled until at least one field actually differs from its original value.
* Click **Delete** to remove the row (only available for rows that have a usable key).
* Click **Cancel** to close the sheet without changes.
* If a row has no internal row ID or other unique key, the sheet shows a message indicating
  that the row cannot be modified through the dashboard, and both **Save** and **Delete**
  stay disabled.

> **Permission required:** access to the selected DSN and table

&nbsp;

### SQL Tab

The SQL tab provides an interactive SQL environment for running queries and modifying data
in any DSN connected to the server. It includes a syntax-highlighted editor, a statement
preprocessor, and a point-and-click wizard for building common SQL statements.

&nbsp;

#### Toolbar

| Control | Description |
| :--- | :--- |
| **DSN** picker | Selects the database connection that all statements in the editor will run against. |
| **✕ Clear** | Clears the editor contents and any previous results. |
| **🔨 Build** | Opens the SQL Build wizard to construct a statement interactively. |
| **≡ Format** | Reformats the SQL currently in the editor. If the [Format setting](#settings) is enabled, this also happens automatically before every Submit. |
| **▶ Submit** | Executes all statements in the editor. Keyboard shortcut: **Ctrl+Enter** (or **Cmd+Enter** on macOS). |
| **📂 Open** | Opens a file picker to load a `.sql` or `.txt` file from your local disk into the editor. |
| **💾 Save** | Saves the current editor contents to a file. On Chrome and Edge the browser shows a native Save dialog; on other browsers the file is downloaded to the default Downloads folder. |

&nbsp;

#### Writing SQL

Type or paste one or more SQL statements into the editor. The editor highlights SQL keywords,
type names, string literals, numeric literals, and comments as you type.

**Multiple statements** are separated by semicolons. Each statement can span multiple lines;
the preprocessor joins continuation lines automatically before sending them to the server.

**Comments** — `--`, `//`, and `#` all introduce a line comment, and `/* ... */` block
comments are also recognized; none of these need to be the first characters on the line, so
trailing/inline comments are stripped too. This lets you annotate your queries without
affecting what the server sees:

```sql
-- Fetch recent orders for reporting
SELECT order_id, customer, total   -- only open orders
FROM orders
WHERE created_at > '2024-01-01'
ORDER BY created_at DESC;
```

**DSN hint** — a comment anywhere in the editor that combines the word `dsn` with a
connection name (in either order, e.g. `-- production dsn` or `-- dsn production`) switches
the DSN picker to that connection whenever you press Enter anywhere in the editor, and again
whenever a file is loaded with **Open**:

```sql
-- production dsn
SELECT count(*) FROM customers;
```

This is convenient for saved query files that are always intended to run against a specific
database. If the editor contains hints for more than one distinct DSN name, no switch
happens — resolve the ambiguity by removing the extra hint.

**Results** appear below the editor:

* A `SELECT` that returns rows is shown as a scrollable table. The internal `_row_id_` column
  is always hidden.
* Any other statement (INSERT, UPDATE, DELETE, CREATE, etc.) shows the number of rows
  affected.
* Errors are shown in red.

> **Note:** When multiple statements are submitted together, only the last statement's rows
> are returned. A warning banner is shown if a `SELECT` that is not the last statement is
> detected, because its results will be discarded.

&nbsp;

#### SQL Build Wizard

Click **🔨 Build** to open the SQL Build wizard. The wizard slides in from the right and
guides you through building a complete SQL statement without typing.

1. Choose a **Statement type** using the button bar at the top of the wizard. Row operations
   (SELECT, INSERT, UPDATE, DELETE) appear on the first row; schema operations (ALTER TABLE,
   CREATE TABLE) appear on the second row. The active type is highlighted in blue.
2. Fill in the fields for that statement type (described below).
3. The generated SQL appears in the preview pane at the bottom of the wizard as you make
   selections — it updates live with every change.
4. Click **Insert** to append the generated statement to the editor at the current cursor
   position, then close the wizard.
5. Click **Cancel** to close the wizard without inserting anything.

**Pre-loading from a selection** — if you select text in the SQL editor _before_ clicking
**🔨 Build**, the wizard attempts to parse the selected text as a SQL statement and
pre-populate all fields automatically. The following statement types can be pre-loaded:
SELECT, INSERT, UPDATE, DELETE, CREATE TABLE, and all three forms of ALTER TABLE (Add
Column, Drop Column, Rename Column). If the selected text cannot be parsed — for example
because it contains JOINs, subqueries, or OR conditions that the wizard does not support —
you are asked whether to open a blank wizard instead. When the wizard is pre-loaded from a
selection, clicking **Insert** replaces the original selection with the rebuilt statement
rather than appending it at the cursor.

&nbsp;

##### SELECT

Build a `SELECT … FROM … WHERE … ORDER BY` query.

| Section | Description |
| :--- | :--- |
| **Table** | Pick the table to query from the dropdown. |
| **Columns** | Check **Select all columns (\*)** to use `SELECT *`, or uncheck it to choose individual columns from the grid. |
| **WHERE clause** | Click **+ Add condition** to add a filter row. Each row has a column picker, an operator (`=`, `<>`, `<`, `<=`, `>`, `>=`, `IS NULL`, `IS NOT NULL`, `LIKE`, `NOT LIKE`), and a value field. Multiple conditions are combined with `AND`. Remove a condition with the **✕** button. |
| **ORDER BY** | Click **+ Add column** to add a sort row. Each row has a column picker and an `ASC`/`DESC` direction. Multiple sort columns are listed in order. |

**Example output:**

```sql
SELECT order_id, customer, total
FROM orders
WHERE status = 'open'
  AND total >= 100
ORDER BY created_at DESC
```

&nbsp;

##### INSERT

Build an `INSERT INTO … VALUES (…)` statement.

| Section | Description |
| :--- | :--- |
| **Table** | Pick the target table. |
| **Values** | One row per column. Each row shows the column name, its SQL type, a value input, and a **Null** button. Type the value to insert; click **Null** to insert a SQL `NULL` instead. The internal `_row_id_` column is not shown — it is assigned automatically by the database. |

String values are automatically single-quoted in the preview; numeric values are left
unquoted. A bullet (•) after the type hint indicates the column does not allow `NULL`.

**Example output:**

```sql
INSERT INTO customers
  (name, email, active)
VALUES
  ('Alice', 'alice@example.com', 1)
```

&nbsp;

##### UPDATE

Build an `UPDATE … SET … WHERE …` statement.

| Section | Description |
| :--- | :--- |
| **Table** | Pick the target table. |
| **SET values** | One row per column. Each row starts unchecked and dimmed. Check the box next to a column to include it in the `SET` clause and enable its value input. Click **Null** to set the column to `NULL`. |
| **WHERE** | Works the same as the SELECT WHERE section. A unique key column (or `_row_id_`) is pre-populated when the table is selected. |

If you click **Insert** without any WHERE conditions, the wizard shows a confirmation dialog
warning that the statement will update **all rows** in the table. If you confirm, the
statement is inserted with a warning comment prepended:

```sql
// WARNING: this statement affects all rows
UPDATE products
SET price = 9.99
```

&nbsp;

##### DELETE

Build a `DELETE FROM … WHERE …` statement.

| Section | Description |
| :--- | :--- |
| **Table** | Pick the target table. |
| **WHERE** | Works the same as the UPDATE WHERE section. A unique key column (or `_row_id_`) is pre-populated when the table is selected to help prevent accidental bulk deletes. |

The same no-WHERE confirmation dialog applies: attempting to insert a DELETE without any
WHERE conditions prompts you to confirm before inserting the statement with a warning comment.

**Example output:**

```sql
DELETE FROM orders
WHERE order_id = 1042
```

&nbsp;

##### CREATE TABLE

Build a `CREATE TABLE (…)` statement to define a new table in the selected DSN.

| Section | Description |
| :--- | :--- |
| **Table name** | Type the name of the new table. An inline indicator shows ✔ when the name is available or ✘ when a table with that name already exists in the DSN. The Insert button is disabled until a valid, unique name is entered. |
| **Columns** | Click **+ Add column** to add a column definition row. Each row has a **Name** input, a **Type** dropdown, a **Unique** checkbox, and a **Nullable** checkbox (checked by default). Remove a column with the **✕** button. At least one column is required. |

If the selected DSN has Row ID support enabled, the wizard automatically adds a locked
`_row_id_` column (marked with 🔒) as the first row — it cannot be edited or removed, and
overrides any `_row_id_` column that a pre-loaded `CREATE TABLE` statement may have defined.

The **Type** dropdown offers the most commonly used SQL data types:

```text
VARCHAR  TEXT     CHAR      INT      INTEGER  BIGINT   SMALLINT
FLOAT    DOUBLE   DECIMAL   NUMERIC  BOOLEAN
DATE     DATETIME TIMESTAMP UUID     JSON
```

Column constraints are added in this order: `NOT NULL` (when Nullable is unchecked), then
`UNIQUE` (when Unique is checked).

**Example output:**

```sql
CREATE TABLE customers (
    id INTEGER NOT NULL UNIQUE,
    name VARCHAR NOT NULL,
    email VARCHAR,
    active BOOLEAN NOT NULL
)
```

&nbsp;

##### ALTER TABLE

Build an `ALTER TABLE` statement to modify the structure of an existing table. Select the
table to alter from the **Table** picker, then choose one of the three operations using the
**Columns** button bar:

| Operation | Effect |
| :--- | :--- |
| **Add** | Adds one or more new columns to the table |
| **Drop** | Removes one or more existing columns from the table |
| **Rename** | Renames one or more existing columns |

&nbsp;

###### Add Column

Fill in the column definition fields, which are identical to those in the CREATE TABLE wizard:

| Field | Description |
| :--- | :--- |
| **Name** | The name for the new column |
| **Type** | SQL data type (same choices as CREATE TABLE) |
| **Unique** | When checked, adds a `UNIQUE` constraint |
| **Nullable** | When unchecked, adds a `NOT NULL` constraint |

For **Postgres** connections, click **+ Add another** to define multiple columns at once —
the wizard generates a single `ALTER TABLE` statement with comma-separated `ADD COLUMN`
clauses. For **SQLite** connections, only one column can be added per statement, so the
**+ Add another** button is not shown.

Example output (Postgres, two columns):

```sql
ALTER TABLE orders
  ADD COLUMN shipped_at TIMESTAMP,
  ADD COLUMN carrier VARCHAR
```

Example output (SQLite, single column):

```sql
ALTER TABLE orders ADD COLUMN notes TEXT
```

&nbsp;

###### Drop Column

The wizard displays all existing columns. Select the column(s) to remove:

* **Postgres** — checkboxes allow selecting multiple columns. The generated statement drops
  all selected columns in one `ALTER TABLE` statement with comma-separated `DROP COLUMN`
  clauses.
* **SQLite** — radio buttons are used instead of checkboxes because SQLite supports only one
  `DROP COLUMN` per statement.

Example output (Postgres, two columns):

```sql
ALTER TABLE orders
  DROP COLUMN shipped_at,
  DROP COLUMN carrier
```

Example output (SQLite, single column):

```sql
ALTER TABLE orders DROP COLUMN notes
```

&nbsp;

###### Rename Column

Every existing column in the table is listed with a `→` arrow and a text input for the new
name. Leave the input blank to keep a column's current name. At least one new name must be
entered before the **Insert** button is enabled.

Each rename generates a separate `ALTER TABLE … RENAME COLUMN … TO …` statement, because
neither Postgres nor SQLite supports renaming multiple columns in one `ALTER TABLE` statement.

Example output:

```sql
ALTER TABLE customers RENAME COLUMN name TO full_name
ALTER TABLE customers RENAME COLUMN email TO email_address
```

&nbsp;

> **Schema refresh** — after any SQL submission that includes an `ALTER TABLE` statement
> succeeds, the dashboard automatically re-fetches the column metadata for the table
> currently open in the Data tab, keeping the schema view in sync with the database.

&nbsp;

> **Permission required:** `ego.sql` (or `ego.root`) to use the tab at all. A non-admin
> `ego.sql` user is further limited per statement: `ego.table.read` to `SELECT` a table,
> `ego.table.write` to `INSERT`/`UPDATE`/`DELETE` it, and `ego.dsn.admin` for schema changes
> (`CREATE`/`ALTER`/`DROP TABLE`, `CREATE`/`DROP INDEX`, `CREATE`/`DROP VIEW`).

&nbsp;

### Code Tab

The Code tab is an interactive development environment that lets you write, run, and
debug _Ego_ programs directly in the browser.

&nbsp;

#### Layout

```text
┌──────────────────────────────────────────────────────────────────┐
│  [▶ Run ▾]  [≡ Format]  (spinner)                                │
├────────────────────────────────────────┬─────────────────────────┤
│  Editor      [📂 Open] [💾 Save] [✕]   │  Output   (elapsed) [💾][✕]│
│                                        │                         │
│  (syntax-highlighted source)           │  (program output)       │
│                                        │                         │
│                                        ├─────────────────────────┤
│                                        │  Debugger (debug only)  │
│                                        │  [Go][Step][Over][Ret]  │
│  (line numbers on left edge)           │  (debugger output)      │
│                                        │  debug>  [input] [Send] │
├────────────────────────────────────────┴─────────────────────────┤
│  Console                                                    [✕]  │
│  (history of REPL interactions)                                  │
│  ego> _                                                          │
└──────────────────────────────────────────────────────────────────┘
```

The vertical divider between the editor and the right pane, and the horizontal divider
above the console, can both be dragged to resize the panes. The Console pane itself is shown
or hidden by the **Console** setting in [Settings](#settings), not by a toolbar button.

&nbsp;

#### Code Toolbar

The toolbar runs across the top of the Code tab:

| Control | Description |
| :--- | :--- |
| **▶ Run** (main button) | Compiles and executes the program in the current run mode. |
| **▾** (dropdown arrow) | Opens a menu to choose the run mode: **▶ Run**, **🐛 Debug**, or **👣 Trace**. The mode is remembered until changed. In Trace mode, a normal run executes with the server's instruction-by-instruction virtual-machine trace streamed into the Output pane alongside the program's own output — Trace mode replaced what used to be a separate toggle button. |
| **≡ Format** | Reformats the editor's source on demand, regardless of the Format setting below. |
| _(spinner)_ | Animated spinner visible while the server is processing a request. |

&nbsp;

#### Editor Pane

The left pane contains the code editor:

* Type or paste _Ego_ source code. Syntax highlighting updates as you type.
* Line numbers are shown on the left edge.
* The current debug line is highlighted with a colored band during a debug session.

The editor pane's label bar contains three inline buttons:

| Button | Action |
| :--- | :--- |
| **📂 Open** | Opens a file picker to load an `.ego` file from your local disk into the editor. |
| **💾 Save** | Saves the current editor contents to a `.ego` file. |
| **✕** | Clears the editor contents. |

**Ctrl+Enter** (or **Cmd+Enter** on macOS) runs the code without reaching for the mouse.

&nbsp;

#### Output Pane

The right pane shows the output of the most recent run:

* Output from `fmt.Print`, `fmt.Println`, and similar calls appears here.
* Compiler errors and runtime errors are highlighted in red.
* After a run completes, the elapsed execution time is displayed in the pane label bar.
* **💾** in the label bar saves the current output to a `.txt` file; **✕** clears it.

&nbsp;

#### Formatting Code

Click **≡ Format** in the toolbar at any time to send the editor's current source to the
server's AST-based _Ego_ formatter (the same kind of reformatting the SQL tab's Format button
does for SQL, but this one understands _Ego_ syntax) and replace the editor contents with the
result.

If you would rather this happen automatically, enable the **Format** setting in
[Settings](#settings): with it on, the editor is reformatted before every **Run** or **Debug**
(Trace mode included, since it runs the same way as Run). The setting is off by default, so
existing formatting is left alone unless you opt in.

&nbsp;

#### Running Code

Click **▶ Run** (or press **Ctrl+Enter** in the editor) to compile and execute the current
program. The button is disabled and a spinner appears while the server processes the request.

**How execution works:**

* If the editor contains a function named `main` — declared as `func main()` — that
  function is called automatically. This lets you structure your code the way a real Go
  program would be structured, with helper functions and a clear entry point.
* If there is no `func main()`, every top-level statement in the editor is executed in
  order, as a script.

&nbsp;

#### Debug Mode

Select **🐛 Debug** from the **▾** dropdown to run the program under the interactive
debugger. The right pane shows both the **Output** section and the **Debugger** section
below it.

The Debugger section contains:

* **Stepping buttons** in the label bar — shortcuts for the most common commands:

  | Button | Debugger command |
  | :--- | :--- |
  | **Go** | `continue` — resume execution until the next breakpoint or end of program |
  | **Step** | `step` — execute one statement, stepping _into_ function calls |
  | **Step Over** | `step over` — execute one statement, stepping _over_ function calls |
  | **Step Return** | `step return` — run until the current function returns |

* **Debugger output** — messages from the debugger (breakpoint hits, variable values, etc.)
* A **`debug>`** command input where you can type any debugger command and press **Send**
  (or Enter) to execute it.
* A **✕** button to clear the debugger output.

Type `help` at the `debug>` prompt to display the full command reference:

| Command | Description |
| :--- | :--- |
| `break at <line>` | Halt execution when the given line is reached |
| `break when <expression>` | Halt execution when an _Ego_ expression evaluates to true |
| `break clear at <line>` | Remove the breakpoint at the given line |
| `break clear when <expression>` | Remove the conditional breakpoint for the given expression |
| `break load ["file"]` | Restore breakpoints from a previously saved file |
| `break save ["file"]` | Save the current breakpoint list to a file |
| `continue` | Resume execution until the next breakpoint or program end |
| `exit` | End the debug session |
| `help` | Display this command reference |
| `print <expression>` | Print the value of any _Ego_ expression |
| `set <variable> = <expression>` | Assign a new value to a variable while paused |
| `show breaks` | List all active breakpoints |
| `show calls [<n>]` | Display the call stack to a given depth |
| `show line` | Show the source line currently being executed |
| `show package <name> [<name> ...]` | Display exported constants, types, and functions for one or more packages |
| `show scope` | Display the nested call scope and symbol table chain |
| `show source [<start>[:<end>]]` | Display source lines from the current module |
| `show symbols` | Display all variables in the current scope |
| `step [into]` | Execute one statement, stepping into any function call |
| `step over` | Execute one statement, stepping over function calls |
| `step return` | Run until the current function returns |

The debug session ends automatically when the program finishes, or when you send `exit`
or click the **✕** button to clear debugger output.

&nbsp;

#### Console Pane (REPL)

The console at the bottom of the tab provides a read-eval-print loop (REPL). It can be
shown or hidden with the **Console** toggle in [Settings](#settings); the preference persists
across sessions and is applied as soon as the page loads.

Type a single _Ego_ statement at the `ego>` prompt and press **Enter** to execute it
immediately. The result or any output appears directly in the console history above the
prompt. The **✕** button clears the console history.

The key difference between the editor and the console:

| | Editor | Console |
| :--- | :--- | :--- |
| Execution | Runs the entire program (or calls `func main()`) on each **Run** | Executes one statement at a time as you type |
| Symbol table | Fresh on every **Run** — variables from one run are gone in the next | **Persistent** across statements — variables declared in earlier statements remain available |
| Use case | Writing and testing complete programs | Exploratory, incremental work; quick calculations |

The persistent symbol table for the console is stored on the server and is tied to the
specific browser tab (identified by a UUID generated when the page loads). Symbol tables
for inactive sessions are automatically cleaned up by the server after one hour of
inactivity.

> **Permission required:** `ego.code`

&nbsp;

### Log Tab

The Log tab displays the server's log output and lets you configure which categories of
messages are written to the log.

**Viewing the log:**

* The log viewer shows the most recent lines from the server log file (default: 500 lines).
* Use the scrollbar, or the ↑ and ↓ buttons, to move through the entries.

**Toolbar buttons:**

The toolbar uses icons rather than words. Hover over any button to see what it does.

| Button | Action |
| :--- | :--- |
| ↺ | Reloads the log from the server |
| ↑ | Scrolls to the beginning of the log |
| ↓ | Scrolls to the end of the log |
| Funnel | Opens the Log Filter sheet. A dot on the funnel means a filter is in force |
| Magnifier | Searches the lines currently displayed |
| ‹ _n_ / _m_ › | Steps between search matches, showing the position of the current one |
| Gear | Opens the Logger Configuration sheet |

**Searching versus filtering:**

These are two different things, and the difference matters:

* **Searching** looks through the lines already displayed, highlighting matches. It never
  contacts the server.
* **Filtering** changes which lines the server sends in the first place. Filtering must
  happen on the server because the log is structured there — each line records a session
  number, a logging class, and a message identifier separately — and those fields are
  combined into a single readable sentence before the line is sent.

**Searching the log:**

1. Type a search term in the search box.
2. Click the magnifier (or press Enter) to highlight all matches and jump to the first.
3. Use ‹ and › to step between matches. The count between them shows your position,
   for example `3 / 18`.
4. Click **✕** inside the search box, or press Escape, to clear the search.

**Log Filter sheet:**

Opened with the funnel. Changing any of these causes the log to be re-requested from the
server. The settings are remembered between visits.

* **Limit results** — the most lines to return, counting back from the newest. The limit is
  applied _after_ the filters below, so asking for 50 lines of class `REST` returns 50 `REST`
  lines if the log holds that many, rather than however many happen to fall within the last
  50 lines of the file. The limit applies even when no filter is set.
* **Session number** — returns only messages logged while the server handled one particular
  endpoint request. Every request is assigned a session number, which appears in the log line
  as `[7]`. Leave the field empty to include every session.
* **Message identifier** — a wildcard pattern matched against the message's internal
  identifier, such as `log.server.request`. Use `*` for any run of characters and `?` for a
  single character; matching ignores case. The pattern is matched against the identifier
  rather than the text you see on screen, so it selects the same lines whatever language the
  dashboard is displaying.
* **Logging class** — restricts results to the checked categories. Leaving every box
  unchecked includes them all. Categories that are currently switched off are still listed,
  because the log file may already contain lines written while they were on. The list of
  available classes is not hardcoded into the dashboard — it is fetched from the server, so
  it always matches whatever logging categories that particular build of _Ego_ registers (see
  [Available log categories](#available-log-categories) below).
* **Clear filters** — removes every filter. This deliberately leaves **Limit results**
  alone, since that governs how much is fetched rather than what qualifies.

If the server rejects a filter — an unknown logging class, a malformed pattern, or a class
or message filter on a server that writes its log as plain text rather than JSON — the
reason is shown in place of the log content.

**Logger Configuration sheet:**

Opened with the gear.

* **Log file path** — the path of the current server log file (read-only).
* **Keep previous logs** — the number of rotated log files to retain when the log is purged.
* **Logger toggles** — a toggle switch for each available logging category. Enabling a
  logger causes the server to start writing that category of messages immediately; disabling
  it stops them. Changes take effect as soon as you click **Save**.

<a id="available-log-categories"></a>**Available log categories:**

The current build of _Ego_ registers 28 logging categories. Both the sheets above populate
their category lists directly from the server (`GET /admin/loggers`) rather than from a fixed
list built into the dashboard, so a custom build that adds its own logger would show up here
automatically. As of this writing, the categories are:

| Logger | What it records |
| :--- | :--- |
| APP | General application/console-level messages outside the server request path |
| ASSET | Static asset (dashboard HTML/CSS/JS) serving |
| AUTH | Authentication and authorization decisions |
| BYTECODE | Disassembly of compiled _Ego_ pseudo-instructions |
| CACHE | In-memory cache add/evict/purge activity |
| CHILD | Child-process service invocation (`ego.server.child.services`) |
| CLI | Command-line argument processing |
| COMPILER | Package imports and source-file compilation steps |
| DB | Database connection lifecycle events |
| DEBUG | Interactive debugger session activity |
| GOROUTINE | _Ego_ `go` statement goroutine launch and lifecycle |
| INFO | Informational request/response detail, such as header dumps |
| INTERNAL | Internal error conditions and recovered panics; on by default |
| OPTIMIZER | Bytecode optimizer decisions |
| PACKAGES | Runtime package loading |
| RESOURCES | The `internal/resources` struct-reflection DDL/CRUD framework |
| REST | HTTP request and response details for the server |
| ROUTE | Request routing and media-type negotiation |
| SERVER | High-level server lifecycle events |
| SERVICES | Compilation and execution of `lib/services/*.ego` service endpoints |
| SQL | SQL statements generated by the tables/DSN REST endpoints |
| STATS | Reserved for future statistics logging; not currently emitted |
| SYMBOLS | Symbol table creation, lookup, and scope transitions |
| TABLES | Table-server request handling above the SQL-generation layer |
| TRACE | Execution of every _Ego_ virtual-machine instruction |
| TOKENIZER | Lexical analysis of _Ego_ source text |
| USER | Messages generated by `@LOG` directives inside _Ego_ programs |
| VALID | JSON request-body validation against endpoint schemas |

> **Permission required:** `ego.root` for logger configuration; no special permission is
> needed to view the log.

&nbsp;

## Keyboard Shortcuts

| Shortcut | Where | Action |
| :--- | :--- | :--- |
| Ctrl+Enter / Cmd+Enter | SQL editor | Submit all statements |
| Ctrl+Enter / Cmd+Enter | Code editor | Run the program |
| Enter | Code console | Execute the current console statement |
| Enter | Log search box | Find the next match |

&nbsp;

## Related Documentation

* [Ego Server](SERVER.md) — starting and configuring the _Ego_ REST server
* [Ego Server APIs](API.md) — REST endpoints that the dashboard uses internally
* [Ego Table Server Commands](TABLES.md) — managing DSNs and database tables
* [Language Reference](LANGUAGE.md) — _Ego_ language syntax for use in the Code tab
