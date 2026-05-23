# Ego Database Architecture and Dialect Audit

This document describes how Ego manages database connections, identifies which database dialects are supported, and serves as a living audit of dialect-specific issues that need attention before full parity between SQLite and PostgreSQL can be claimed.

---

## 1. Overview of Database Driver Architecture

### 1.1 Supported Drivers

Ego supports two database backends at all levels of the stack:

| Provider string | Driver registered | Package |
|---|---|---|
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
```
sqlite://path/to/file.db      → opens path/to/file.db
sqlite3://path/to/file.db     → same (deprecated form)
```

**PostgreSQL** connections use a standard PostgreSQL URL:
```
postgres://user:pass@host:port/database?sslmode=disable
```

The `dsns/connections.go` `Connection()` function constructs these from the `defs.DSN` struct fields. The `?sslmode=disable` parameter is appended when `d.Secured == false`.

### 1.3 Where Connections Are Opened

Database connections are opened in four distinct subsystems, each with its own open path:

| Subsystem | File | Purpose |
|---|---|---|
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
|---|---|---|
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
|---|---|---|
| `string`, `json.RawMessage`, `[]string`, `uuid.UUID` | `SQLStringType` | `"char varying"` |
| `int` | `SQLIntType` | `"integer"` |
| `bool` | `SQLBoolType` | `"boolean"` |
| `float32` | `SQLFloatType` | `"float"` |
| `float64` | `SQLDoubleType` | `"double"` |

These types use PostgreSQL naming conventions. SQLite accepts them through type affinity rules (`char varying` → TEXT affinity, `integer` → INTEGER affinity, etc.), so table creation generally succeeds on both databases.

### 3.2 Identifier Quoting in the `resources/` Framework

All SQL generated by `resources/generators.go` uses `strconv.Quote()` to quote column names and table names:

```go
sql.WriteString(strconv.Quote(column.SQLName))
sql.WriteString(fmt.Sprintf("create table %s (", strconv.Quote(r.Table)))
```

`strconv.Quote()` is Go's string-literal quoter. For simple ASCII identifiers it happens to produce valid SQL double-quoted identifiers (e.g., `"credentials"`). However, it uses Go backslash escaping conventions for non-ASCII and control characters (e.g., `\n`, `\t`, `\\`), which are **not valid inside SQL double-quoted identifiers**. SQL's correct way to embed a double-quote inside a double-quoted identifier is to double it (`"col""name"`). For the current system table column names (all lowercase ASCII) this is harmless in practice, but the mechanism is incorrect.

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

### ~~Issue DB-4: `MapColumnType()` Is PostgreSQL-Centric and Ignores Provider (Moderate)~~ ✅ Fixed May 2026

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

### Issue DB-5: SQL Statement Tokenizer Re-quotes String Literals with `strconv.Quote()` (Moderate)

**File:** `server/tables/sql.go` (approximately lines 333–335, inside `getStatementsFromRequest`)

**Description:** The function that splits a raw SQL payload into individual statements by tokenizing and re-assembling re-quotes string tokens with Go's `strconv.Quote()`:

```go
if token.IsString() {
    next = next + strconv.Quote(token.Spelling())
} else {
    next = next + token.Spelling()
}
```

`strconv.Quote()` wraps the string content in double quotes with Go backslash escaping. If the original SQL contained a single-quoted string literal like `'hello world'`, after tokenization the token's spelling is `hello world` (stripped of quotes), and `strconv.Quote("hello world")` produces `"hello world"` — a double-quoted token. In PostgreSQL this is an **identifier**, not a string literal, and the query will fail.

This affects all raw SQL passed to the `@sql` endpoint when any string literal appears.

**Proposed fix:** Re-quote string tokens using standard SQL single-quote wrapping (doubling any internal single quotes) rather than `strconv.Quote()`:

```go
if token.IsString() {
    s := strings.ReplaceAll(token.Spelling(), "'", "''")
    next = next + "'" + s + "'"
} else {
    next = next + token.Spelling()
}
```

---

### Issue DB-6: `strconv.Quote()` Used as a SQL Identifier Quoter (Widespread, Low-Severity)

**Files:** `resources/generators.go`, `resources/filters.go`, `server/tables/parsing/generators.go`, `server/tables/parsing/parsing.go`, `server/tables/parsingAbstract.go`, `server/tables/scripting/update.go`, `app-cli/settings/databases.go`

**Description:** Throughout the codebase, `strconv.Quote()` is used to produce double-quoted SQL identifiers for column and table names. Go's `strconv.Quote()` is designed for Go string literals, not SQL identifiers. The differences:

| Situation | Go `strconv.Quote()` | Correct SQL double-quote |
|---|---|---|
| Simple ASCII name | `"mycolumn"` | `"mycolumn"` — same ✓ |
| Name with backslash | `"my\\column"` | `"my\column"` — WRONG |
| Name with internal double-quote | `"my\"col"` | `"my""col"` — WRONG |
| Name with newline | `"my\ncol"` | `"my\ncol"` — WRONG in SQL |
| Unicode (non-BMP) | Go Unicode escapes | passthrough — different |

For the current set of system table and user table names (all lowercase ASCII letters and underscores), the outputs are identical and no failures occur. The bug is latent: it would surface if anyone created a column or table with a name containing a backslash, internal double-quote, or certain Unicode characters.

**Proposed fix:** Introduce a dedicated `sqlIdentifier(name string) string` helper:

```go
func sqlIdentifier(name string) string {
    return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
```

Replace all `strconv.Quote(identifierName)` calls in SQL-generating code with `sqlIdentifier(identifierName)`.

---

### Issue DB-7: `resources/generators.go` Uses Invalid `nullable` Column Modifier (Low-Severity, Latent)

**File:** `resources/generators.go` (`createTableSQL()`, line 48)

**Description:** The system table CREATE TABLE generator emits `nullable` as a column constraint keyword:

```go
if column.Nullable {
    sql.WriteString(" nullable")
}
```

Neither PostgreSQL nor SQLite recognizes `nullable` as a valid column constraint. The correct SQL is `NULL` (to explicitly allow nulls) or `NOT NULL`. PostgreSQL would reject this DDL; SQLite might accept it silently as an unknown keyword.

**Status:** Currently harmless because no caller ever sets `column.Nullable = true` via `ResHandle.Nullable()`. But any future use of that method on a system table would produce invalid DDL.

**Proposed fix:** Change the generated SQL to use standard SQL:

```go
if column.Nullable {
    sql.WriteString(" NULL")
} else {
    // columns are implicitly nullable by default; omit for brevity, or:
    // sql.WriteString(" NOT NULL")
}
```

---

### Issue DB-8: `resources/generators.go` Inconsistent Table Name Quoting in `insertSQL()`  (Low-Severity)

**File:** `resources/generators.go`

**Description:** `createTableSQL()` and `readRowSQL()` wrap `r.Table` in `strconv.Quote()`, but `insertSQL()` does not:

```go
// createTableSQL — table is quoted:
sql.WriteString(fmt.Sprintf("create table %s (", strconv.Quote(r.Table)))

// insertSQL — table is NOT quoted:
sql.WriteString(fmt.Sprintf("insert into %s(", r.Table))
```

This means `INSERT INTO credentials(...)` succeeds, but would fail if the table name were a reserved word or contained special characters. This is a minor inconsistency.

**Proposed fix:** Apply `strconv.Quote()` (or the future `sqlIdentifier()` helper) consistently to all table name references in `resources/generators.go`, including `insertSQL()` and `deleteRowSQL()`.

---

### Issue DB-9: PRAGMA Calls in `getSqliteColumnMetadata()` Use Unquoted Table Names Inconsistently (Low-Severity)

**File:** `server/tables/describe.go` (`getSqliteColumnMetadata()`)

**Description:** The function receives `tableName` which may be a double-quoted name from `FullName()` (e.g., `"mytable"`). The code extracts just the table part by splitting on `.`:

```go
if strings.Contains(tableName, ".") {
    tableName = strings.Split(tableName, ".")[1]
}
q := fmt.Sprintf("PRAGMA index_list(%s)", tableName)
```

If `tableName` is `"mytable"` (with double-quotes), the PRAGMA becomes `PRAGMA index_list("mytable")`, which SQLite accepts. However:
- The split on `"."` is fragile: if either the schema or table name contains a dot (unusual but possible), the split is incorrect.
- If `tableName` comes in as `"schema"."table"`, the `strings.Split()[1]` result is `"table"` (retaining the leading double-quote but losing the trailing one), producing a malformed PRAGMA argument.

**Proposed fix:** Use `parsing.StripQuotes()` followed by explicit re-quoting, or use `parsing.TableNameParts()` which already handles this correctly:

```go
parts := parsing.TableNameParts(db.Provider, db.User, tableName)
tableOnly := parts[len(parts)-1]  // unquoted bare name
q := fmt.Sprintf(`PRAGMA index_list("%s")`, tableOnly)
```

---

### Issue DB-10: `tablesListQuery` Schema Injection Uses Single-Quoted Value Without Parameterization (Low-Severity)

**File:** `server/tables/defs.go`, `server/tables/list.go`

**Description:**
```go
tablesListQuery = `SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}' ORDER BY table_name`
```

The `{{schema}}` substitution inserts the current user's name directly into a single-quoted SQL string literal. `SQLEscape()` is applied, which blocks embedded `'` and `;` characters. This provides basic SQL injection protection, but is not as robust as parameterized queries.

Additionally, if a schema name legitimately contains a single quote (unusual for Ego-managed schemas but possible if operating against an external PostgreSQL database), `SQLEscape` rejects it with an error rather than escaping it.

**Proposed fix:** Use a parameterized query:
```sql
SELECT table_name FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name
```

The same pattern applies to `nullableColumnsQuery` and `uniqueColumnsQuery`.

---

### Issue DB-11: `uniqueColumnsQuery` Schema+Table in `::regclass` Cast May Mishandle Quoted Names (Low-Severity)

**File:** `server/tables/defs.go`

**Description:**
```sql
uniqueColumnsQuery = `SELECT a.attname FROM pg_index i ...
    WHERE i.indrelid = '{{schema}}.{{table}}'::regclass`
```

After `QueryParameters()` processes this with a double-quoted `tableName` like `"admin"."mytable"`:
1. `QueryParameters` splits on `.` to produce `args["schema"] = '"admin"'` and `args["table"] = '"mytable"'`
2. `SQLEscape` strips the outer double-quotes from each, yielding `admin` and `mytable`
3. The final query becomes: `WHERE i.indrelid = 'admin.mytable'::regclass`

PostgreSQL's `::regclass` cast accepts unquoted names like `'admin.mytable'` and folds them to lowercase for lookup. This works for typical lowercase schema and table names. However, if the schema or table name was originally mixed-case and created with quotes, PostgreSQL would have stored it case-sensitively, and the unquoted `'Admin.MyTable'` cast would fail to find it. In Ego, all user names and table names are lowercased at the API level, so this is low risk but worth documenting.

**Proposed fix:** Preserve the double-quoting inside the regclass cast:
```sql
WHERE i.indrelid = '"{{schema}}"."{{table}}"'::regclass
```
Which requires the `QueryParameters` substitution to not strip the outer double-quotes for this particular template, or use a dedicated `pg_class` lookup by `relname` and `relnamespace` instead.

---

### Issue DB-12: `createSchemaIfNeeded()` Not Gated on Provider (Minor)

**File:** `server/tables/tables.go`

**Description:** `createSchemaIfNeeded()` issues `CREATE SCHEMA IF NOT EXISTS {{schema}}` without first checking whether the database is PostgreSQL. SQLite does not support `CREATE SCHEMA`, and calling this function against a SQLite DSN would produce an error. Currently the function is only called from `CreateTable()`, which checks the provider before calling schema creation, but the guard is located at the call site rather than inside the function itself, which is fragile.

**Proposed fix:** Move the provider check inside `createSchemaIfNeeded()`:

```go
func createSchemaIfNeeded(...) bool {
    if db.Provider == defs.SqliteProvider {
        return true  // SQLite has no schemas; nothing to do
    }
    // ... PostgreSQL schema creation ...
}
```

---

### Issue DB-13: `SQLStringType = "char varying"` in `resources/defs.go` Is PostgreSQL-Preferred (Minor)

**File:** `resources/defs.go`

**Description:**
```go
SQLStringType = "char varying"
```

This is a PostgreSQL-style type name. SQLite uses `TEXT` as its canonical string type. While SQLite's type affinity rules accept `char varying` (any type name containing "char" gets TEXT affinity), using the PostgreSQL-centric name in the resource framework means that if this ever needs to be schema-inspected or compared against SQLite metadata output, the names won't match.

**Proposed fix:** Make `SQLStringType` dialect-aware, or adopt the more portable `TEXT` which both SQLite and PostgreSQL accept as a valid column type.

---

### Issue DB-14: `ALTER TABLE` Migration in `users_sqldb.go` Uses Hard-Coded `char varying` (Minor)

**File:** `server/auth/users_sqldb.go` (line 122)

**Description:**
```go
`ALTER TABLE "credentials" ADD COLUMN "lasttokenat" char varying`
```

This is PostgreSQL-preferred syntax for the type. For SQLite, `TEXT` would be more idiomatic. Both would work due to type affinity, but inconsistency could cause confusion when inspecting schema metadata.

**Proposed fix:** Make the type dialect-aware, using `TEXT` (accepted by both databases) or branching on the provider.

---

## 6. Summary Table

| # | Severity | File(s) | Issue |
|---|---|---|---|
| DB-1 | ~~**Critical**~~ ✅ **Fixed** | `app-cli/settings/databases.go` | `strconv.Quote()` used for SQL string values; breaks PostgreSQL. Fixed May 2026: all values converted to `$1` parameters; `id string` DDL type corrected to `id TEXT`. |
| DB-2 | ~~**Critical**~~ ✅ **Fixed** | `server/cluster/cluster.go` | PostgreSQL driver not imported; cluster fails with Postgres system DB. Fixed May 2026: added `_ "github.com/lib/pq"` import. |
| DB-3 | ~~**Critical**~~ ✅ **Fixed** | `server/cluster/membership.go` | `?` placeholders and `INSERT OR REPLACE` are SQLite-only. Fixed May 2026: `$N` placeholders throughout; `upsertMember` branches on `dbProvider` for `INSERT OR REPLACE` (SQLite) vs `ON CONFLICT` (PostgreSQL). |
| DB-4 | ~~**Moderate**~~ ✅ **Fixed** | `server/tables/parsing/parsing.go` | `MapColumnType()` is PostgreSQL-centric; no provider parameter. Fixed May 2026: added `provider` parameter; SQLite uses `TEXT`/`INTEGER`/`REAL` affinities, PostgreSQL retains its dialect. |
| DB-5 | Moderate | `server/tables/sql.go` | SQL tokenizer re-quotes string literals as Go double-quoted strings |
| DB-6 | Low/Latent | widespread | `strconv.Quote()` used for identifier quoting; wrong for special chars |
| DB-7 | Low/Latent | `resources/generators.go` | `nullable` keyword is not valid SQL |
| DB-8 | Low | `resources/generators.go` | `insertSQL()` does not quote table name; inconsistent with other generators |
| DB-9 | Low | `server/tables/describe.go` | PRAGMA table/index args use fragile `.`-split of double-quoted names |
| DB-10 | Low | `server/tables/defs.go`, `list.go` | Schema name interpolated into SQL string; should use parameter |
| DB-11 | Low | `server/tables/defs.go` | `::regclass` cast loses double-quoting; fails for mixed-case names |
| DB-12 | Minor | `server/tables/tables.go` | `CREATE SCHEMA` not gated inside function; SQLite would error |
| DB-13 | Minor | `resources/defs.go` | `SQLStringType = "char varying"` PostgreSQL-preferred; use `TEXT` |
| DB-14 | Minor | `server/auth/users_sqldb.go` | `ALTER TABLE` column type hard-coded as `char varying` |

---

## 7. Quoting Reference: SQLite vs PostgreSQL

The following table summarizes the quoting conventions relevant to this codebase:

| Use | SQLite | PostgreSQL | Notes |
|---|---|---|---|
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
