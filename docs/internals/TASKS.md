# Scheduled Server Tasks

Status: **implemented** (Phases 1-4 landed, end-to-end verified). This doc doubles as the implementation tracker
during development and as the internals reference for the feature once it lands — update
it in place rather than leaving stale sections once code diverges from the plan below.

## Progress

- [x] Config keys added (`internal/defs/config.go`, `ego.server.tasks.*`)
- [x] `internal/server/tasks` package: `defs.go` (Task/State types, registry)
- [x] `internal/server/tasks` package: `permissions.go` (dir 0700 / file 0600 enforcement)
- [x] `internal/server/tasks` package: `load.go` (directory scan, validation, dup-id handling)
- [x] TASKS log class registered (`ui.DefineLogger("TASKS", false)`)
- [x] Unit tests for `permissions.go` and `load.go`
- [x] `internal/server/tasks` package: `state.go` (sidecar `.state.json`, 0600)
- [x] `internal/server/tasks` package: `scheduler.go` (due-task selection, concurrency cap)
- [x] Unit tests for `state.go` and `scheduler.go` (race-clean, `-race -count=10`)
- [x] `internal/server/tasks` package: `dispatch.go` + `save.go` ({{key}} substitution)
- [x] Unit tests for `dispatch.go` and `save.go` (real in-process router round trip)
- [x] `internal/server/tasks` package: `routes.go` (`GetTasksHandler`/`RunTaskHandler`/`DeleteTaskHandler`) + `/admin/tasks` path constants
- [x] `internal/server/tasks` package: `deactivate.go` (surgical `active` field patch for DELETE)
- [x] Unit tests for `routes.go` and `deactivate.go`
- [x] Startup wiring (`internal/commands/server.go`, `internal/commands/routes.go`)
- [x] End-to-end verification against the real built binary (see Verification section)
- [x] Permission-enforcement verification (done as part of the same end-to-end run)
- [x] `POST /admin/tasks/@reload`: `reload.go` (`Reload`, `ReloadTaskID`), `routes.go`
      (`ReloadTasksHandler`), `defs.go` (`upsert`/`removeMissing`) -- add/edit/reactivate a
      task by editing its file, no restart required
- [x] Unit tests for `reload.go` and `ReloadTasksHandler`; end-to-end verified against the
      built binary (add, edit, delete a task file while the server runs; see Phase 5 notes)

Phase 2 note: `dispatchFunc` in `scheduler.go` is a package-level function variable
(default: log "no dispatcher registered" and report failure) so the scheduler's due-task
and concurrency logic is unit-testable without a running router/auth service. Phase 3's
`dispatch.go` assigns the real implementation via its own `init()`. Also: `recordRun`
(`state.go`) deliberately keeps a task's `Running` flag true until *after* `SaveState`
returns, not just until the endpoint call finishes -- a run isn't fully done until its
result is durably recorded, and it gives callers a single, simple signal
(`runningCount() == 0`) for "this run, including its state write, has completely
finished."

Phase 3 notes:

- **The in-process timeout is best-effort, not preemptive.** `dispatch()` races the
  in-process `doDispatch()` call against `time.After(timeout)` on a channel; if the
  timeout wins, the run is recorded as a failure immediately, but the abandoned
  `doDispatch()` goroutine is *not* killed -- Go has no cooperative-cancellation hook into
  an arbitrary running goroutine, and the router's `HandlerFunc` signature takes no
  `context.Context`. The goroutine finishes on its own time and its result is simply
  discarded. This is the direct cost of the in-process dispatch decision from the design
  phase (no real `net/http` connection to close). Accepted as-is; a slow or hung target
  endpoint leaks one goroutine per timeout until it eventually returns, not indefinitely.
- **`save.go`'s substitution is intentionally minimal** compared to `tools/apitest`'s
  `dictionary` package: plain `{{name}}` replacement only, no `|format` pipe directives,
  no `$uuid`/`$hash` dynamic values. A task only ever needs to carry one call's saved
  value into another call's endpoint/parameters/body, not apitest's richer
  test-authoring template language.
- **Testing a real in-process dispatch requires a minimal live `auth.AuthService`**, since
  `router.Authenticate()` unconditionally calls `auth.GetPermissions`, which panics on a
  nil `AuthService`. `main_test.go` sets one up with `auth.NewFileService` against a temp
  file, mirroring the existing pattern in `internal/router/auth_test.go`.
- **A target route must declare every query parameter a task sends** via the route's own
  `.Parameter(name, kind)` call, or the router rejects the request with 400 "invalid
  option keyword" -- this is a router-wide whitelisting behavior, not specific to tasks;
  it surfaced while writing `dispatch_test.go`'s fixture route and is worth remembering
  when authoring real task JSON against real endpoints.

Phase 4 notes:

- **`Task.Active` is the one field mutated after load** (by `DELETE /admin/tasks/{id}`),
  so it's the one field that needs lock discipline everywhere it's read: `isDue`
  (`scheduler.go`) reads it inside the same `registryLock.RLock()` critical section as the
  state lookup (not before it), and reporting goes through a new `Snapshot()` helper
  (`defs.go`) that copies out `Description`/`ID`/`Active` plus the current `State` under a
  single lock, rather than handing `GetTasksHandler` live `*Task` pointers to read
  unsynchronized after the lock is released. `setActive` (`defs.go`) is the only writer.
- **A handler's `int` return value is not what sets the HTTP status** — `router.ServeHTTP`
  only uses it for the server-log line; the handler itself must call `w.WriteHeader(status)`
  for anything other than the implicit-200 default. `RunTaskHandler` returning
  `http.StatusAccepted` without also calling `w.WriteHeader(http.StatusAccepted)` shipped
  as a real bug that `TestRunTaskHandlerStartsAsyncAndReturnsAccepted` caught (server
  reported 200, not 202) — fixed by adding the explicit call.
- **End-to-end verification was run against the actual built binary**, not just unit
  tests, using an isolated `taskstest` CLI profile (`ego.runtime.path` pointed at a temp
  dir, `ego.server.tasks.enabled=true`) with one real task file targeting
  `/admin/heartbeat`. Confirmed from the server's own JSON log and live `curl` calls:
  directory permissions auto-corrected 0755→0700, file permissions auto-corrected
  0644→0600, the task loaded and dispatched at startup (`tasks.run.complete`,
  status=200, success=true, heartbeat counter incremented), `GET /admin/tasks` reported
  it correctly, `POST /admin/tasks/{id}` returned 202 and produced a new `lastRun`,
  and `DELETE /admin/tasks/{id}` flipped `"active": "true"` to `"active": "false"` in the
  file while leaving every comment and all formatting untouched. All test artifacts
  (temp directory, CLI profile, log files) were cleaned up afterward.

Phase 5 notes (`POST /admin/tasks/@reload`):

- **`@reload` is a reserved task id**, following the same convention as this codebase's
  other `@name` pseudo-identifiers (`@sql`, `@permissions`, `@metadata`, `@generate`):
  `validateTask` (`load.go`) rejects any real task file that tries to declare
  `"id": "@reload"`, so it can never collide with a real task.
- **Removal is keyed on file path, not on id.** `Reload`'s `removeMissing` (`defs.go`)
  only forgets a task whose *file* is confirmed gone from the directory listing. A file
  that's still present but currently fails to parse or validate must NOT cause its task to
  be removed -- that would let one bad edit silently kill a running task's registration and
  execution history. `TestReloadSkipsInvalidFileWithoutTouchingExistingEntry` pins this
  down; the first implementation got it wrong (it keyed removal on which *ids* were
  successfully parsed this pass, which conflated "file deleted" with "file broken").
- **`upsert` swaps the `*Task` pointer rather than mutating fields on the existing one.**
  A run already in flight, holding the pointer it captured before the reload, keeps running
  against the definition it started with -- only *later* lookups see the edit. This also
  means an edited task's `*State` (last run/status/success) is deliberately left alone by
  upsert, so editing a task's definition doesn't erase its execution history.
- **The response reuses `defs.TasksResponse`** with a human-readable `Message` summarizing
  counts (e.g. `"reloaded: 1 total, 0 new, 1 updated, 0 removed"`) rather than a new
  response type, since the shape (`ServerInfo`/`Status`/`Message`/`Count`) already fit.
- **End-to-end verified against the built binary**, live, without restarting the server:
  added a task file after startup and reloaded (picked up, ran on the next scheduler tick);
  edited its description and reloaded (definition updated, `lastRun`/`success` preserved);
  deleted the file and reloaded (task removed, `GET` back to empty). The server's own log
  showed `tasks.reload` with correct new/updated/removed counts and the calling admin's
  username at every step.

Phase 1 note: task-file validation checks that `user` is present but does **not** check
that the named user actually exists in the auth database (`internal/server/auth`) — that
check is deferred to first dispatch in Phase 2/3, not done at load time as originally
sketched in Gap #3 below. Reasoning: the auth subsystem is a separate, independently
initialized service, a load-time check could only ever catch the common case anyway (a
user can be deleted after the server starts), and keeping `load.go` free of an `auth`
import keeps its unit tests independent of auth initialization order.

## Context

Ego's server currently has no way to run recurring or startup-triggered work on its own —
anything periodic (cleanup, data refresh, calling another service on a timer) has to be
driven from outside the process (cron, an external script). This feature adds a task
subsystem: JSON files under `lib/tasks/` each describe one scheduled call to an Ego server
endpoint (method, body, expected status, a value-extraction/substitution mechanism, and a
repeat interval), a background scheduler dispatches them under a concurrency cap, and a new
`/admin/tasks` endpoint lets a root operator inspect, force-run, or deactivate them. Because
task files can carry credentials or sensitive request bodies, file-permission enforcement
(0600, owner-only) is a hard precondition for a task to be loadable at all.

## Task file format

One JSON file per task under `lib/tasks/`, comments allowed (`#` or `//` at the start of a
line, stripped before parsing):

```jsonc
{
	"task": "description of the task",
	"id": "a40452b9-91d3-45fc-a374-d271e81f308f",
	"active": "true",
	"user": "admin",
	"method": "post",
	"endpoint": "/services/jiggle",
	"parameters": {
		"source": "true"
	},
	"body": {
		"table": "mydata",
		"operation": "purge"
	},
	"status": 200,
	"save": {
		"TOKEN": "system.token"
	},
	"timeout": "5m",
	"repeat": "once"
}
```

Field semantics:

- `task` — free-text description, used only in logging.
- `id` — unique identifier (UUID recommended, not required). Duplicate IDs across files are
  a load-time error (see Gaps #4).
- `active` — whether the scheduler may run this task; `false` means loaded and validated at
  startup but never dispatched.
- `user` — identity the task runs as; the endpoint call carries this user's real, live
  permissions (see Dispatch mechanism below).
- `method`, `endpoint`, `parameters`, `body` — the request to make.
- `status` — expected HTTP status; on mismatch, the `save` block is skipped and a TASKS
  error is logged.
- `save` — map of `name: jsonPath` pulled out of the response body into a global,
  in-memory, cross-task substitution dictionary (`{{name}}` usable in a later task's
  `endpoint`/`parameters`/`body`).
- `timeout` — Go duration string, with an Ego extension allowing `d` for days (e.g. `"30d"`).
  Defaults to `ego.server.tasks.default.timeout`, clamped to `ego.server.tasks.max.timeout`.
- `repeat` — `"once"` (startup only) or a duration string (Ego `d`-extended) for recurring
  execution. The interval restarts from when the task *finishes*, not when it starts.

## Decisions made during design

- **Last-run persistence**: a sidecar state file (not the task JSON itself) tracks
  last-run time/status per task, so recurring schedules survive server restarts without
  ever rewriting — or risking comments in — the user-authored task file.
- **`save` dictionary scope**: one global, in-memory, cross-task key/value store (matches
  `tools/apitest`'s save/substitute model). Lost on restart; not persisted, since
  perpetuating secrets like tokens across restarts is its own can of worms.
- **Dispatch mechanism**: in-process. The scheduler mints a real bearer token via the
  existing `tokens.New()` (already used by the OAuth2 AS flow to hand a caller a token with
  no password check), builds an `*http.Request` with `httptest.NewRequest`, and drives it
  straight through the existing `Router.ServeHTTP` with an `httptest.NewRecorder()` — no
  TCP/TLS/reverse-proxy-prefix concerns, and it exercises the exact same auth/permission
  code every real caller goes through.
- **`DELETE /admin/tasks/{id}`**: deactivates by a surgical text patch of just the
  `"active"` field's value in the raw file (not a full JSON re-marshal), so hand-written
  comments in the task file survive.

## Gaps / concerns

1. **Config key naming** in the original spec was inconsistent (`ego.server.tasks.enabled`,
   `ego.server.task.default.timeout`, `ego.config.task.max.timeout`,
   `ego.sever.task.max.concurrent` — note the "sever" typo and `task`/`tasks` and
   `server`/`config` prefix drift). Normalized below to one `ego.server.tasks.*` family.
2. **Directory permissions, not just file permissions**: `lib/tasks/` itself is checked and
   enforced to `0700` at startup, in addition to the `0600` file check — a world-readable
   directory can leak filenames/existence even if file contents are protected.
3. **Unknown/invalid `user` field**: treated like any other load-time validation failure —
   logged to TASKS and marked not-runnable, without blocking the load of other tasks.
4. **Duplicate `id` across files**: first file wins (sorted filename order, deterministic);
   the second is rejected and logged as a TASKS error; the rest of the directory still loads.
5. **`"body"` vs `"request"`**: the field is `"body"`, matching the JSON sample (the prose
   in the original request used both terms for the same thing).
6. **Startup `"once"` tasks must not block server start** — dispatched onto the scheduler's
   worker pool asynchronously like any other due task, not run synchronously during
   `Initialize()`.
7. **Overdue recurring tasks**: if a restart happens after a task's interval has already
   elapsed (per the sidecar state), it's treated as immediately due and runs on the next
   scheduler tick rather than waiting a full fresh interval.
8. **Logging sensitive payloads**: task `body`/`parameters` and saved values (e.g. tokens)
   are never logged at normal TASKS verbosity — only task id/description, endpoint, status,
   duration, and pass/fail go to the default log line. Full bodies are never logged in the
   clear, matching how credentials are handled elsewhere in this codebase.
9. **No live task-reload endpoint** — only per-task run (`POST`) and deactivate (`DELETE`)
   are exposed. Adding/editing tasks requires a server restart to pick up new/changed files.
   Accepted as a v1 limitation.

## Config keys

Added to `internal/defs/config.go`, following the existing `ServerKeyPrefix`-block
convention, all under `TasksKeyPrefix = ServerKeyPrefix + "tasks."`:

| Key | Type | Default | Purpose |
|---|---|---|---|
| `ego.server.tasks.enabled` | bool | `false` | Master switch; when false, no task loading, no scheduler goroutine, no `/admin/tasks` route registered. |
| `ego.server.tasks.default.timeout` | duration | `"30s"` | Used when a task omits `"timeout"`. |
| `ego.server.tasks.max.timeout` | duration | `"1h"` | Hard ceiling; a task requesting more is clamped and logged. |
| `ego.server.tasks.max.concurrent` | int | `3` | Size of the scheduler's worker pool. |

All four need entries in `ValidSettings`. None need `RestrictedSettings` (nothing secret) or
special-casing in `ReadonlySetting` for `PATCH /admin/config` — they're safe to leave
patchable like other server tuning knobs.

## Package layout

New package `internal/server/tasks/`, mirroring `internal/server/tables/` and
`internal/server/admin/`:

- **`defs.go`** — `Task` struct (json tags matching the spec above), an in-memory
  `TaskState` (last run time, last status, running bool), and the package-level registry
  (`map[string]*Task` keyed by id, guarded by a mutex).
- **`load.go`** — directory scan of `lib/tasks/` (path resolved the same way
  `loadAllValidations()` resolves `lib/validations/` in
  `internal/router/validations.go:67-73`: `settings.Get(defs.LibPathName)` or fall back to
  `filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)`, then join `"tasks"`).
  For each `*.json` file: enforce/repair permissions (see `permissions.go`), read via
  `ui.ReadJSONFile` (`internal/cli/ui/json.go:14`) to strip comment lines, `json.Unmarshal`,
  validate required fields and `id` uniqueness, register into the task map.
- **`permissions.go`** — directory (`0700`) and file (`0600`) permission check-and-repair,
  generalizing the existing pattern in `internal/server/oauth/authserver/permissions.go`
  (`ensureMode`, ~line 88). A file that can't be fixed (e.g. `chmod` fails because the
  process doesn't own it) is skipped and logged via `ui.Log(tasksLogger, ...)`, not fatal to
  the rest.
- **`state.go`** — sidecar state file (e.g. `lib/tasks/.state.json`, itself kept at `0600`)
  recording `{id: {lastRun, lastStatus, success}}`; loaded once at startup, written after
  each task run.
- **`scheduler.go`** — background goroutine following the existing idiom
  (`internal/router/ratelimit.go`'s `time.Sleep` loop / `internal/server/oauth/oauth.go`'s
  `time.Ticker`, both wrapped in `util.SafeCall` for panic isolation, per the "NILPTR-6"
  precedent). Each tick: find due tasks (`active == true`, not currently running, and
  `now >= lastRun + repeat`, or never run), dispatch up to
  `ego.server.tasks.max.concurrent` at once via a buffered channel/semaphore, skip the rest
  until the next tick if the pool is full.
- **`dispatch.go`** — for one task: mint a token with
  `tokens.New(task.User, "", timeoutOrTaskTTL, defs.InstanceID, sessionID)`
  (`internal/language/tokens/new.go:60`), apply the global save-dictionary substitution to
  `endpoint`/`parameters`/`body` (porting the `{{key}}` substitution logic from
  `tools/apitest/dictionary/subs.go` into this package, since `apitest` is a separate Go
  module and can't be imported directly), build the request with
  `httptest.NewRequest(method, endpoint, bodyReader)` plus query params, run it through
  `router.ServerRouter.ServeHTTP(httptest.NewRecorder(), req)`, compare the resulting status
  to `task.status`, and on match run the `save` extractions via
  `internal/cli/parser.GetItem` (`internal/cli/parser/item.go:9`) against the response body,
  storing results in the global save map. Update `state.go` with the outcome either way.
- **`routes.go`** — `AddStaticRoutes(r *router.Router)`, called from
  `internal/commands/routes.go` the same way `tables.AddStaticRoutes(r)` is (routes.go:284),
  gated on `settings.GetBool(defs.TasksEnabledSetting)` so the routes don't even exist when
  the feature is off. Registers:
  - `GET /admin/tasks` — `.Permissions(defs.RootPermission)`, returns the task list with
    description/id/last-run/status from the registry + state.
  - `POST /admin/tasks/{id}` — same permission; runs the named task immediately (still
    counts against the concurrency cap) and resets its repeat timer from completion time.
    The reserved id `@reload` (`ReloadTaskID`, `reload.go`) is special-cased instead to
    re-scan `lib/tasks/` and merge the results into the running registry (`Reload`) — new
    files are added, edited files are updated in place (execution history preserved),
    and files that were deleted are forgotten. This is what lets an admin add, edit, or
    reactivate a task without stopping the server.
  - `DELETE /admin/tasks/{id}` — same permission; surgical text-patch of the `"active"`
    field in the on-disk file to `false`, and marks the in-memory task inactive.
  - Path constants added to `internal/defs/rest.go` next to the other `Admin*Path`
    constants: `AdminTasksPath = AdminPath + "tasks/"`,
    `AdminTasksIDPath = AdminTasksPath + "{{id}}"`.

New **TASKS** log class: added via `ui.DefineLogger("TASKS", false)`
(`internal/cli/ui/messaging.go:147`) at package init, rather than editing the static `iota`
block and its parallel `loggers` slice — avoids the two-list-in-lockstep footgun and needs
no changes to existing files.

## Startup wiring

In `internal/commands/server.go`'s `RunServer`, alongside the existing `auth.Initialize(c)` /
`dsns.Initialize(c)` calls (server.go:177, 182): if `settings.GetBool(defs.TasksEnabledSetting)`,
call `tasks.Initialize()` (loads tasks, starts the scheduler goroutine). Route registration
hooks into `defineStaticRoutes()` in `internal/commands/routes.go` next to
`tables.AddStaticRoutes(r)`.

## Verification

All items below are done, not just planned.

- **Unit tests** — one test file per source file, table-driven where it fits this repo's
  conventions: `permissions_test.go`, `load_test.go`, `state_test.go`, `scheduler_test.go`
  (fake-clock due-task selection, concurrency-limit races run under `-race`),
  `save_test.go` (substitution), `dispatch_test.go` (a genuine in-process router round
  trip, not a mock), `deactivate_test.go`, `routes_test.go`. Whole-package result: clean
  under `go test ./internal/server/tasks/... -race -count=10`.
- **End-to-end, against the real built binary**: an isolated `taskstest` CLI profile
  pointed `ego.runtime.path` at a temp directory, `ego.server.tasks.enabled=true`, one task
  file hitting `/admin/heartbeat` with `"repeat": "once"`. Verified via the server's own
  JSON log and live `curl` calls: startup dispatch succeeded (`tasks.run.complete`,
  status=200, success=true), `GET /admin/tasks` reported it, `POST /admin/tasks/{id}`
  returned 202 and re-ran it (new `lastRun`), `DELETE /admin/tasks/{id}` flipped
  `"active": "true"` to `"active": "false"` in place with every comment intact. See the
  Phase 4 notes above for the full transcript summary.
- **Permission enforcement**: covered both ways in the same end-to-end run and in unit
  tests. The real run started with the task file at `0644` and the tasks directory at
  default `0755`; the log showed both corrected to `0600`/`0700` before the task loaded.
  `TestLoadAllRejectsUnfixablePermissions` covers the unfixable case (a task file made
  unreachable via an inaccessible parent directory) at the unit level, since genuinely
  unfixable permissions require a different file owner, not reproducible in an automated
  same-user test run.
