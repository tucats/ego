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
$ ego --set ego.compiler.extensions=true run foo.ego
```

This invocation sets the `ego.compiler.extensions`  configuration value to `true`, which
enables language extensions like the `print` command in the Ego language. This option is
followed by the rest of the Ego command (in this case, running a program named "foo.ego".

You can specify multiple configuration options on a single invocation:

```sh
$ ego --set ego.compiler.extensions=true,ego.runtime.exec=true run foo.ego
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
$ export EGO_COMPILER_EXTENSIONS=true
$ export EGO_runtime_EXEC=true
$ ego run foo.ego
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
| ego.compiler.extensions | Support language extensions |
| ego.compiler.import | Automatically import common packages |
| ego.compiler.normalized | Symbol names are case-insensitive |
| ego.compiler.optimize | Enable bytecode optimizer |
| ego.compiler.types | Specify strict, relaxed, or dynamic types |
| ego.compiler.unknown.var.error | If true, variables referenced without being set are an error |
| ego.compiler.unused.var.error | If true, variables created or set but not read are an error |
| ego.compiler.var.usage.logging | If true, include COMPILER log messages for variable usage |
| ego.console.auto.help | Display help text when incomplete commands are given to CLI |
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
| ego.runtime.path | The current EGO_PATH value |
| ego.runtime.precision.error | If true, conversions that result in data loss are an error |
| ego.runtime.rest.errors | If true, REST API errors are returned as runtime errors |
| ego.runtime.rest.timeout | When present, duration of REST timeout Value |
| ego.runtime.stack.trace | If true, show partial stack contents during trace |
| ego.runtime.symbol.allocation | Default allocation size of symbol table extensions |
| ego.runtime.unchecked.errors | If true, unchecked errors are returned as runtime errors |
| ego.server.child.services | Use child processes to execute services instead of threads |
| ego.server.child.services.dir | Location for transient request and response files (default is /tmp) |
| ego.server.child.services.retain | If true, keep child service payload files after service ends |
| ego.server.database.empty.filter.error | If true, empty filter values are treated as errors |
| ego.server.database.empty.rowset.error | If true, empty rowset values are treated as errors |
| ego.server.database.partial.insert.error | If true, partial inserts are treated as errors |
| ego.server.default.credential | Default username:password to configure server |
| ego.server.default.log.file | Name of default server log file |
| ego.server.default.logging | Default logging classes to enable when starting server |
| ego.server.insecure | If true, server does not accept HTTPS connections |
| ego.server.memory.log.interval | The duration between server memory usage log entries |
| ego.server.piddir | Directory where server PID files are stored |
| ego.server.report.fqdn | If true, report fully qualified server name in REST responses |
| ego.server.start.log.age | Age of oldest entries is system start log database |
| ego.server.token.expiration | Default expiration value applied to auth tokens |
| ego.server.token.key | Generated random key encryption value used by server operations |
| ego.table.autoparse.dsn | If true, multipart names are assumed to be the dsn and table |

Note that values that start with "ego." are reserved to _Ego_. You cannot create additional configuration
items with that prefix. However, you can create additional configuration values with any other prefix
(such as "app." or whatever makes sense for your usage of _Ego_). These configuration values are stored
and managed identically to the _Ego_ configurations values. You can access these values from within
an _Ego_ program using the profile.Get() and profile.Set() functions.
