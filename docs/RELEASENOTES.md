# Ego Relese Notes

## Ego 1.5 “Whole Grain”

### 1.5 Language Features

* Better compliance with Go implicit conversion of constant values.
* Better compliance with Go function return value typing
* Add support for if-else-if cascading conditional statements
* When extensions enabled, support for a type of “type” which contains an Ego type definition.
* Add @symbols directive to dump symbol table data
* Add @serialized directive to dump JSON expression of values.
* Support package alias names in import statements.

### 1.5 Runtime Features

* Add strings package functions like Replace, ReplaceAll, Trim.
* Add strings.Builder type and associated methods.
* Add strings package functions for Roman numerals
* Add tables package functions to read from table objects.
* Add time package object methods of After, Before, Clock
* Add fmt package Scan function.
* Move deprecated ioutil package functions to io package.
* Generate Ego stack trace when panic() called.
* Fix errors in how functions return a value of type error
* Fix errors in structure field ordering for output/formatting.
* Fix errors in symbol table scoping for nested local functions.

### 1.5 Server Features

* Support using data source names to securely hold database connection string information (including credentials) to be accessed by name by authorized users.
* Redact passwords in log messages during server operation.
* Server can be configured to run service requests in a child process.
* Better in-memory caching of heavily re-used objects like DSNS, authorization records, and token authentication.
* Logging updates for tracking service execution more closely.
* Improved standards compliance for security with HTTPS/HTTP redirects.
* Server can archive log files that age out to a .zip file

### 1.5 Commandline Features

* Shorten server status output unless -v option used.
* Ego will create lib directory and generate minimum required contents on first run if not already present.
* Support for creating, examining, and using data source names to read database tables.
* Add “help” command.
* Ego can act as a shell processor in Unix/Mac shell scripts (#!/bin/ego)

## Ego 1.4 "Sugar Free"

### 1.4 Language Features

* Runtime close() calls Close method of a type if found.
* Ability to unwrap interface types with x.(type) notation.
* Switch statement with conditional cases.
* Switch statement assigning local value from switch value.

### 1.4 Runtime Features

* Added strconv runtime package.
* Revised reflect package.
* Support exec package on Windows.
* Added string sealing functions to strings package.
* More correct unicode support in strings package.
* Removed blockprint functions from strings package.
* Runtime improvements in db, fmt packages.

### 1.4 Server features

* Support for HTTP/HTTPS automatic redirect in server.
* Addition of @endpoint directive for HTTP services.
* Support for external authentication service for server.
* Support serving video as a server asset.
* WriteHeader function for HTTP services.
* Removed the /code endpoint.
* Use native versions of logging, admin services when Ego versions not found.
* Support for data source names in table and SQL access endpoints.

### 1.4 Commandline Features

* Added STATS and SERVICES loggers.
* Added --project option to run all .ego code in a directory.
* Renamed --debug option to --log which better reflects its purpose.
* Added dsns subcommand group for managing data source names on a server.

### 1.4 Bug Fixes

* Fix issues with relaxed type checking.
* Fix issues with concurrent access to symbol tables and values.
* Fix resource leaks in database handling.
* Performance improvements.

## Ego 1.3 "Acai Berry"

### 1.3 Major new features

This release has several main themes:

* Further improvements in language compatibility with Go.
* More robust server operation, with new services.
* Internal code cleanup
* And many many many bug fixes.

The changes are expressed in the following sections.

#### 1.3 Language Features

* More fully evolved type system, supporting proper user-generated types.
* Types can have receivers (by pointer or by value)
* Types work correctly when exported from packages.
* If-statements with assignment supported.
* Switch statements with case selectors as expressions
* Extended reflect() to return much more information.
* Better syntax checking (managing of reserved words).
* Support of slices notation with implied start or end values.
* String length, ranging over a string, and table cell alignment operations are all now unicode-safe.
* Package global variables.
* Allow formatting of a function value, which returns it's declaration.
* The "print" command will attempt to format arrays, structures, and arrays of structures as a text table.
* Support proper localization. Currently supports "en" localizations. This means column headings, error messages, and prompts can be localized. Note that language terms and log messages are not localizable.
* Allow field lists in structure declarations.
* Allow variable name lists in variable declarations.
* Debuggers allows saving and re-loading breakpoint lists.
* Proper support for []byte, modified json and file i/o to use the []byte class to match Go runtimes.
* Added support for exec.Command, exec.LookPath, and methods on Command objects for executing commands and retrieving the stdout and stderr from the command. This requires a config file change to enable.
* Added functions to "util" package to list symbol tables, list the contents of symbol tables, and list packages and their exported names.
* Added modulo "%" operator to language.

#### 1.3 Server Features

* Support for /tables endpoints which allow basic SQL operations on a table controlled by the server, via REST endpoints. See the "Tables API" documentation for more information.
* Support for TLS in network communications between client and server.
* Heartbeat and "UP" service endpoints no longer require authorization token.
* Additional logging classes. SQL for back-end database operations performed by the /tables endpoints, REST for logging REST call activity.
* More complete support for media types in rest calls.
* API versions are returned with all REST payloads.

#### 1.3 Performance Features

* Types are managed more efficiently in memory.
* Added peephole optimier for common bytecode patterns.
* Symbol table made more efficient for managing readonly state.
* Internal package structure simplified, making imports faster.

#### 1.3 Deprecated Features

* Gremlin client functionality removed.

## Ego 1.2 "Jamba Juice"

First stable release ready to have the project set to Public.
