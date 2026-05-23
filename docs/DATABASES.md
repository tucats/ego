# Ego Database Architecture and Dialect Audit

This document describes how Ego manages database connections, identifies which database dialects are supported, and serves as a living audit of dialect-specific issues that need attention before full parity between SQLite and PostgreSQL can be claimed.

---

## 1. Overview of Database Driver Architecture

### 1.1 Supported Drivers

Ego supports two database backends at all levels of the stack:

| Provider string | Driver registered | Package |
| - | - | - |
| `"sqlite"` | `modernc.org/sqlite` (registered as `"sqlite"`) | `modernc.org/sqlite` |
| `"sqlite3"` | Deprecated alias, normalized to `"sqlite"` at runtime | same |
| `"postgres"` or `"postgresql"` | `lib/pq` (registered as `"postgres"`) | `github.com/lib/pq` |

The aliases `"sqlite3"` and `"postgresql"` are normalized at open-time in every subsystem that opens a connection. The canonical names in the codebase are defined in the `defs` package:

```go
defs.SqliteProvider           = "sqlite"
defs.DeprecatedSqliteProvider = "sqlite3"
defs.PostgresProvider         = "postgres"
```

### 1.2 Connection String Formats

**SQLite** connections use a bare filesystem path after the scheme is stripped:

```text
sqlite://path/to/file.db      → opens path/to/file.db
sqlite3://path/to/file.db     → same (deprecated form)
```

**PostgreSQL** connections use a standard PostgreSQL URL:

```text
postgres://user:pass@host:port/database?sslmode=disable
```

The `dsns/connections.go` `Connection()` function constructs these from the `defs.DSN` struct fields. The `?sslmode=disable` parameter is appended when `d.Secured == false`.

### 1.3 Where Connections Are Opened

Database connections are opened in four distinct subsystems, each with its own open path:

| Subsystem | File | Purpose |
| - | - | - |
| System / auth database | `resources/open.go` | Users, credentials, starts log, DSNs |
| Server table operations | `server/tables/database/open.go` | User-facing `/tables` and `/dsns` endpoints |
| Configuration persistence | `app-cli/settings/databases.go` | Configuration key-value store |
| Cluster membership | `server/cluster/cluster.go` `openSystemDB()` | Cluster node registry |
| Ego runtime SQL package | `runtime/sql/db.go` `openDatabase()` | `sql.Open()` called from Ego programs |

All five paths apply the SQLite WAL-mode and busy-timeout PRAGMAs immediately after opening when the driver is `"sqlite"`:

```go
db.Exec("PRAGMA journal_mode=WAL;")
db.Exec("PRAGMA busy_timeout=5000;")
```

These are SQLite-only directives and are correctly gated on the provider check in every location.

### 1.4 DSN Schema / Two-Part Name Handling

For PostgreSQL, Ego uses the authenticated user's name as the database schema. When a `defs.DSN` object has a non-empty `Schema` field, that schema overrides the user name. The `FullName()` function in `server/tables/parsing/parsing.go` encapsulates this:

- **SQLite**: `"tablename"` (just the quoted table name — no schema prefix)
- **PostgreSQL**: `"schemaname"."tablename"` (both parts double-quoted)

The function handles pre-qualified names (those already containing a `.`) by splitting on the first dot and quoting both parts.

---

## 2. System Tables

All persistent server state is stored in a single database file (the "system database"). The path is resolved in order of precedence:

1. `--users` CLI flag
2. `ego.server.userdata` configuration setting
3. Default: `<ego.runtime.path>/ego-system.db`

The system database contains the following tables:

| Table | Owner package | Description |
| - | - | - |
| `credentials` | `server/auth/` via `resources/` | User accounts and permissions |
| `starts` | `server/auth/` via `resources/` | Server startup log |
| `dsns` | `dsns/` via `resources/` | Named data source definitions |
| `permissions` | `dsns/` via `resources/` | Per-DSN access control |
| `config_ids` | `app-cli/settings/` | Configuration profile metadata |
| `config_items` | `app-cli/settings/` | Configuration key-value pairs |
| `cluster` | `server/cluster/` | Cluster membership registry |

Tables owned by the `resources/` framework are created via struct reflection (see §3 below). The `config_*` and `cluster` tables are created with hand-written DDL.

---

## 3. The `resources/` Framework

The `resources` package provides a generic CRUD layer over a database table. It reflects over a Go struct to derive column names and types, then generates SQL for `CREATE TABLE`, `INSERT`, `UPDATE`, `DELETE`, and `SELECT` operations.

### 3.1 Type Mapping

Column types are derived from Go reflection in `resources/describe.go`:

| Go type | SQL type constant | SQL string used |
| - | - | - |
| `string`, `json.RawMessage`, `[]string`, `uuid.UUID` | `SQLStringType` | `"TEXT"` |
| `int` | `SQLIntType` | `"integer"` |
| `bool` | `SQLBoolType` | `"boolean"` |
| `float32` | `SQLFloatType` | `"float"` |
| `float64` | `SQLDoubleType` | `"double"` |

`TEXT` is accepted by both SQLite and PostgreSQL as a valid column type for string data, making the resource framework fully portable. (Prior to the DB-13 fix in May 2026, `SQLStringType` was `"char varying"`, which is a PostgreSQL-preferred alias — SQLite accepted it silently via type affinity, but the names did not match SQLite's own schema inspection output.)

### 3.2 Identifier Quoting in the `resources/` Framework

All SQL generated by `resources/generators.go` uses `egostrings.SQLIdentifier()` to quote column names and table names:

```go
sql.WriteString(egostrings.SQLIdentifier(column.SQLName))
sql.WriteString(fmt.Sprintf("create table %s (", egostrings.SQLIdentifier(r.Table)))
```

`egostrings.SQLIdentifier()` wraps the name in ANSI SQL double-quotes and doubles any internal double-quote character (`"` → `""`), which is the correct SQL standard escaping for both SQLite and PostgreSQL. (Prior to the DB-6 fix in May 2026, `strconv.Quote()` was used instead, which applies Go backslash escaping — not valid inside SQL double-quoted identifiers.)

---

## 4. Dialect-Aware Code: What Is Handled Correctly

### 4.1 Table Listing

`server/tables/list.go` branches on `db.Provider`:

```go
if db.Provider == defs.SqliteProvider {
    q = "select name from sqlite_schema where (type='table' or type='view')"
} else {
    // uses: SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}'
}
```

### 4.2 Column Metadata

`server/tables/describe.go` dispatches to separate implementations:

- `getPostgresColumnMetadata()` — uses `information_schema.columns` and `pg_index`/`pg_attribute` system tables
- `getSqliteColumnMetadata()` — uses `PRAGMA index_list()`, `PRAGMA index_info()`, and `PRAGMA table_info()`

### 4.3 Schema Qualification (`FullName`)

The `FullName()` function in `server/tables/parsing/parsing.go` produces correctly dialect-specific table names for all query generators in `server/tables/`.

### 4.4 LIMIT/OFFSET Pagination

Both SQLite and PostgreSQL use identical `LIMIT n OFFSET m` syntax. The `PagingClauses()` function generates this syntax unconditionally. ✓

### 4.5 Parameterized Query Placeholders

The REST table server (`server/tables/parsing/generators.go`) uses `$N` positional placeholders (`$1`, `$2`, …) throughout its generated queries. PostgreSQL requires this style; SQLite's `modernc.org/sqlite` driver also accepts `$N` placeholders. ✓

### 4.6 Schema Creation for PostgreSQL

`createSchemaIfNeeded()` in `server/tables/tables.go` issues `CREATE SCHEMA IF NOT EXISTS {{schema}}` only for PostgreSQL (SQLite has no schema concept in its connection model). ✓

### 4.7 Error Message Stripping

PostgreSQL errors from `lib/pq` carry a `"pq: "` prefix. Multiple locations strip it before user-facing display:

```go
strings.TrimPrefix(err.Error(), "pq: ")
```

This is handled consistently in `describe.go`, `rows.go`, and `tables.go`. ✓

---

## 5. Open Issues Found During Audit

The following issues were identified as incorrect, incomplete, or likely to cause failures when Postgres is used in place of SQLite (or vice versa). They are numbered for tracking during discussion and remediation.

---

### Issue DB-1: `strconv.Quote()` Used for SQL String Literal Values ✅ Fixed

**File:** `app-cli/settings/databases.go`

**Description:** Several SQL statements in the configuration persistence layer used `strconv.Quote()` to embed runtime string values directly into SQL. For example, before the fix:

```go
// Lines 331-333 (before fix)
sql := fmt.Sprintf(
    `UPDATE %s SET modified = CURRENT_TIMESTAMP WHERE id = %s`,
    strconv.Quote(d.Table),   // correct — identifier quoting
    strconv.Quote(cp.ID),     // WRONG — value quoting, produces "uuid-string"
)
```

Also at lines 353–355, 412–414, 421–423, and 455–457:

```go
sql = fmt.Sprintf(`DELETE FROM %s WHERE id = %s`,
    strconv.Quote(d.Items),
    strconv.Quote(cp.ID))    // cp.ID is a UUID string value

sql := fmt.Sprintf(`SELECT id, description, version, salt FROM %s WHERE name = %s LIMIT 1`,
    strconv.Quote(d.Table),
    strconv.Quote(name))     // name is a configuration profile name string
```

`strconv.Quote()` produces Go double-quoted strings (e.g., `"my-profile-id"`). In SQL:

- **PostgreSQL** treats double-quoted tokens as **identifiers** (column/table names), not string literals. These queries fail with `column "my-profile-id" does not exist`.
- **SQLite** accepts double-quoted tokens as string literals when no matching identifier is found (a non-standard compatibility quirk). These queries work by accident on SQLite only.

**Resolution (May 2026):** All five occurrences were fixed by converting the embedded values to `$1` positional parameters and passing them as arguments to `Exec`/`QueryRow`. The table-name uses of `strconv.Quote()` (identifier quoting) were left unchanged. Example of the fix pattern:

```go
// After fix — UPDATE
sql := fmt.Sprintf(`UPDATE %s SET modified = CURRENT_TIMESTAMP WHERE id = $1`,
    strconv.Quote(d.Table))
rows, err = tx.Exec(sql, cp.ID)

// After fix — DELETE
sql = fmt.Sprintf(`DELETE FROM %s WHERE id = $1`,
    strconv.Quote(d.Items))
_, err = tx.Exec(sql, cp.ID)

// After fix — SELECT (findConfig)
sql := fmt.Sprintf(`SELECT id, description, version, salt FROM %s WHERE name = $1 LIMIT 1`,
    strconv.Quote(d.Table))
row := d.db.QueryRow(sql, name)
```

A companion bug was also fixed in `NewDatabaseConfigService`: the `config_ids` table DDL used `id string PRIMARY KEY` — `string` is not a valid PostgreSQL type (SQLite accepted it silently via type affinity). This was changed to `id TEXT PRIMARY KEY`, which is accepted by both PostgreSQL and SQLite.

The fix was validated end-to-end against a live PostgreSQL instance, exercising `NewDatabaseConfigService`, `Load` (new-profile creation path), `Save`, `Load` (reload path), and `DeleteProfile`. All operations succeeded correctly with proper parameterized queries.

`$1` positional placeholders are used throughout (consistent with the rest of the file). `modernc.org/sqlite` accepts both `?` and `$N` style placeholders, so this change is backward-compatible with SQLite.

---

### Issue DB-2: Cluster Package Missing PostgreSQL Driver Import ✅ Fixed

**File:** `server/cluster/cluster.go`

**Description:** The cluster package's `openSystemDB()` function handles PostgreSQL connection strings (`postgres://...`) but the package only imported the SQLite driver:

```go
import (
    _ "modernc.org/sqlite"   // was present
    // _ "github.com/lib/pq" // was MISSING
)
```

If the system database is configured to use PostgreSQL (e.g., `ego.server.userdata = postgres://...`), the call to `sql.Open("postgres", connStr)` would fail with `sql: unknown driver "postgres"` because the driver was never registered in this package.

**Resolution (May 2026):** Added the PostgreSQL driver blank import:

```go
import (
    _ "github.com/lib/pq"
    _ "modernc.org/sqlite"
)
```

---

### Issue DB-3: Cluster SQL Uses SQLite-Only `?` Placeholders and `INSERT OR REPLACE` ✅ Fixed

**File:** `server/cluster/membership.go`, `server/cluster/cluster.go`

**Description:** All SQL in `membership.go` used `?` parameter placeholders and the SQLite-specific `INSERT OR REPLACE` syntax:

```go
// SQLite-specific UPSERT syntax (before fix)
`INSERT OR REPLACE INTO cluster ... VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

// SQLite-style placeholder (before fix)
`SELECT ... FROM cluster WHERE name = ? ORDER BY joined_at`
`UPDATE cluster SET state = 'removed', last_seen = ? WHERE node_id = ?`
```

PostgreSQL requires `$1`, `$2`, … positional placeholders. PostgreSQL does not support `INSERT OR REPLACE`; the equivalent is `INSERT ... ON CONFLICT (node_id) DO UPDATE SET ...`.

**Resolution (May 2026):**

1. **`server/cluster/cluster.go`**: Added a package-level `dbProvider string` variable. It is set to the driver name (`"sqlite"` or `"postgres"`) inside `openSystemDB()` immediately after `sql.Open()`, so all membership functions can read it without needing the provider threaded through every call.

2. **`server/cluster/membership.go`**: All four functions updated:
   - `ListMembers`, `RemoveMember`, `UpdateLastSeen`: `?` replaced with `$1`/`$2` positional parameters. `modernc.org/sqlite` accepts both `?` and `$N` style, so this is backward-compatible with SQLite.
   - `upsertMember`: branches on `dbProvider`:

```go
if dbProvider == defs.PostgresProvider {
    query = `INSERT INTO cluster (name, node_id, host, port, scheme, joined_at, last_seen, state)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
             ON CONFLICT (node_id) DO UPDATE SET
                 name=EXCLUDED.name, host=EXCLUDED.host, ...`
} else {
    query = `INSERT OR REPLACE INTO cluster (...) VALUES ($1, $2, ..., $8)`
}
```

3. **`server/cluster/membership_test.go`** (new): Package-internal tests cover all four membership functions against both `:memory:` SQLite and a live PostgreSQL instance (`TestMembershipSQLite` and `TestMembershipPostgres`). The PostgreSQL test is automatically skipped when no server is available.

---

### Issue DB-4: `MapColumnType()` Is PostgreSQL-Centric and Ignores Provider (Moderate) ✅ Fixed May 2026

**File:** `server/tables/parsing/parsing.go` (`MapColumnType()`)

**Fix:** Added a `provider string` parameter and dispatched to dialect-specific type maps. SQLite uses
standard type affinity names (`TEXT`, `INTEGER`, `REAL`); PostgreSQL retains its dialect
(`CHAR VARYING`, `DOUBLE PRECISION`, `TIMESTAMP WITH TIME ZONE`, etc.). The erroneous `INT32` DDL
type (not valid SQL) was corrected to `INTEGER` for both dialects.

```go
func MapColumnType(native, provider string) string {
    if strings.EqualFold(provider, defs.SqliteProvider) || strings.EqualFold(provider, defs.DeprecatedSqliteProvider) {
        // SQLite type affinity: TEXT, INTEGER, REAL
        types = map[string]string{
            data.StringTypeName: "TEXT",
            data.Int32TypeName:  "INTEGER",
            ...
            "timestamp":         "TEXT", // SQLite stores times as ISO-8601 text
        }
    } else {
        // PostgreSQL type names
        types = map[string]string{
            data.StringTypeName: "CHAR VARYING",
            data.Int32TypeName:  "INTEGER",
            ...
        }
    }
}
```

The single caller (`FormCreateQuery` in `generators.go`) already had `provider` in scope and was
updated to pass it through.

---

### Issue DB-5: SQL Statement Tokenizer Re-quotes String Literals with `strconv.Quote()` (Moderate) ✅ Fixed May 2026

**File:** `server/tables/sql.go` (`splitSQLStatements`)

**Root cause (refined from original analysis):** The Ego tokenizer (`text/scanner`-based) never
classified SQL single-quoted strings as `StringTokenClass` — they landed in `ValueTokenClass` with
their quotes intact. The real failures were:

1. `'it''s fine'` (SQL `''` escape) was split by the Go scanner into two tokens `'it'` + `'s fine'`
   and re-emitted as `'it' 's fine'` with a spurious space — invalid SQL.
2. The tokenizer added unwanted spaces around every punctuation character (parentheses, `.`, `,`),
   mutating the SQL string even when it was otherwise correct.
3. `"mycol"` (double-quoted SQL identifier) was classified as `StringTokenClass`, stripped of
   quotes, and re-quoted with `strconv.Quote()`. For plain ASCII names the result was coincidentally
   correct; a name containing a backslash would have been silently corrupted.

**Fix:** Replaced the Ego-tokenizer-based implementation entirely with a SQL-aware character
scanner that correctly handles:

- `'...'` single-quoted string literals, including the `''` escape sequence
- `"..."` double-quoted SQL identifiers, including the `""` escape sequence
- `--` SQL line comments (content stripped)
- `/* */` SQL block comments (content stripped)
- `#` and `//` Ego-style comment lines (stripped at the line level, existing behavior)

The scanner emits SQL **verbatim** — no re-tokenizing, no re-quoting, no added whitespace.
Semicolons are recognized as statement separators only when they appear outside all quoting
and comment contexts. This eliminates the dependency on the Ego tokenizer for SQL input.

The `strconv` import was removed from `sql.go`. Eighteen test cases covering all of the above
scenarios were added to `server/tables/sql_test.go`.

---

### Issue DB-6: `strconv.Quote()` Used as a SQL Identifier Quoter (Widespread, Low-Severity) ✅ Fixed May 2026

**Files fixed:** `resources/generators.go`, `resources/filters.go`, `server/tables/parsing/generators.go`, `server/tables/parsing/parsing.go`, `server/tables/parsingAbstract.go`, `server/tables/scripting/update.go`, `app-cli/settings/databases.go`

**Fix:** Added `egostrings.SQLIdentifier(name string) string` to `egostrings/quotes.go` alongside the existing `SingleQuote` helper:

```go
func SQLIdentifier(name string) string {
    return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
```

All 21 `strconv.Quote()` calls used for SQL identifier quoting across the seven files were replaced with `egostrings.SQLIdentifier()`. The `strconv` import was removed from the six files that no longer needed it; `server/tables/parsing/generators.go` retains `strconv` for its `strconv.Itoa` calls and the intentional `strconv.Quote()` on line 525 (which produces an Ego-dialect string literal, not a SQL identifier — correctly left unchanged).

Seven test cases for `SQLIdentifier` were added to `egostrings/strings_test.go`, covering simple names, names with internal double-quotes, backslashes, spaces, and the empty string.

---

### ~~Issue DB-7: `resources/generators.go` Uses Invalid `nullable` Column Modifier (Low-Severity, Latent)~~ ✅ Fixed May 2026

**File:** `resources/generators.go` (`createTableSQL()`)

**Fix:** Changed `sql.WriteString(" nullable")` to `sql.WriteString(" NULL")`. `NULL` is the correct ANSI SQL keyword for an explicitly-nullable column constraint; `nullable` is not recognized by PostgreSQL or SQLite.

The fix has no observable runtime effect today because no caller sets `column.Nullable = true` via `ResHandle.Nullable()` — all system table columns use the default `Nullable: false`, which emits nothing (columns are nullable by default in SQL). Four test cases were added to `resources/generators_test.go` covering the new `NULL` emission, the no-emission for non-nullable columns, and the correct double-quoting of table names (including names containing an internal double-quote).

---

### ~~Issue DB-8: `resources/generators.go` Inconsistent Table Name Quoting in `insertSQL()`~~ ✅ Fixed May 2026

**File:** `resources/generators.go`

**Description:** `createTableSQL()` and `readRowSQL()` quoted `r.Table` with `egostrings.SQLIdentifier()`, but `insertSQL()`, `updateSQL()`, and `deleteRowSQL()` did not, leaving table names unquoted in those statements.

**Fix:** Applied `egostrings.SQLIdentifier()` consistently to all table name references in `insertSQL()`, `updateSQL()`, and `deleteRowSQL()`. All five generator functions now uniformly use the SQL identifier quoter. The companion `resources/generators_test.go` file was created with `TestInsertUpdateDeleteSQL` and `TestCreateTableSQL` test functions that verify the quoting behavior.

---

### ~~Issue DB-9: PRAGMA Calls in `getSqliteColumnMetadata()` Use Unquoted Table Names Inconsistently~~ ✅ Fixed May 2026

**File:** `server/tables/describe.go` (`getSqliteColumnMetadata()`)

**Description:** The function extracted the bare table name with a fragile `.`-split that could produce a malformed PRAGMA argument when `tableName` arrived as `"schema"."table"` from `FullName()`.

**Fix:** Replaced the `.`-split with `parsing.TableNameParts()` + `egostrings.SQLIdentifier()`:

```go
parts := parsing.TableNameParts(db.Provider, session.User, tableName)
tableOnly := egostrings.SQLIdentifier(parts[len(parts)-1])  // correctly-quoted bare name
q := fmt.Sprintf("PRAGMA index_list(%s)", tableOnly)
// ...
q = fmt.Sprintf("PRAGMA table_info(%s)", tableOnly)
```

`TableNameParts` strips double-quotes and handles schema-prefixed names correctly for both providers. `SQLIdentifier` re-wraps the bare name with correct SQL double-quoting. The `PRAGMA index_info` call in the loop body uses the index name (a loop variable), not the table name, and was left unchanged.

---

### ~~Issue DB-10: `tablesListQuery` Schema Injection Uses Single-Quoted Value Without Parameterization~~ ✅ Fixed May 2026

**Files:** `server/tables/defs.go`, `server/tables/list.go`, `server/tables/describe.go`

**Description:** `tablesListQuery` and `nullableColumnsQuery` used `{{schema}}` / `{{table}}` template substitution with `SQLEscape()` rather than parameterized queries. Schema-injection risk was low (Ego user names are restricted) but the pattern was inconsistent with best practice.

**Fix:** Converted both queries to use positional parameters:

```go
// defs.go — before:
tablesListQuery = `... WHERE table_schema = '{{schema}}' ORDER BY table_name`
// after:
tablesListQuery = `... WHERE table_schema = $1 ORDER BY table_name`

// nullableColumnsQuery — before:
`WHERE c.table_schema = '{{schema}}' AND c.table_name = '{{table}}'`
// after:
`WHERE c.table_schema = $1 AND c.table_name = $2`
```

In `list.go`, the `parsing.QueryParameters()` call was removed; instead `params := []any{schema}` is passed directly to `db.Query(q, params...)`. For the SQLite branch `params` is set to `nil`. In `describe.go`, `parsing.TableNameParts()` extracts the bare schema and table names that are passed as `$1`/`$2`.

---

### ~~Issue DB-11: `uniqueColumnsQuery` Schema+Table in `::regclass` Cast May Mishandle Quoted Names~~ ✅ Fixed May 2026

**File:** `server/tables/defs.go`, `server/tables/describe.go`

**Description:** The `uniqueColumnsQuery` used `'{{schema}}.{{table}}'::regclass` to resolve a table OID. PostgreSQL's `::regclass` folds unquoted names to lowercase, so mixed-case table or schema names stored with quotes would not be found.

**Fix:** Replaced the `::regclass` approach with a parameterized join through `pg_class` and `pg_namespace`, which compares by exact `relname`/`nspname` values:

```sql
SELECT a.attname
FROM   pg_index i
JOIN   pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
JOIN   pg_class c ON c.oid = i.indrelid
JOIN   pg_namespace n ON n.oid = c.relnamespace
WHERE  n.nspname = $1   -- bare schema name (case-sensitive, exact match)
AND    c.relname = $2   -- bare table name  (case-sensitive, exact match)
AND    i.indisunique = true;
```

`$1` and `$2` are the unquoted bare schema and table names extracted by `parsing.TableNameParts()` in the caller. This correctly handles any casing without relying on PostgreSQL's `::regclass` identifier resolution.

---

### ~~Issue DB-12: `createSchemaIfNeeded()` Not Gated on Provider~~ ✅ Fixed May 2026

**File:** `server/tables/tables.go`

**Description:** `createSchemaIfNeeded()` issued `CREATE SCHEMA IF NOT EXISTS {{schema}}` without an internal provider check. The guard was at the call site only, which is fragile if the function is ever called from additional locations.

**Fix:** Added an early-return guard at the top of `createSchemaIfNeeded()`:

```go
func createSchemaIfNeeded(...) bool {
    // SQLite has no schema concept; return immediately to avoid running
    // a PostgreSQL-specific DDL statement against an SQLite connection.
    if db.Provider == defs.SqliteProvider {
        return true
    }
    // ... PostgreSQL schema creation ...
}
```

---

### ~~Issue DB-13: `SQLStringType = "char varying"` in `resources/defs.go` Is PostgreSQL-Preferred~~ ✅ Fixed May 2026

**File:** `resources/defs.go`

**Description:** `SQLStringType` was `"char varying"` — a PostgreSQL-style name. SQLite's schema inspection returns `TEXT`, so any comparison against live metadata would fail to match.

**Fix:** Changed to the portable `TEXT`, which is accepted as a valid column type by both SQLite and PostgreSQL:

```go
// before:
SQLStringType = "char varying"
// after:
SQLStringType = "TEXT"
```

The `resources/generators_test.go` test that checked for `"id" char varying` was updated to expect `"id" TEXT`.

---

### ~~Issue DB-14: `ALTER TABLE` Migration in `users_sqldb.go` Uses Hard-Coded `char varying`~~ ✅ Fixed May 2026

**File:** `server/auth/users_sqldb.go`

**Description:** The migration statement that adds the `lasttokenat` column used `char varying` rather than the portable `TEXT`.

**Fix:** Changed the column type to `TEXT`:

```go
// before:
`ALTER TABLE "credentials" ADD COLUMN "lasttokenat" char varying`
// after:
`ALTER TABLE "credentials" ADD COLUMN "lasttokenat" TEXT`
```

This is consistent with the DB-13 fix (`SQLStringType = "TEXT"`) and matches what `CreateIf()` now generates for string columns.

---

## 6. Summary Table

| # | Severity | File(s) | Issue |
| - | - | - | - |
| DB-1 | ~~**Critical**~~ ✅ **Fixed** | `app-cli/settings/databases.go` | `strconv.Quote()` used for SQL string values; breaks PostgreSQL. Fixed May 2026: all values converted to `$1` parameters; `id string` DDL type corrected to `id TEXT`. |
| DB-2 | ~~**Critical**~~ ✅ **Fixed** | `server/cluster/cluster.go` | PostgreSQL driver not imported; cluster fails with Postgres system DB. Fixed May 2026: added `_ "github.com/lib/pq"` import. |
| DB-3 | ~~**Critical**~~ ✅ **Fixed** | `server/cluster/membership.go` | `?` placeholders and `INSERT OR REPLACE` are SQLite-only. Fixed May 2026: `$N` placeholders throughout; `upsertMember` branches on `dbProvider` for `INSERT OR REPLACE` (SQLite) vs `ON CONFLICT` (PostgreSQL). |
| DB-4 | ~~**Moderate**~~ ✅ **Fixed** | `server/tables/parsing/parsing.go` | `MapColumnType()` is PostgreSQL-centric; no provider parameter. Fixed May 2026: added `provider` parameter; SQLite uses `TEXT`/`INTEGER`/`REAL` affinities, PostgreSQL retains its dialect. |
| DB-5 | ~~**Moderate**~~ ✅ **Fixed** | `server/tables/sql.go` | SQL tokenizer re-quoted string literals and mutated SQL. Fixed May 2026: replaced tokenizer with a SQL-aware character scanner; emits SQL verbatim. |
| DB-6 | ~~**Low/Latent**~~ ✅ **Fixed** | widespread (7 files) | `strconv.Quote()` used for identifier quoting. Fixed May 2026: `egostrings.SQLIdentifier()` added; all 21 call sites replaced. |
| DB-7 | ~~**Low/Latent**~~ ✅ **Fixed** | `resources/generators.go` | `nullable` keyword is not valid SQL. Fixed May 2026: changed to `NULL`. |
| DB-8 | ~~**Low**~~ ✅ **Fixed** | `resources/generators.go` | `insertSQL()`, `updateSQL()`, `deleteRowSQL()` did not quote table name. Fixed May 2026: `egostrings.SQLIdentifier()` applied consistently. |
| DB-9 | ~~**Low**~~ ✅ **Fixed** | `server/tables/describe.go` | PRAGMA args used fragile `.`-split of double-quoted names. Fixed May 2026: `parsing.TableNameParts()` + `egostrings.SQLIdentifier()`. |
| DB-10 | ~~**Low**~~ ✅ **Fixed** | `server/tables/defs.go`, `list.go`, `describe.go` | Schema/table names interpolated into SQL strings. Fixed May 2026: `tablesListQuery` and `nullableColumnsQuery` use `$1`/`$2` parameters; `QueryParameters()` calls removed. |
| DB-11 | ~~**Low**~~ ✅ **Fixed** | `server/tables/defs.go`, `describe.go` | `::regclass` cast folds names to lowercase; fails for mixed-case. Fixed May 2026: replaced with parameterized `pg_class`/`pg_namespace` join. |
| DB-12 | ~~**Minor**~~ ✅ **Fixed** | `server/tables/tables.go` | `CREATE SCHEMA` not gated on provider inside function. Fixed May 2026: added early-return SQLite guard at top of `createSchemaIfNeeded()`. |
| DB-13 | ~~**Minor**~~ ✅ **Fixed** | `resources/defs.go` | `SQLStringType = "char varying"` PostgreSQL-preferred. Fixed May 2026: changed to `TEXT` (portable to both dialects). |
| DB-14 | ~~**Minor**~~ ✅ **Fixed** | `server/auth/users_sqldb.go` | `ALTER TABLE` column type hard-coded as `char varying`. Fixed May 2026: changed to `TEXT` for consistency with DB-13. |

---

## 7. Quoting Reference: SQLite vs PostgreSQL

The following table summarizes the quoting conventions relevant to this codebase:

| Use | SQLite | PostgreSQL | Notes |
| - | - | - | - |
| Identifier (table, column) | `"name"` or `` `name` `` or `[name]` | `"name"` | Both accept ANSI SQL double-quote; SQLite additionally accepts backtick and brackets |
| String literal | `'value'` | `'value'` | Standard; single-quote only in ANSI SQL |
| String literal (PostgreSQL extension) | Not supported | `E'value'` or `$$value$$` | PostgreSQL dollar quoting for multi-line strings |
| Embedded double-quote in identifier | `"my""col"` | `"my""col"` | SQL standard: double the double-quote |
| Embedded single-quote in literal | `'it''s'` | `'it''s'` | SQL standard: double the single-quote |
| Go `strconv.Quote()` output | `"value"` (Go style) | `"value"` (Go style) | Go uses backslash escaping — **NOT** valid SQL quoting |
| Double-quoted string as value | Accepted (SQLite quirk) | **Rejected** (treated as identifier) | Source of DB-1 and DB-5 bugs |
| Parameter placeholder | `?` or `$N` | `$N` only | SQLite's `modernc.org/sqlite` accepts `$N` |

---

*Document written May 2026 based on audit of Ego commit `74f21a22` and surrounding history. Issues are labeled DB-1 through DB-14 for discussion tracking.*
