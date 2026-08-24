# Scheduled Server Tasks

Ego's server can run recurring or startup-triggered work on its own —
anything periodic (cleanup, data refresh, calling another service on a timer).
This means you do not need to use `cron` or an external script.

Tasks are defined as JSON files under `lib/tasks/`; each describes one scheduled call to an Ego server
endpoint (method, body, expected status, a value-extraction/substitution mechanism, and an
optional recurring interval), a background scheduler dispatches them under a concurrency
cap. These are managed by `Ego` during startup, and can be managed via the
`/admin/tasks` endpoint, which lets a "ego.root" user inspect, force-run, or deactivate them. Because
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
	"tests": [
		{
			"name": "purge count is a number",
			"query": "affected",
			"op": "ge",
			"value": "0"
		}
	],
	"timeout": "5m",
	"interval": "1h",
	"count": 24,
	"after": "10m"
}
```

Field semantics:

- `task` — free-text description, used only in logging.
- `id` — unique identifier (UUID recommended, not required). Duplicate IDs across files are
  a load-time error (see Gaps #4). The id `@reload` is reserved (see `POST /admin/tasks/@reload`
  below) and rejected at load time if a task file tries to use it.
- `active` — whether the scheduler may run this task; `false` means loaded and validated at
  startup but never dispatched.
- `user` — identity the task runs as; the endpoint call carries this user's real, live
  permissions (see Dispatch mechanism below).
- `method`, `endpoint`, `parameters`, `body` — the request to make.
- `status` — expected HTTP status; on mismatch, the `save` block is skipped and a TASKS
  error is logged.
- `save` — map of `name: jsonPath` pulled out of the response body into a global,
  in-memory, cross-task substitution dictionary (`{{name}}` usable in a later task's
  `endpoint`/`parameters`/`body`). One entry, `{{SESSIONID}}`, is preloaded automatically at
  startup with this server instance's UUID (`defs.InstanceID`) -- no `save` step needed to
  obtain it.
- `tests` — **optional** array of response validations, patterned after `tools/apitest`'s
  own response `tests` block (see Phase 8 notes for the full design writeup). Each entry:
  `name` (required, used for diagnostics), `query` (required, a dot-notation path evaluated
  against the response body), `value` (the expected value, `{{name}}`-substituted before
  comparing; ignored by `exists`/`not-exists`), and `op` (one of `eq` [default], `ne`, `lt`,
  `le`, `gt`, `ge`, `contains`, `not-contains`, `len`, `exists`, `not-exists`). Evaluated
  only when the status already matched (same gate as `save`, run independently of it); stops
  at the first failing check. A task whose status matched but whose `tests` did not all pass
  is still recorded as unsuccessful, with the first failing check's `name` reported as
  `failedTest` in `GET /admin/tasks` -- so a query of the tasks shows not just *that* a task
  failed, but *which check* failed it, when that's the reason.
- `timeout` — Go duration string, with an Ego extension allowing `d` for days (e.g. `"30d"`).
  Defaults to `ego.server.tasks.default.timeout`, clamped to `ego.server.tasks.max.timeout`.
- `interval` — Go duration string (Ego `d`-extended) for recurring execution: the task
  becomes due again this long after its *previous run finished* (not from when it started).
  **Optional.** If omitted, the task is one-shot: once eligible (see `after`), it runs
  exactly once and never again — the same as `"count": 1`. There is no `"once"` sentinel
  value anymore; if present, `interval` must always be a real duration.
- `count` — caps the total number of times the task will ever run, across restarts (the
  run count is persisted in the sidecar state file). **Optional**; zero or absent means no
  limit. Since a one-shot task (no `interval`) only ever gets one run regardless, `count`
  is only meaningful alongside `interval`: a `count` other than `1` with no `interval` is
  rejected at load time as an ambiguous task definition (it can never recur to use up a
  bigger budget, and never gets a first chance to run again after `1` — see Gaps #10). Use
  `count: 1` (or omit both fields) for an explicit one-shot task.
- `after` — Go duration string (Ego `d`-extended): a delay, measured from when the task
  was first loaded into the running registry, before it becomes eligible to run for the
  first time. **Optional**; absent means eligible immediately. Lets an admin stagger tasks
  so they don't all fire the moment the server starts (e.g. `"after": "30m"`). This only
  gates the *first* run — it is never reapplied to later recurrences of the same task.

## Config keys

The following configuration settings control the operation of `tasks` in Ego.

| Key | Type | Default | Purpose |
|---|---|---|---|
| `ego.server.tasks.enabled` | bool | `false` | Master switch; when false, no task loading, no scheduler goroutine, no `/admin/tasks` route registered. |
| `ego.server.tasks.default.timeout` | duration | `"30s"` | Used when a task omits `"timeout"`. |
| `ego.server.tasks.max.timeout` | duration | `"1h"` | Hard ceiling; a task requesting more is clamped and logged. |
| `ego.server.tasks.max.concurrent` | int | `3` | Size of the scheduler's worker pool. |

There is a `TASKS` log class that will report on activites by the server to load, validate,
schedule, and execute tasks.
