// Compiler is a package that compiles text in the _Ego_ language into pseudo-code.
// This pseudo-code  can then be executed using the bytecode package.
//
// _Ego_ is largely based on Go syntax and semantics, but has some important
// differences:
//
//   - Ego supports optional language extensions (such as try/catch blocks)
//   - By default, Ego runs with relaxed type checking and will modify value/
//     types as needed.
//   - Strict type checking can be enabled when an Ego program is run.
//
// The compiler functions by reading a string value that is entirely contained in
// memmory (there is no Reader interface). It generates a bytecode stream that is
// also stored in memory. The language includes "import" statements which will search
// for and read source files in the 'lib' directory if found, adding them to the
// current bytecode stream.
//
// The compiler is a top-down, recursive-descent compiler that works on a stream
// of tokens. Each token contains it's spelling and class (identifier, reserved,
// integer, string, etc). In this way, the tokenizer owns a part of the parsing
// of the code, to establish token meaning. The tokenizer is also responsible for
// creating composite tokens. For example "<" followed by "=" is converted to a
// single token "<=" by the tokenizer. Thus, the compiler can assume semantically
// correct individual tokens.
//
// The compiler processes the imput source for statements in a loop. Some statements
// are valid outside a block (const, var, type) and others can only be executed
// inside a block such as a function declaration. When a type or function definition
// is compiled, it is converted to an in-memory representation of the function (as
// bytecode) or type (as Type) and code is generated to load the function into the
// symbol table for the given object. In this way, code that is compiled in the REPL
// mode simply executes the code and stores the definition. In an import operation
// from source, the compiler executes the generated code for the import, and then
// captures the symbols created by the execution of the code. These symbols are
// then placed in the appropriate Package definition (for the given import package).
//
// While compiling source, any types that are compiled are also stored in the
// compillation metadata. This assists in recognizing when something is a type
// reference during compilation. There is a specialized interface to the type
// compiler functionality that can be used elsewhere in Ego (typically in the
// runtime packages) to compile a type definitoin at runtime for inclusion in
// a native Type definition).
//
// Compiler objects should be relatively cheap to instantiate, and in fact the
// compiler uses a dependent compiler for some operations that generate a bytecode
// sequence that the compiler is still deciding the context for it's use. For
// example, expressions are always initially compiled as bytecode via a sub-
// compiler instance. The resulting bytecode might then be appended directly to
// the active stream, modified for use (such as an lvalue being modified from a
// reference operation to a store operation), or deferred such as when compiling
// the various 'for' statement variations.
//
// The compiler supports "directives" which are instructions that immediately
// affect the compilation in some way, often by setting global state values
// or complex data types in the symbol table of the code. All directives start
// with the "@" character. The directive may change attributes of the active
// compiler, generate code, or both.
//
// There is no linkage phase to compilation. All identifiers are preserved by
// text name in the generated code, and are resolved at runtime. This is used to
// support the flexibility of untyped or relaxed type operations, or to enforce
// strict typing as requested when Ego is run.
//
// The compiler fails on the first error found. There is no "find the next valid
// source and keep trying". Compiler errors are reported using standard Ego Error
// objects, which include the module name and line number (retrieved from the
// tokenizer data) where the error occurred, as well as any context information.
//
// See the documentation in the Language Reference for more details on the compiled
// language.
package compiler
