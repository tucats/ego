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

![Sign In overlay](signin.png)

Once authenticated, the dashboard stores a bearer token in memory for the current browser
session. If the _Remember login_ setting is enabled (see [Settings](#settings) below), the
token is also written to a browser cookie that expires after 24 hours, so the login survives
a page refresh or a new tab opened to the same server.

If you remain idle for more than **15 minutes**, the dashboard automatically signs you out and
re-displays the login overlay.

&nbsp;

## Header Bar

The top of every page shows:

| Area | Content |
| :--- | :--- |
| Logo & title | Ego logo and the text "Ego Server Dashboard" |
| Server name | The hostname of the server you are connected to |
| Instance ID | The server's unique UUID (useful when running multiple instances) |
| Since | The date and time the server was started (its uptime reference point) |
| ☰ (hamburger) | Opens the [Settings](#settings) sheet and the **Log Out** button |

&nbsp;

## Settings

Click the hamburger menu (☰) in the top-right corner to access settings:

| Setting | Description |
| :--- | :--- |
| **Remember login** | When enabled, the session token is saved to a browser cookie so a page refresh does not require you to sign in again. The cookie expires after 24 hours. |
| **Dark mode** | Switches the dashboard to a dark color scheme. The Code tab always uses a dark theme regardless of this setting. |

Both settings are remembered in 30-day browser cookies.

Click **Log Out** in the same menu to immediately end your session and return to the login
overlay.

&nbsp;

## Tabs

The dashboard is organized into seven tabs. Click a tab name to switch to it; the last active
tab is remembered between page loads.

&nbsp;

### Status Tab

The Status tab shows the memory and cache state of the running server.

**Memory metrics** — a snapshot of Go runtime statistics:

| Field | Description |
| :--- | :--- |
| Requests processed | Total HTTP requests handled since startup |
| Application memory | Total memory obtained from the operating system (bytes) |
| Heap in use | Memory currently allocated on the heap |
| Stack in use | Memory currently in use by goroutine stacks |
| Objects in use | Number of live heap objects |
| GC cycles run | Number of garbage-collection cycles completed |

**Cache status** — a summary of the server's internal caches:

| Field | Description |
| :--- | :--- |
| Cached services | Number of compiled _Ego_ service endpoints held in memory |
| Cached assets | Number of static files (HTML, CSS, JS) held in memory, and their total byte size |
| Authorizations | Number of cached access-control decisions |
| Cached tokens | Active bearer tokens in the token cache |
| Blacklisted tokens | Tokens that have been explicitly invalidated |
| User items | Cached user-account records |
| DSN entries | Cached database connection descriptors |
| Schema entries | Cached table-schema descriptions |

Below the summary a **detail table** lists each individual cached item with its endpoint name,
class, reuse count, and the time it was last accessed.

**Toolbar buttons:**

| Button | Action |
| :--- | :--- |
| Refresh | Reloads memory and cache data from the server |
| Flush Caches | Deletes all cached items, forcing the server to recompile services and reload assets on the next request |

> **Permission required:** `ego.admin`

&nbsp;

### Users Tab

The Users tab lists every user account on the server and lets you create, edit, and delete
accounts.

**User list columns:**

| Column | Description |
| :--- | :--- |
| Username | The login name for the account |
| User ID | An internal identifier assigned by the server |
| Permissions | Comma-separated list of capabilities granted to this user (e.g. `ego.logon, ego.admin`) |

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
* Click **Save** to apply changes, or **Delete** to remove the account entirely.

Common permission values:

| Permission | Grants |
| :--- | :--- |
| `ego.logon` | Ability to authenticate (required for all interactive use) |
| `ego.admin` | Full administrative access: users, loggers, caches, memory stats |
| `ego.code.run` | Ability to execute arbitrary _Ego_ code in the Code tab |
| `ego.dsn.admin` | Ability to manage data source connections |

> **Permission required:** `ego.admin`

&nbsp;

### DSNs Tab

DSN stands for _Data Source Name_ — a named connection descriptor that tells the server how
to connect to a database. The DSNs tab lists every DSN configured on the server.

**Columns:**

| Column | Description |
| :--- | :--- |
| Name | The identifier used to reference this connection in _Ego_ programs and REST requests |
| Provider | Database engine: `sqlite3`, `postgres`, `mysql`, etc. |
| Database | Name of the database (or file path for SQLite) |
| Host | Hostname of the database server (blank for SQLite) |
| Port | TCP port (blank for SQLite) |
| User | Database login username |
| Secured | `Yes` if the connection uses SSL/TLS |
| Restricted | `Yes` if access to this DSN is limited to admin users |

DSN management (creating and deleting connections) is done through the _Ego_ CLI or the REST
API. The dashboard displays the current DSN list for reference. See
[Ego Table Server Commands](TABLES.md) for more information.

> **Permission required:** `ego.admin`

&nbsp;

### Tables Tab

The Tables tab lets you browse the database tables available through a DSN.

1. Select a **DSN** from the dropdown at the top of the tab. The table list updates
   automatically.
2. The table list shows the **name**, **schema**, **column count**, and **row count** for
   each table.
3. Click any table row to open a **detail sheet** listing each column's name, data type,
   size, and whether it is nullable or must contain a unique value.

> **Permission required:** access to the selected DSN

&nbsp;

### Data Tab

The Data tab lets you browse and edit the rows stored in a database table.

**Selecting data:**

1. Choose a **DSN** from the first dropdown.
2. Choose a **Table** from the second dropdown (populated automatically when a DSN is
   selected).
3. The rows of the selected table are loaded and displayed.

**Reading the data grid:**

* Each column in the table becomes a column in the grid.
* Numeric columns (`int`, `float`, and related types) are right-aligned.
* Fields that contain no value are shown as `null` in italic grey text.
* Float values always display a decimal point (e.g. `42.0`) to distinguish them from
  integers.
* A **row count** summary is shown below the grid.

**Choosing visible columns** — click **Columns** to open a picker sheet:

* Toggle individual columns on or off using the checkboxes.
* Click **Select all** to make every column visible again.
* The selection resets automatically when you switch to a different DSN or table.

**Editing a row** — click any row in the grid to open an edit sheet:

* All fields for that row are shown.
* Modify field values and click **Edit** to save changes back to the database.
* Click **Delete** to remove the row (only available for rows that have a row ID).
* Click **Cancel** to close the sheet without changes.
* If a row has no internal row ID, the sheet shows a message indicating that the row
  cannot be modified through the dashboard.

> **Permission required:** access to the selected DSN and table

&nbsp;

### Log Tab

The Log tab displays the server's log output and lets you configure which categories of
messages are written to the log.

**Viewing the log:**

* The log viewer shows the most recent lines from the server log file (default: 500 lines).
* Use the scrollbar to move through earlier entries.

**Searching the log:**

1. Type a search term in the search box.
2. Click **Find** (or press Enter) to highlight the first match.
3. Use **Next** and **Prev** to move between matches.
4. The status area shows the current match position (e.g. `Match 5 of 23`).
5. Click **✕** next to the search box to clear the search.

**Toolbar buttons:**

| Button | Action |
| :--- | :--- |
| Refresh | Reloads the log from the server |
| Go to End | Scrolls the log viewer to the most recent entries |
| Configure | Opens the Logger Configuration sheet |

**Logger Configuration sheet:**

* **Log file path** — the path of the current server log file (read-only).
* **Keep previous logs** — the number of rotated log files to retain when the log is purged.
* **Lines to fetch** — how many trailing lines to load each time the log tab is refreshed.
  This preference is saved in a browser cookie.
* **Logger toggles** — a toggle switch for each available logging category. Enabling a
  logger causes the server to start writing that category of messages immediately; disabling
  it stops them. Changes take effect as soon as you click **Save**.

Available log categories:

| Logger | What it records |
| :--- | :--- |
| AUTH | Authentication and authorization decisions |
| BYTECODE | Disassembly of compiled _Ego_ pseudo-instructions |
| CLI | Command-line argument processing |
| COMPILER | Package imports and source-file compilation steps |
| DB | Database connection lifecycle events |
| REST | HTTP request and response details for the server |
| SERVER | High-level server lifecycle events |
| SYMBOLS | Symbol table creation, lookup, and scope transitions |
| TABLES | SQL statements generated by the `/tables` REST endpoint |
| TRACE | Execution of every _Ego_ virtual-machine instruction |
| USER | Messages generated by `@LOG` directives inside _Ego_ programs |

> **Permission required:** `ego.admin` for logger configuration; no special permission is
> needed to view the log.

&nbsp;

### Code Tab

The Code tab is an interactive development environment that lets you write and run _Ego_
programs directly in the browser.

**Layout:**

```text
┌──────────────────────────────────────────────────────┐
│  [Open]  [Clear]                           [Run ▾]   │
├────────────────────────┬─────────────────────────────┤
│                        │                             │
│   Editor               │   Output                    │
│   (left pane)          │   (right pane)              │
│                        │                             │
├────────────────────────┴─────────────────────────────┤
│   Console (REPL)                        [Clear]      │
│   ego> _                                             │
└──────────────────────────────────────────────────────┘
```

The vertical divider between the editor and output panes, and the horizontal divider above
the console, can both be dragged to resize the panes.

**Editor pane** (top-left):

* Type or paste _Ego_ source code into the editor.
* Line numbers are shown on the left edge.
* Click **Open** to load an `.ego` file from your local disk.
* Click **Clear** to erase the editor contents.

**Output pane** (top-right):

* Shows the text output produced when the program runs.
* Displays any compiler or runtime error messages.
* Click **Clear** to erase the output.

**Running code:**

Click the **Run** button to compile and execute the code in the editor. A spinner is shown
while the server is processing the request.

The **Run** button has a dropdown arrow (▾) with two options:

| Option | Behavior |
| :--- | :--- |
| **Run** | Normal execution; output goes to the output pane |
| **Trace** | Execution with the TRACE logger temporarily enabled; the full instruction-by-instruction trace of the virtual machine appears in the output pane alongside program output. Useful for debugging. |

**Console pane (REPL):**

The console at the bottom of the tab provides a read-eval-print loop (REPL). Type an _Ego_
statement at the `ego>` prompt and press Enter to execute it immediately.

The key difference between the editor and the console:

| | Editor | Console |
| :--- | :--- | :--- |
| Symbol table | Fresh on every **Run** — variables declared in one run do not persist to the next | **Persistent** across runs — variables you declare remain available in subsequent statements |
| Use case | Testing complete programs | Exploratory, incremental work |

The persistent symbol table for the console is stored on the server and is tied to the
specific browser tab (identified by a UUID generated when the page loads). Symbol tables
for inactive sessions are automatically cleaned up by the server after one hour of
inactivity.

> **Permission required:** `ego.code.run` (and `ego.admin`)

&nbsp;

## Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| Enter (in Console) | Execute the current console statement |
| Enter (in Search box) | Find the next match in the log |

&nbsp;

## Related Documentation

* [Ego Server](SERVER.md) — starting and configuring the _Ego_ REST server
* [Ego Server APIs](API.md) — REST endpoints that the dashboard uses internally
* [Ego Table Server Commands](TABLES.md) — managing DSNs and database tables
* [Language Reference](LANGUAGE.md) — _Ego_ language syntax for use in the Code tab
