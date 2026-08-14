# Ego Configuration

When Ego is run, it attempts to locate configuration information, which it uses to set
runtime values like the Ego library path, login information, compiler settings, runtime
settings, etc.

Configuration data has a given name, called a `profile`. This name defines one of potentially
many possible profiles stored in the data. For example, a profile might be created for
every-day use, a second one created for performing administrative tasks, and a third used
when a server is started. These allow partitioning information like login token values,
encryption tokens, etc. as desired.

In short, an profile name always defines the named configuration data to be read in. A configuration
provider (file system or database) can hold multiple profiles at the same time. Only one profile
is active at a time, designated by the --profile global option on the command line or the
`EGO_PROFILE` environment variable.

If no named profile is given for an invocation of Ego, then the profile name `default` is
assumed.

## Configuration Persistence

The configuration can be read a number of ways. Absent any other settings, the configuration
is read from a directory named ".ego" located in the current user's home directory. If this
directory does not exist, then it is created and a default profile is created automatically
with appropriate defaults.

This can be overridden by using the EGO_CONFIG environment variable, which is either a
string containing a file path to a directory where the configuration information is found
(or created), or it is a URL to the configuration provider. This URL can be one of the
following schemes:

| Scheme | Description |
| ------ | ----------- |
| file:// | The text after the scheme is a file system path |
| postgres:// | The text after the scheme is a PostgreSql URL |
| sqlite3:/// | The text after the scheme is the file system path to a Sqlite database |

When EGO_CONFIG is a URL to a database, a database connection is opened (the URL _must_
contain any required authentication information) and the configuration data for the
current profile is read. When a configuration item is modified, it is rewritten back to
the database. The database configuration information is stored across two tables, named
"config_ids" and "config_items", which are created if they do not already exist in the
database URL.

When EGO_CONFIG is a file system path, the profile and encrypted values (stored as JSON)
are located in this directory. When a value is updated, the file is re-written, with a
timestamp in the JSON indicating when the profile was last modified.

Using a database to store the configuration information can have several possible benefits
or uses.

* When Ego is running in a container, it may not have local storage for configuration data.
* Using a database allows external administration of the configuration outside of Ego commands.
* Using a database allows configurations for multiple server instances and profiles to be stored in a common location for easy backups, restores, etc.

## Setting Options on the Command Line

Any Ego option can be explicitly set on the command line during invocation of Ego (either in
command line, REPL, or server mode) by specifying the `--set` global option on the command
line. The option is followed by a list of values. The values must not have spaces in them,
or they must be enclosed in quotes.

```sh
ego --set ego.compiler.extensions=true run foo.ego
```

This invocation sets the `ego.compiler.extensions`  configuration value to `true`, which
enables language extensions like the `print` command in the Ego language. This option is
followed by the rest of the Ego command (in this case, running a program named "foo.ego".

You can specify multiple configuration options on a single invocation:

```sh
ego --set ego.compiler.extensions=true,ego.runtime.exec=true run foo.ego
```

This sets both `ego.compiler.extensions` and `ego.runtime.exec` config items to `true`.
Note that if the option value has a space in it, the value must be enclosed in quotes.
Also, spaces are not allowed between comma-separated items.

## setting Options using Environment Variables

In addition to reading from the configuration, options can be set using environment variables.
Any option can be set by creating an environment variable with the option name, were all letters
are upper-case and the dot (".") is replaced by an underscore ("_") character. For example,
the above `--set` example could also be done using:

```sh
export EGO_COMPILER_EXTENSIONS=true
export EGO_runtime_EXEC=true
ego run foo.ego
```

The export operations define environment variables and these are read by _Ego_ when it starts
up. The order of precedence for option values is as follows:

1. If specified on a command line, that value is used.
2. If not on the command line, but present as an environment variable, that value is used.
3. If not on the command line or an environment variable, the configuration value is used.

This allows the use of environment variables and/or command line options to override the values
stored in the configuration. This is an alternative mechanism for defining configuration values
for Kubernetes or Docker containers where there may not be local persistent storage, but values
can be injected when the container starts via environment variables.

Environment variable values can be set either in the environment before the `ego` command is
run, or specified in the "env.json" file in the default configuration directory.

The "env.json" file is a simple JSON file located in the ".ego" subdirectory of the current user's home
directory. This JSON file contains an object describing the environment variables and their values to
apply before running the _Ego_ command. For example,

```json
{
    "EGO_GRAMMAR": "verb",
    "EGO_COMPILER_EXTENSIONS": "true"
}
```

This file creates two environment variables before the _Ego_ command processor starts. The first defines
the grammar to use (the "verb" grammar, in this case) as well as setting a default configuration option
to set `ego.compiler.extensions` to `true`.

## Command Line Environment Variables

Some command line options have environment variable equivalents as well. Some of these correspond
to configuration values, though some are just used as a way of making CLI default values for
various options.

| Name | Description |
| ---- | ----------- |
| EGO_USERNAME | The default username when logging into a server |
| EGO_PASSWORD | The default password when logging into a server |
| EGO_LOGON_SERVER | The default server to logon on receive a token |
| EGO_INSECURE_CLIENT | Ignore missing server certificates when talking to a remote server |
| EGO_PROFILE | The default profile name to use to read the configuration data |
| EGO_DEFAULT_LOGGING | A comma-separated list of loggers to enable |
| EGO_LOG_FILE | The name of the output log file (defaults to console) |
| EGO_LOCALIZATION_FILE | Name of a JSON file containing additional string localizations |
| EGO_OUTPUT_FORMAT | Default output format, "text", "json", or "indented" |
| EGO_LOG_FORMAT | Default log fie format, "text" or "json" |
| EGO_QUIET | If "true", suppress extraneous confirmation output messages |
| EGO_MAX_PROCS | If set, integer value for maximum number of CPUS to allocate for threads |
| EGO_LOG_ARCHIVE | If set, default .zip file in which to store archived log files |
| EGO_PORT | Default port number to assign to REST server (default is 443/80) |
| EGO_REALM | String for realm-based password challenges from browsers to REST server |
| EGO_TYPES | Specify _Ego_ language typing, "strict", "relaxed", or "dynamic" |
| EGO_TRACE | If "true", enable runtime tracing of _Ego_ programs |

## All Configuration Variables

Here is a table of all currently-defined Ego configuration key values:

| Key | Description |
| --- | ----------- |
| ego.application.server | URL of the application server. If not specified, use logon server |
| ego.compiler.extensions | Support language extensions |
| ego.compiler.full.stack | Display full stack contents during trace logging |
| ego.compiler.import | Automatically import common packages |
| ego.compiler.normalized | Symbol names are case-insensitive |
| ego.compiler.optimize | Enable bytecode optimizer |
| ego.compiler.types | Specify strict, relaxed, or dynamic types |
| ego.compiler.type.shadowing | If true (the default), var names can shadow reserved types (same as Go) |
| ego.compiler.unknown.var.error | If true, variables referenced without being set are an error |
| ego.compiler.unused.var.error | If true, variables created or set but not read are an error |
| ego.compiler.var.usage.logging | If true, include COMPILER log messages for variable usage |
| ego.console.auto.help | Display help text when incomplete commands are given to CLI |
| ego.console.history | Location of console history file |
| ego.console.log | Specify log format of text, json, or indented |
| ego.console.no.copyright | Suppress the copyright message |
| ego.console.output | Specify output destination of stdout or file |
| ego.console.prompt.missing.options | If true, prompt for missing required option values on commands |
| ego.console.readline | Specify if the Unix-style readline package is used |
| ego.log.archive | Name of archive zip file for purged log files, if any |
| ego.log.retain | Number of log files to retain before purging |
| ego.log.timestamp | Timestamp format string for log messages |
| ego.logon.server | URL of server to authenticate with |
| ego.logon.token | Current logon token |
| ego.logon.token.expiration | When the current logon token will expire |
| ego.runtime.deep.scope | If true, all symbol tables in scope are visible |
| ego.runtime.exec | If true, allow os.Exec() operations |
| ego.runtime.panics | If true, panic() causes Ego to panic |
| ego.runtime.path | The current EGO_PATH value |
| ego.runtime.path.lib | The location of the lib directory. Defaults to EGO_PATH |
| ego.runtime.precision.error | If true, conversions that result in data loss are an error |
| ego.runtime.rest.compression | If true, REST client calls accept gzip-compressed response payloads |
| ego.runtime.rest.errors | If true, REST API errors are returned as runtime errors |
| ego.runtime.rest.server.cert | Location of the server CERT file |
| ego.runtime.rest.timeout | When present, duration of REST timeout value |
| ego.runtime.sandbox.path | Root path for sandboxed I/O |
| ego.runtime.stack.trace | If true, show partial stack contents during trace |
| ego.runtime.symbol.allocation | Default allocation size of symbol table extensions |
| ego.runtime.timezone | Reference zone ("local", "UTC", or an IANA name) used to resolve bare zone abbreviations |
| ego.runtime.unchecked.errors | If true, unchecked errors are returned as runtime errors |
| ego.server.ai.endpoint | URL of the Ollama-compatible AI text-generation endpoint used by `POST /dsns/_dsn_/tables/@generate` (default is `http://localhost:11434/api/generate`) |
| ego.server.ai.model | Model name passed to the AI endpoint configured by ego.server.ai.endpoint; required to enable `@generate` — there is no default |
| ego.server.ai.timeout | Maximum time to wait for a response from the AI endpoint configured by ego.server.ai.endpoint (e.g. "120s"); default is "120s" |
| ego.server.allow.passkeys | If true, the server will allow FaceID/TouchID passkeys |
| ego.server.cache.size | Number of service programs to cache in memory |
| ego.server.child.services | Use child processes to execute services instead of threads |
| ego.server.child.services.dir | Location for transient request and response files (default is /tmp) |
| ego.server.child.services.limit | Maximum number of child services to run simultaneously |
| ego.server.child.services.retain | If true, keep child service payload files after service ends |
| ego.server.compression.threshold | Smallest response payload size in bytes that the server will compress |
| ego.server.database.empty.filter.error | If true, empty filter values are treated as errors |
| ego.server.database.empty.rowset.error | If true, empty rowset values are treated as errors |
| ego.server.database.partial.insert.error | If true, partial inserts are treated as errors |
| ego.server.default.credential | Default username:password to configure server |
| ego.server.default.log.file | Name of default server log file |
| ego.server.default.logging | Default logging classes to enable when starting server |
| ego.server.idle.timeout | Maximum time a keep-alive connection may remain idle before being closed (e.g. "120s") |
| ego.server.insecure | If true, server does not accept HTTPS connections |
| ego.server.js.minify | If true, JavaScript assets are minified before being cached and served |
| ego.server.js.shortvarnames | If true, the JavaScript minifier also renames local variables to short generated names |
| ego.server.max.body.size | Maximum request body size in bytes; requests larger than this are rejected with 413 (default 32 MiB) |
| ego.server.memory.log.interval | The duration between server memory usage log entries |
| ego.server.panic.recovery | If true (the default), a panic in a request handler is logged and returned as HTTP 500 instead of dropping the connection. Set false during development to let panics propagate |
| ego.server.piddir | Directory where server PID files are stored |
| ego.server.plaintext.passwords | If true, legacy {quoted} plaintext passwords are accepted and migrated to bcrypt |
| ego.server.read.header.timeout | Maximum time allowed to receive all HTTP request headers before closing the connection (e.g. "10s") |
| ego.server.read.timeout | Maximum time allowed to read the complete HTTP request including body (e.g. "30s") |
| ego.server.report.fqdn | If true, report fully qualified server name in REST responses |
| ego.server.start.log.age | Age of oldest entries in system start log database |
| ego.server.superuser | The username:password to create as the default admin account |
| ego.server.token.expiration | Default expiration value applied to auth tokens |
| ego.server.token.key | Generated random key encryption value used by server operations |
| ego.server.userdata | File or database URL of the credentials database |
| ego.server.userdata.key | The encryption key for the userdata file if stored as text |
| ego.server.webauthn.rpid | Relying Party ID (domain name) used for WebAuthn passkey authentication |
| ego.server.write.timeout | Maximum time allowed to send the complete HTTP response (e.g. "120s") |
| ego.table.autoparse.dsn | If true, multipart names are assumed to be the dsn and table |
| ego.table.default.dsn | The default data source name to use with tables commands |

Note that values that start with "ego." are reserved to _Ego_. You cannot create additional configuration
items with that prefix. However, you can create additional configuration values with any other prefix
(such as "app." or whatever makes sense for your usage of _Ego_). These configuration values are stored
and managed identically to the _Ego_ configurations values. You can access these values from within
an _Ego_ program using the profile.Get() and profile.Set() functions.

## Configuration Option Reference

The table above lists every configuration key alongside the short string shown by `ego config`
(sourced from the localization catalog). This section goes a level deeper: for each option it
explains, based on the current source code, what actually changes at runtime when the value is
set, what the effective default is if the key is never set at all, and — for the handful of keys
that gate internal performance work — when it's worth changing the value while tracking down a
bug rather than just tuning performance.

A few keys in the table above use names that no longer match any setting in the code (for
example the table lists `ego.server.cache.size`, but the actual keys are
`ego.server.cache.maxsize` and `ego.server.service.cache.size`, which are two different caches —
see [Server settings](#server-settings) below). Where this section and the table disagree, this
section reflects the current code and is authoritative. Settings added since the table was last
updated (the OAuth2, cluster, and several server/runtime keys) only appear here.

Unless otherwise noted, a boolean setting that has never been set at all (no profile entry, no
`--set`, no environment variable) reads as `false`. A number of settings override that rule
explicitly in code — those are called out below.

### Compiler optimization and performance settings

These four settings control the compiler and runtime optimizations that were added to reduce
the cost of name-based symbol lookup (see `docs/internals/SLOTS.md` and
`docs/internals/GLOBALS.md`, the design docs behind PERFORMANCE.md Findings 7 and 17). They are
independent kill-switches layered on top of each other, not a single on/off toggle, so it's worth
understanding how they interact before changing any of them while chasing a bug.

| Setting | Type | Effective default | Controls |
| ------- | ---- | ------------------ | -------- |
| `ego.compiler.optimize` | integer | 3 | Whether the bytecode peephole optimizer runs at all, and how aggressively |
| `ego.compiler.registers` | bool | `false`, unless opt level > 2 | Whether eligible local variables compile to integer register slots instead of name-based symbol table lookups |
| `ego.compiler.constfold` | bool | `true` | Whether a package-level `const` reference folds to a literal at compile time instead of a runtime `Load` |
| `ego.runtime.globalcache` | bool | `true` | Whether a resolved reference to a package-level global (a `var`, or a `const` too complex to fold) is cached on the bytecode so repeat executions skip the scope walk |

**Optimizer Levels (expressed as integer):**

* `0` — the optimizer never runs.
* `1` — the optimizer runs only when the bytecode looks like it would benefit (large enough, or
  contains a loop.
* `2` (or higher) — the optimizer always runs, regardless of size.
* Any value greater than `2` also causes `ego run`/`ego test` to force `ego.compiler.registers`,
  `ego.compiler.constfold`, and `ego.runtime.globalcache` to `true`. The assumption is that for
  this level of optimization, you'd want maximum compiler and runtime performance.

**`ego.compiler.registers`** (docs/internals/SLOTS.md) governs whether a function's parameters
and `:=`/`var` block locals — for functions proven not to contain a capturing closure, `go`, or
`defer` — are assigned to integer register slots at compile time, bypassing the symbol table's
name-based map lookup at runtime. It defaults to `false` when absent, and is independent of the
peephole optimizer level (so `ego test`, which always disables the optimizer, can still exercise
registers if explicitly turned on). This is purely a performance path with no observable semantic
difference — introspection (`show symbols`, `print`, error formatting) sees slotted locals by
name exactly as before. If you suspect a bug in how local variables are being read, written,
addressed (`&x`), or captured, and the symptom only appears (or only disappears) depending on this
setting, that's a strong signal the bug is in the register-slot path specifically
(`internal/language/compiler/slots.go`, `internal/language/bytecode/slots.go`) rather than the
name-based path shared by everything else.

**`ego.compiler.constfold`** (docs/internals/GLOBALS.md, PERFORMANCE.md Finding 17, "Tier 1")
governs whether a reference to a package-level `const` in the same compilation unit is folded
directly into a literal `Push` instruction at compile time, instead of emitting a runtime `Load`.
Unlike registers, this defaults to `true` when the key is absent, because the compiler's purity
check (in `compileConst`) already guarantees a foldable value has no side effects, and the
folding logic refuses to fold any name a local declaration has shadowed — so, unlike registers,
turning it off should never change a program's behavior, only its speed. If a program that
references a package `const` from deep recursion is unexpectedly slow, or conversely a value you
expected to be a compile-time constant doesn't behave like one (e.g. in a disassembly listing),
try `--set ego.compiler.constfold=false` to compare against the unoptimized name-based `Load`
path and confirm whether folding is implicated.

**`ego.runtime.globalcache`** (docs/internals/GLOBALS.md, Finding 17 "Tier 2") is the runtime
counterpart to `constfold`, and covers the cases `constfold` doesn't: a package-level `var`, or a
`const` too complex to fold at compile time. When a `Load`/`Store`/`AddressOf`/`Deref`
instruction resolves such a name to its owning global symbol table (the program's own top-level
table, or an imported package's table), the resolved table is cached on the compiled `*ByteCode`
instruction itself, so a later execution of the _same instruction_ - including from deep
recursion - skips the O(depth) walk through intervening call frames. It defaults to `true`, and
is a pure kill-switch: setting it `false` restores the always-correct, unoptimized name-based
walk on every access. If you're debugging something that smells like a stale or incorrectly
shared reference to a package `var` — particularly anything involving goroutines, closures, or
`InPackage` package-proxy boundaries, which is exactly the territory this cache's safety argument
had to be careful about — reproducing with `--set ego.runtime.globalcache=false` is the fastest
way to confirm or rule out the cache as the cause. `ego run` resets this to `true` at the start
of every invocation before applying the profile/CLI override, so unlike `optimize`, there's no
`defaults.json` parsing surprise here.

**Practical debugging workflow:** because all four settings default independently (three of the
four to "on" in some form), a puzzling perf regression or a subtle correctness bug in
variable/const/global handling is usually fastest to isolate by turning them off one at a time —
`--set ego.compiler.registers=false`, then `constfold=false`, then `globalcache=false` — and
re-running against a baseline of all three off (equivalent to Ego's behavior before any of this
optimization work landed) to bisect which layer is responsible.

### Logon and authentication settings

| Setting | Settable via `--set` | Description |
| ------- | :---: | ----------- |
| `ego.application.server` | no | Base URL of the server providing application services (typically the same as the logon server; set this only when application services are hosted separately from logon). Managed programmatically, not by the user directly. |
| `ego.logon.server` | yes | Base URL of the server used to authenticate `ego logon`. |
| `ego.logon.token` | no | The token produced by the last successful `ego logon`, used by default for admin commands and REST calls. Written by the logon flow, not user-editable, and excluded from `profile.Get()` in Ego programs (see `RestrictedSettings`). |
| `ego.logon.token.expiration` | no | Expiration timestamp recorded from the last logon, used to give a clearer "token expired" message instead of a bare "not authorized". |
| `ego.logon.refresh.token` | no | OAuth2 refresh token obtained during an `ego logon --oauth`, used to silently renew an expired access token on the next logon without prompting. |
| `ego.logon.oauth.server` | yes | Explicit OAuth2/OIDC issuer URL for CLI logins; overrides the auto-detected `ego.server.oauth.as.issuer`/`ego.server.oauth.provider` order. |
| `ego.logon.oauth.client.id` | yes | OAuth2 `client_id` the CLI presents when starting an Authorization Code flow. Defaults to `ego-cli`, the public client Ego's own Authorization Server pre-registers. |
| `ego.logon.oauth.scopes` | yes | Space-separated OAuth2 scopes requested during `ego logon --oauth`. `openid` is always included. Default: `"openid profile"`. |

### Logging settings

| Setting | Description |
| ------- | ----------- |
| `ego.log.timestamp` | Go-style timestamp format string used to prefix log messages. Default (set on first profile init): `"2006-01-02 15:04:05"`. |
| `ego.log.archive` | Name of a `.zip` file that rolled-over log files are copied into. If unset, rolled-off logs are simply deleted. |
| `ego.log.retain` | Number of old log files to keep before purging in server mode. Default: `3`. |

### Runtime settings

| Setting | Description |
| ------- | ----------- |
| `ego.runtime.path` | Filesystem location used to find `services`, `lib`, and `tests` directories (`EGO_PATH`). |
| `ego.runtime.path.lib` | Overrides just the `lib` directory location (e.g. to point at `/usr/local/lib`). Defaults to `<runtime.path>/lib`. |
| `ego.runtime.suppress.library.init` | If `true`, skip automatically creating the `lib/` directory tree on startup. |
| `ego.runtime.exec` | If `true`, allows `util.Exec()` to run an arbitrary native shell command from Ego code. Defaults to `false`; this is a real privilege-escalation surface, so only enable it for trusted scripts. |
| `ego.runtime.insecure.client` | If `true`, the REST client accepts servers with missing/invalid TLS certificates. For dev/test use only. |
| `ego.runtime.precision.error` | If `true`, a numeric cast that loses precision (e.g. converting an out-of-range value into a `byte`) is a runtime error instead of silently truncating. |
| `ego.runtime.symbol.allocation` | Initial/growth allocation size for symbol table storage. Default: `32`. Larger values reduce reallocation overhead for programs with very large symbol tables at the cost of some wasted memory for small ones; rarely worth changing. |
| `ego.runtime.unchecked.errors` | If `true`, calling a function that returns `(value, error)` without assigning the error is itself a runtime error. |
| `ego.runtime.panics` | If `true`, Ego's `panic()` builtin triggers an actual Go-level panic (crashing the process) instead of being handled as an Ego runtime error. |
| `ego.runtime.deep.scope` | If `true`, all symbol tables currently in scope are visible to a running statement, rather than being bounded by function call barriers. Mostly relevant to test/debug tooling that needs to see across scope boundaries. |
| `ego.runtime.stack.trace` | If `true`, the `@trace` directive/tracing output prints the full call stack instead of a single-line summary. |
| `ego.runtime.float.div.zero.error` | If `true`, dividing a float by zero is a runtime error. Default `false` matches Go's own behavior of producing `+Inf`/`-Inf`/`NaN`. |
| `ego.runtime.sandbox.path` | If set, every file path an Ego program touches (e.g. via `ReadFile()`) must be under this path, or is silently prefixed with it. Primarily used to confine server-mode file I/O. |
| `ego.runtime.timezone` | Reference timezone used to give meaning to a bare zone abbreviation (`"EST"`, `"CST"`) in a string passed to `time.ParseAny()`. An IANA name such as `America/New_York`, or `UTC`, or `local` (the default) meaning "whatever this host is configured for". See [Timezones and `time.ParseAny()`](#timezones-and-timeparseany) below — the default is a guess, and a deployment that cares should set this explicitly. |

**REST client settings** (`ego.runtime.rest.*`), which affect Ego programs making outbound REST
calls and the `ego` CLI's own calls to a server:

| Setting | Description |
| ------- | ----------- |
| `ego.runtime.rest.errors` | If `true` (the default), a non-success `ErrorResponse` payload from a REST call is surfaced as an Ego runtime error rather than just being returned as data. |
| `ego.runtime.rest.timeout` | Duration string (e.g. `"10s"`) bounding how long a REST client call waits. Default: `10s`. |
| `ego.runtime.rest.server.cert` | Path to a server certificate to trust for REST calls. Set to `"system"` to use the OS trust store instead of loading a specific file. |
| `ego.runtime.rest.compression` | If `true` (the default), REST client calls advertise gzip support via `Accept-Encoding`, so a willing server can send a compressed body. Response bodies are transparently decompressed either way — this setting only changes what goes over the wire, useful to turn off when capturing raw traffic with a packet analyzer. |

#### Timezones and `time.ParseAny()`

`ego.runtime.timezone` exists because a timezone _abbreviation_ is not enough information to
locate a moment in time, and `time.ParseAny()` accepts strings that contain one.

Three kinds of timestamp reach `time.ParseAny()`, and only the third is ambiguous:

| Input | Result | Why |
| ----- | ------ | --- |
| `"Dec 7, 1959"` | `1959-12-07 00:00:00 +0000 UTC` | Names no zone at all. Read as UTC, matching Go's own `time.Parse()`. |
| `"2024-01-15T10:00:00-08:00"` | `2024-01-15 10:00:00 -0800` | States its offset numerically. There is exactly one instant it can mean. |
| `"December 7, 1959 10:35am EST"` | depends on `ego.runtime.timezone` | `"EST"` is three letters with no offset attached. |

**`ego.runtime.timezone` only affects the third row.** A string with no zone information is
still read as UTC, and a numeric offset always wins, so setting this cannot shift the meaning
of a timestamp that was already unambiguous — including a Unix epoch value like `"1500000000"`,
which is an absolute instant by definition.

**Why abbreviations are genuinely ambiguous.** There is no global registry of them, and they
collide. `"CST"` is US Central Standard Time (−06:00), China Standard Time (+08:00), and Cuba
Standard Time (−05:00). `"IST"` is Indian, Irish, and Israel Standard Time. Go can only turn
an abbreviation into an offset by looking it up in the zone table of some _particular_
location, so something has to choose that location — there is no correct answer derivable from
the input string alone. That is what this setting names:

```bash
ego config set ego.runtime.timezone=America/Chicago   # "CST" means -06:00
ego config set ego.runtime.timezone=Asia/Shanghai     # "CST" means +08:00
```

The value is an IANA timezone name (`America/New_York`, `Europe/Paris`, `Asia/Tokyo`), the
word `UTC`, or the word `local`. Ego embeds a copy of the IANA timezone database, so a named
zone loads even on a slim container image that ships no timezone data of its own.

**What `local` means, and why it is only a guess.** `local` is the value a new configuration
starts with, and it means "resolve abbreviations against whatever timezone this host is
configured for". Ego takes that from the `TZ` environment variable if one is set, otherwise
from the host's `/etc/localtime`, and otherwise falls back to UTC. That last fallback is the
catch: a container or minimal server install usually has neither, so its local zone is UTC —
where no regional abbreviation resolves at all, and `"EST"` silently comes out as `+0000`.
This is backwards from what a user would want, since the environment least likely to know what
`"EST"` means is also the most common deployment target.

Nothing better is available to guess with. Go exposes no other locale information, and a
language or country locale would not settle the question anyway — the United States spans six
timezones. **If your programs parse timestamps containing abbreviations, set
`ego.runtime.timezone` explicitly rather than relying on `local`.** That is the only way to get
the same answer on a developer laptop and in production. A test or a single run can pin it
without touching the saved configuration:

```bash
ego --set ego.runtime.timezone=America/New_York run report.ego
```

**When the abbreviation isn't one the reference zone uses** — parsing `"JST"` against
`America/New_York`, for instance — the name is kept and the offset is zero. This is not
reported as an error, because it is what `time.ParseAny()` has always returned for an
abbreviation it could not resolve, and making it an error would break existing programs that
tolerate the zero offset. A caller that needs certainty should arrange for a numeric offset in
the input instead. Setting `ego.runtime.timezone` to a name Go cannot load _is_ reported as an
error, but only on a call that actually needed a reference zone.

**Database table columns are stricter.** The same setting decides what an abbreviation means
for a `timestamp`, `date`, or `time` column, but there an abbreviation the reference zone
cannot resolve is _rejected_ rather than given a zero offset — the insert or update fails and
no row is written. The tradeoff comes out differently because the value is being stored: a
wrong offset in a running program is transient, while a wrong offset written to a column is
normalized to a UTC instant, becomes the record, reads back cleanly forever after, and cannot
be repaired without knowing how the server was configured at the moment of the write. RFC 3339
is the documented format for these columns and states its offset numerically, so it is
unaffected either way. See [TABLES.md](TABLES.md) under "Timestamp values".

Note that for table columns the reference zone is the one configured on the _server_ storing
the row, not on the client that sent it.

### Compiler settings (non-optimizer)

| Setting | Description |
| ------- | ----------- |
| `ego.compiler.disasm.packages` | If `true`, imported packages are included when a disassembly listing is produced (normally only the main program is shown). |
| `ego.compiler.normalized` | If `true`, symbol names are folded to a common case, making them effectively case-insensitive. Default `false` (case-sensitive, matching Go). |
| `ego.compiler.extensions` | If `true` (the default), language extensions beyond standard Go-like syntax are available — `print`, `call`, `try`/`catch`, etc. |
| `ego.compiler.import` | If `true` (the default), an interactive session automatically imports the common built-in packages instead of requiring explicit `import` statements. |
| `ego.compiler.full.stack` | If `true`, tracing output during compilation lists the full stack contents rather than an abbreviated form. |
| `ego.compiler.types` | One of `"strict"`, `"relaxed"`, or `"dynamic"` (the default), controlling Ego's static/dynamic type enforcement mode. |
| `ego.compiler.type.shadowing` | If `true` (the default, matching Go), a local variable may shadow a built-in type name (`int := 5`). Set `false` to make that a compile-time error — useful in teaching contexts where an accidental shadow is a common, confusing mistake. |
| `ego.compiler.unused.var.error` | If `true` (the default), a variable that is declared/assigned but never read is a compile error. |
| `ego.compiler.unknown.var.error` | If `true`, the compiler reports an unknown-symbol error at compile time itself instead of waiting for the runtime symbol table manager to report it. Marked in code as "somewhat experimental". |
| `ego.compiler.var.usage.logging` | If `true`, compiler logging includes detailed variable usage/scope tracking messages (`COMPILER` log class). Diagnostic use only. |

### Console settings

| Setting | Description |
| ------- | ----------- |
| `ego.console.prompt.missing.options` | If `true` (the default), the console prompts interactively for required command options that weren't supplied, instead of just failing. |
| `ego.console.auto.help` | If `true`, an incomplete CLI command automatically shows help text; if `false` (the default), it shows a terser list of expected terms instead. |
| `ego.console.history` | Path to the readline history file. Defaults to a location under the profile directory. |
| `ego.console.no.copyright` | If `true`, suppresses the copyright banner in interactive mode. Not settable via `--set`. |
| `ego.console.readline` | If `true` (the default), the interactive console uses the Unix-style readline library for line editing/history. |
| `ego.console.interactive` | Internal-only flag indicating the console is running as a REPL (enables function redefinition without an "already exists" error). Not intended to be set directly. |
| `ego.console.output` | Default output format for commands that support multiple formats: `"text"` (the default), `"json"`, or `"indented"`. |
| `ego.console.log` | Default log message format: `"text"`, `"json"`, or `"indented"`. |

### Table settings

| Setting | Description |
| ------- | ----------- |
| `ego.table.autoparse.dsn` | If `true`, a table command line naming `foo.bar` is parsed as DSN `foo`, table `bar`, instead of a literal table name. |
| `ego.table.default.dsn` | Default data source name assumed by table commands when none is given explicitly. |

### Server settings

Core server behavior (`ego.server.*`):

| Setting | Description |
| ------- | ----------- |
| `ego.server.ai.model` | Model name passed to the AI endpoint configured by `ego.server.ai.endpoint`, used by `POST /dsns/_dsn_/tables/@generate`. **Required to enable that endpoint — there is no default.** If unset or empty, `@generate` requests fail with `503 Service Unavailable` rather than silently falling back to a model choice that would inevitably go stale. |
| `ego.server.ai.endpoint` | URL of the Ollama-compatible AI text-generation endpoint used by `@generate`. Default `"http://localhost:11434/api/generate"`. |
| `ego.server.ai.timeout` | Duration string for how long to wait for a response from the AI endpoint configured by `ego.server.ai.endpoint`. Default `"120s"`. |
| `ego.server.report.fqdn` | If `true`, REST responses report the server's fully-qualified domain name instead of the short hostname. |
| `ego.server.default.credential` | `user:password` used as the root account when no user database has been initialized yet. |
| `ego.server.superuser` | `user:password` always granted superuser/root privileges regardless of the normal authorization data — an override, not the initial-setup credential above. |
| `ego.server.userdata` | File path or database URL where the credentials/user database is stored. |
| `ego.server.userdata.key` | Encryption key for the userdata file. If absent, the file is stored as plain, readable JSON — fine for development, not for production. |
| `ego.server.default.log.file` | Base name for the server log file (default `"ego-server.log"`); a datestamp is appended automatically. |
| `ego.server.memory.log.interval` | Duration between server memory-usage log entries. Defaults to every three minutes; an interval with no activity logs nothing, so setting this too long risks losing data during quiet periods. |
| `ego.server.cache.maxsize` | Maximum entries in each of the server's internal low-level caches (tokens, schemas, DSN permissions, etc. — distinct from the service-program cache below). Default `1000`; `0` disables that caching entirely. Lower it if a high-traffic server is spending too much memory on cache entries. |
| `ego.server.service.cache.size` | Maximum number of compiled service programs kept cached in memory. Default `20`. |
| `ego.server.authority` | Host that performs authentication on this server's behalf, when this server delegates auth to another Ego server rather than handling it itself. |
| `ego.server.auth.cache.scan` | Seconds between scans that expire cached authentication data pulled from a remote authority. Default: every `180`s. |
| `ego.server.auth.maxattempts` | Consecutive failed login attempts before an account is temporarily locked. Default `5`; `0` disables lockout entirely. |
| `ego.server.auth.lockout` | Duration string (e.g. `"15m"`) an account stays locked after exceeding `auth.maxattempts`. Default `"15m"`. |
| `ego.server.log.response` | If `true`, the server's own log content is included as the response payload when REST logging replies to a `/log` request. Debug-only; leave off otherwise. |
| `ego.server.compression.threshold` | Smallest response body size, in bytes, the server will gzip (only when the client also advertises gzip support). Default `4096`; `0` disables response compression entirely. |
| `ego.server.piddir` | Directory where server PID files are written for process management. |
| `ego.server.insecure` | If `true`, the server does not require HTTPS/TLS. |
| `ego.server.insecure.redirect` | If `true`, an HTTPS server also starts a plain HTTP listener on port 80 that redirects to HTTPS — useful when running HTTPS on 443 but still wanting to catch requests that arrive on 80. |
| `ego.server.default.port` | Port the server listens on when not given explicitly on the command line or via env var. |
| `ego.server.token.key` | Randomly generated key used to encrypt/sign auth tokens; created automatically on first run if absent. Not meant to be hand-edited. |
| `ego.server.token.expiration` | Duration string for how long a native Ego auth token remains valid. Default `"24h"`. |
| `ego.server.default.logging` | Comma-separated logger classes enabled by default when starting a server without an explicit `--log`. |
| `ego.server.start.log.age` | Days of server-start history to retain in the system database before old entries are purged on startup. Default `30`. |
| `ego.server.plaintext.passwords` | If `true`, legacy `{quoted}` plaintext passwords in the auth store are accepted and migrated to bcrypt on next successful login. If `false` (the default), such entries are rejected and logged as an error instead. |
| `ego.server.js.minify` | If `true`, JavaScript assets served from `/assets` are minified before being cached. |
| `ego.server.js.shortvarnames` | If `true` **and** `js.minify` is also `true`, the minifier additionally renames local variables/parameters to short generated names. No effect on its own. |
| `ego.server.webauthn.rpid` | Relying Party ID (a registrable domain suffix of the dashboard's origin) used for WebAuthn/passkey ceremonies. Passkey endpoints return `501 Not Implemented` while this is empty. |
| `ego.server.allow.passkeys` | If `true`, the server exposes passkey/WebAuthn functionality and the dashboard shows the passkey UI; if `false`, `/config` reports `passkeys:false` and the UI hides it. |
| `ego.server.dashboard.inactivity` | Duration string controlling how long the web dashboard waits for user activity before auto-signing out. Sent to the dashboard at logon so it doesn't rely on its own hard-coded default. Default `"15m"`. |
| `ego.server.read.header.timeout` | Max time to receive all HTTP request headers before the connection is closed — the main defense against Slowloris-style connection exhaustion. Default `"10s"`. |
| `ego.server.read.timeout` | Max time to read a complete request (headers + body). Default `"30s"`. |
| `ego.server.write.timeout` | Max time to send a complete response. Default `"120s"` — set generously for endpoints that return large payloads, like log retrieval. |
| `ego.server.idle.timeout` | Max time a keep-alive connection may sit idle before being closed. Default `"120s"`. |
| `ego.server.max.body.size` | Max accepted request body size in bytes; larger requests are rejected with `413` before the handler runs. Default 32 MiB. |
| `ego.server.max.item.limit` | Max items a single paged `GET` (via the `limit` query parameter) may request. Default `1000`; a caller-supplied `limit` above this is rejected with `400`. |
| `ego.server.panic.recovery` | If `true` (the default), a panic inside a request handler is caught, logged with its stack trace, and converted to an HTTP `500` — the server keeps running. Set `false` during development to let a panic propagate unmodified (Go's `net/http` still catches it at the connection level, but no `500` is sent and any lock the handler held is not released), when you specifically want to see the panic surface immediately. |

Child-service execution (`ego.server.child.services.*`) — running `/service` requests in a
separate OS process instead of an in-process goroutine, generally for stronger isolation:

| Setting | Description |
| ------- | ----------- |
| `ego.server.child.services` | If `true`, service requests run in a child process rather than in-process. |
| `ego.server.child.services.dir` | Where request/response payload files for child processes are written. Defaults to the system temp directory. |
| `ego.server.child.services.limit` | Maximum number of child service processes running simultaneously. |
| `ego.server.child.services.retain` | If `true`, keep a child process's response payload files after the request completes, for debugging. Normally deleted. |
| `ego.server.child.services.timeout` | Duration string for how long to wait for an available child process slot before giving up with an error. |

Database/table server enforcement (`ego.server.database.*`):

| Setting | Description |
| ------- | ----------- |
| `ego.server.database.empty.filter.error` | If `true` (the default), a destructive table operation (delete/update) issued with no filter is rejected as an error, rather than silently applying to every row. |
| `ego.server.database.empty.rowset.error` | If `true` (the default), an operation that would return/affect an empty row set is treated as an error. |
| `ego.server.database.partial.insert.error` | If `true` (the default), a table `insert` must specify every column; a partial insert is rejected. |

### Cluster settings

Standalone by default — these only matter when `--cluster` is used to join this server to a
named cluster of Ego server instances.

| Setting | Description |
| ------- | ----------- |
| `ego.cluster.name` | Name of the cluster this node belongs to. Empty (the default) means standalone mode; no cluster logic runs. |
| `ego.cluster.ping.interval` | How often the health-check goroutine pings each peer. Default `"30s"`. |
| `ego.cluster.ping.timeout` | Max time to wait for one peer health-check ping to complete. Default `"5s"`. |

### OAuth2 Authorization Server settings

These activate only when `ego.server.oauth.as.enabled` is `true`, and let an Ego server act as
its own OIDC-compliant Authorization Server — intended for development/testing; production
deployments should typically point at a dedicated IdP (Okta, Entra ID, Keycloak) via the Resource
Server settings below instead.

| Setting | Description |
| ------- | ----------- |
| `ego.server.oauth.as.enabled` | If `true`, this server registers the standard OIDC endpoints and acts as its own Authorization Server. |
| `ego.server.oauth.as.key.file` | Path to the PEM file holding the EC private key used to sign issued JWTs. Generated automatically (P-256, chmod 0600) on first use if missing. Default: `{EGO_PATH}/lib/oauth/oauth-signing.pem`. |
| `ego.server.oauth.as.clients` | Path to a JSON file listing the OAuth2 clients permitted to request tokens (client_id, secret, redirect URIs, grant types, scopes — see `docs/internals/OAUTH.md`). Default: `{EGO_PATH}/lib/oauth/oauth-clients.json`. |
| `ego.server.oauth.as.issuer` | This server's own base URL, reported as the JWT `iss` claim and used to build OIDC discovery URLs. Must exactly match the publicly reachable server URL. |
| `ego.server.oauth.as.token.expiration` | Lifetime of issued access tokens. Default `"1h"`. |
| `ego.server.oauth.as.refresh.expiration` | Lifetime of issued refresh tokens. Default `"24h"`. |
| `ego.server.oauth.as.code.expiration` | Lifetime of issued authorization codes. Default `"5m"` (the OAuth2 spec recommends codes stay short-lived). |

### OAuth2 Resource Server settings

These activate when `ego.server.oauth.provider` is non-empty, and configure how this server
validates JWT Bearer tokens issued by an _external_ identity provider.

| Setting | Description |
| ------- | ----------- |
| `ego.server.oauth.provider` | Base URL of the external OIDC provider; Ego appends `/.well-known/openid-configuration` to it for discovery. Setting this activates the Resource Server role. |
| `ego.server.oauth.client.id` | Client ID Ego was registered with at the provider. |
| `ego.server.oauth.client.secret` | Client secret from the provider. Treat as a password — don't commit it; `EGO_OAUTH_CLIENT_SECRET` takes precedence if set. |
| `ego.server.oauth.scopes` | Space-separated scopes requested during Authorization Code flow. Must include `openid`. |
| `ego.server.oauth.redirect.uri` | Callback URL the provider redirects to after login; must exactly match what's registered with the provider. |
| `ego.server.oauth.user.claim` | JWT claim used as the Ego username. Default `"sub"`; common alternatives are `"email"`, `"preferred_username"`. |
| `ego.server.oauth.permission.claim` | JWT claim carrying roles/groups/scopes used to derive Ego permissions. Default `"scope"`. |
| `ego.server.oauth.audience` | Expected JWT `aud` claim; tokens with a different audience are rejected. Empty skips audience validation (not recommended for production). |
| `ego.server.oauth.mode` | `"resource-server"` (JWT Bearer only), `"proxy"` (redirect through OAuth2, issue a native Ego token), or `"hybrid"` (both, default when `oauth.provider` is set). |
| `ego.server.oauth.jwks.cache.ttl` | How long the provider's JWKS signing keys are cached before re-fetching. Default `"1h"`. Shorter picks up key rotation faster; longer reduces round-trips. |
| `ego.server.oauth.permission.map` | Comma-separated `scope=permission` pairs mapping provider scopes to Ego permission names. Empty uses a built-in default table. |
