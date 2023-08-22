# Ego Relese Notes

## Ego 1.4 "Sugar Free"

### Language Features

* Runtime close() calls Close method of a type if found.
* Ability to unwrap interface types with x.(type) notation.
* Switch statement with conditional cases.
* Switch statement assigning local value from switch value.

### Runtime Features

* Added strconv runtime package.
* Revised reflect package.
* Support exec package on Windows.
* Added string sealing functions to strings package.
* More correct unicode support in strings package.
* Removed blockprint functions from strings package.
* Runtime improvements in db, fmt packages.

### Server features

* Support for HTTP/HTTPS automatic redirect in server.
* Addition of @endpoint directive for HTTP services.
* Support for external authentication service for server.
* Support serving video as a server asset.
* WriteHeader function for HTTP services.
* Removed the /code endpoint.
* Use native versions of logging, admin services when Ego versions not found.
* Support for data source names in table and SQL access endpoints.

### Commandline Features

* Added STATS and SERVICES loggers.
* Added --project option to run all .ego code in a directory.
* Renamed --debug option to --log which better reflects its purpose.
* Added dsns subcommand group for managing data source names on a server.

### Bug Fixes

* Fix issues with relaxed type checking.
* Fix issues with concurrent access to symbol tables and values.
* Fix resource leaks in database handling.
* Performance improvements.

## Ego 1.3 "Acai Berry"

### Major new features

This release has several main themes:

* Further improvements in language compatibility with Go.
* More robust server operation, with new services.
* Internal code cleanup
* And many many many bug fixes.

The changes are expresed in the following sections.

#### Language

1. More fully evolved type system, supporting proper user-generated types.

2. Types can have receivers (by pointer or by value)

3. Types work correctly when exported from packages.

4. If-statements with assignment supported.

5. Switch statements with case selectors as expressions

6. Extended reflect() to return much more information.

7. Better syntax checking (managing of reserved words).

8. Support of slices notation with implied start or end values.

9. String length, ranging over a string, and table cell alignment operations are all now unicode-safe.

10. Package global variables.

11. Allow formatting of a function, which returns it's declaration.

12. The "print" command will attempt to format arrays, structures, and
    arrays of structures as a text table.

13. Support proper localization. Currently supports "en" localizations. This means column headings, error messages, and prompts can be localized. Note that language terms and log messages are not localizable.

14. Allow field lists in structure declarations.

15. Allow variable name lists in variable declarations.

16. Debuggers allows saving and re-loading breakpoint lists.

17. Proper support for []byte, modified json and file i/o to use the []byte class to match Go runtimes.

18. Added support for exec.Command, exec.LookPath, and methods on Command objects for executing commands and retrieving the stdout and stderr from the command. This requires a config file change to enable.

19. Added functions to "util" package to list symbol tables, list the contents of symbol tables, and list packages and their exported names.

20. Added modulo "%" operator to language.

#### Server Functionality

1. Support for /tables endpoints which allow basic SQL operations on a
   table controlled by the server, via REST endpoints. See the "Tables API"
   documentation for more information.

2. Support for TLS in network communications between client and server.

3. Heartbeat and "UP" service endpoints no longer require authorization token.

4. Additional logging classes. SQL for back-end database operations performed
    by the /tables endpoints, REST for logging REST call activity.

5. More complete support for media types in rest calls.

6. API versions are returned with all REST payloads.

#### Performance

1. Types are managed more efficiently in memory.

2. Added peephole optimier for common bytecode patterns.

3. Symbol table made more efficient for managing readonly state.

4. Internal package structure simplified, making imports faster.

#### Deprecated Features

1. Gremlin client functionality removed.

## Ego 1.2 "Jamba Juice"

First stable release ready to have the project set to Public.
