# Commands Package — Function-to-CLI Mapping

This document maps every exported command function in the `commands` package to the
CLI verb chains that invoke it. Ego supports two parallel grammar styles, selectable
via the `EGO_GRAMMAR` environment variable:

* **Traditional** (class/action order): `ego <class> <action> [options]`
  — defined in `grammar/traditional.go`
* **Verb** (verb/subject order): `ego <verb> <subject> [options]`
  — defined in `grammar/verbs.go` and the per-verb files under `grammar/`

Both styles invoke the same Go functions.

---

## Server lifecycle

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `RunServer` | `ego server run` | `ego server` (bare) |
| `Start` | `ego server start` | `ego start server` |
| `Stop` | `ego server stop` | `ego stop server` |
| `Restart` | `ego server restart` | `ego restart server` |
| `Status` | `ego server status` *(default verb)* | `ego show server status` *(default verb)* |

> `Start`, `Stop`, and `Restart` are not supported on Windows.

---

## Server administration

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `SetCacheSize` | `ego server caches set-size <limit>` | `ego set cache <limit>` |
| `FlushCaches` | `ego server caches flush` | `ego flush cache` |
| `ShowCaches` | `ego server caches show` | `ego show server cache` |
| `ServerMemory` | `ego server memory` | `ego show server memory` |
| `ServerValidations` | `ego server validation [<item>]` | `ego show server validations [<item>]` |
| `Logging` | `ego server logging [options]` | `ego show server log [entries]` / `ego set logging` |
| `LoggingFile` | *(use `--file` flag with `ego server logging`)* | `ego show server log file` |
| `LoggingStatus` | *(use `--status` flag with `ego server logging`)* | `ego show server log status` |
| `FormatLog` | `ego log [<file>...]` | `ego format log [<file>...]` |

---

## Token / blacklist management

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `TokenList` | `ego tokens list` | `ego list tokens` |
| `TokenRevoke` | `ego tokens revoke <id>...` | `ego revoke token <id>...` |
| `TokenDelete` | `ego tokens delete <id>...` | `ego delete token <id>...` |
| `TokenFlush` | `ego tokens flush` | `ego flush tokens` |

---

## DSN (Data Source Name) management

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `DSNSAdd` | `ego dsns add <name>` | `ego create dsn <name>` |
| `DSNSList` | `ego dsns list` | `ego list dsns` |
| `DSNShow` | `ego dsns show <name>` | `ego show dsn <name>` |
| `DSNSDelete` | `ego dsns delete <name>...` | `ego delete dsn <name>...` |
| `DSNSGrant` | `ego dsns grant <name> --username <u> --permissions <p,...>` | `ego grant dsn <name> --username <u> --permissions <p,...>` |
| `DSNSRevoke` | `ego dsns revoke <name> --username <u> --permissions <p,...>` | `ego revoke dsn <name> --username <u> --permissions <p,...>` |

---

## Table / database operations

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `TableList` | `ego table list [<dsn>]` | `ego list tables [<dsn>]` |
| `TableShow` | `ego table show-table <table>` *(aliases: show, metadata, columns)* | `ego show table columns <table>` |
| `TableCreate` | `ego table create <table> [<col>:<type>...]` | `ego create table <table> [<col>:<type>...]` |
| `TableDrop` | `ego table drop <table>...` | `ego delete table <table>...` |
| `TableContents` | `ego table read <table>` *(aliases: select, print, get, contents)* | `ego read table <table>` |
| `TableInsert` | `ego table insert <table> [col=val...]` *(aliases: write, append)* | `ego insert row <table> [col=val...]` |
| `TableUpdate` | `ego table update <table> [col=val...]` | `ego update <table> [col=val...]` |
| `TableDelete` | `ego table delete <table>` | `ego delete rows <table>` |
| `TableSQL` | `ego table sql <sql>` / `ego sql <sql>` | `ego sql <sql>` |
| `TablePermissions` | `ego table permissions [<dsn>]` | `ego show table permissions [<dsn>]` |
| `TableShowPermission` | `ego table show-permission <table>` *(aliases: permission, perm)* | `ego show table permission <table>` |
| `TableGrant` | `ego table grant <table> --permissions <p,...>` | `ego grant table <table> --permissions <p,...>` |
| `TableRevoke` | *(no traditional path)* | `ego revoke table <table> --permissions <p,...>` |

---

## User management

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `AddUser` | `ego server users create <username>` *(alias: add)* | `ego create user <username>` |
| `UpdateUser` | `ego server users update <username>` *(aliases: modify, alter)* | `ego grant user <username>` / `ego set user <username>` |
| `RevokeUser` | *(no traditional path)* | `ego revoke user <username> --permissions <p,...>` |
| `ShowUser` | `ego server users show <username>` | `ego show user <username>` |
| `DeleteUser` | `ego server users delete <username>` | `ego delete user <username>` |
| `ListUsers` | `ego server users list` *(default verb)* | `ego list users` |

---

## Program execution and utilities

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `RunAction` | `ego run [<file>...]` *(default verb)* | `ego run [<file>...]` *(default verb)* |
| `TestAction` | `ego test [<file-or-dir>...]` | `ego test [<file-or-dir>...]` |
| `PathAction` | `ego path` | `ego path` / `ego show path` |

---

## REST client

| Function | Traditional | Verb |
| -------- | ----------- | ---- |
| `RestGet` | `ego rest get <url>` | `ego rest get <url>` |
| `RestPost` | `ego rest post <url>` | `ego rest post <url>` |
| `RestPut` | `ego rest put <url>` | `ego rest put <url>` |
| `RestDelete` | `ego rest delete <url>` | `ego rest delete <url>` |
| `RestPatch` | `ego rest patch <url>` | `ego rest patch <url>` |
