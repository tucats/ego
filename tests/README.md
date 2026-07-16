# tests directory

This directory contains Ego tests. These act as unit tests of various
forms of Ego functionality, written in Ego and run by the `ego test`
command.

The "tests" directory is traversed recursively in alphabetical order to
find the test programs to execute. The directory tree is structured such
tht any Ego tests for a given runtime package are in `tests/{package-name}`.
For example, the `fmt` tests are in `tests/fmt/`. Additionally there are
directories basic language and runtime functionality:

- builtins: language builtin functions like `make` or `len`
- cast: test of data type cast functions like `int()`
- compiler: test o specific compiler features
- datamodel: tests of the basic Go data model
- defer: tests of the `defer` statement
- directives: tests of the various directives like `@capture`
- flow: tests of flow-of-control statements, like `if` or `switch`
- functions: tests of language `func` features
- packages: tests of user-written `package` files
- server: basic tests of client/server functionality
- types: basic test of language support for data types

See the docs/TESTING.md file for documentation on how to write tests.
