# compiler

The `compiler` package is used to compile text in the _Ego_ language into
bytecode that can be executed using the `bytecode` package. This allows for
compiled scripts to be integrated into the application, and run repeatedly
without incurring the overhead of parsing and semantic analysis each time.

The _Ego_ language is loosely based on _Go_ but with some important
differences. Some important attributes of _Ego_ programs are:

* Ego supports optional language extensions (such as try/catch blocks)
* By default, Ego runs with relaxed type checking and will modify value
  types as needed.
* Strict type checking can be enabled when an Ego program is run.

See the documentation in the [Language Reference](../docs/LANGUAGE.md)  for more details
on the compiled language.
