# Ego

**Ego is a scripting language with the syntax and feel of Go — run it interactively,
as a script, or as a full REST/web server, all from a single binary with no
runtime dependencies.**

Think of it as *Emulated Go*: if you already know Go, you already know most of Ego.
But unlike Go, Ego programs don't need to be compiled ahead of time — you type a line
and it runs immediately, values are dynamically typed by default, and you get a REPL
for free.

```sh
$ ego run
ego> fmt.Println(3*5)
15
ego> for i := 0; i < 3; i++ {
....     fmt.Println("hello", i)
.... }
hello 0
hello 1
hello 2
```

## Why Ego?

**Learning Go without the ceremony.** Ego's syntax is close enough to Go that lessons
transfer directly, but you don't need a build step, a `go.mod`, or an IDE to get started.
Type a statement, see the result, and iterate — it's a natural fit for classrooms,
workshops, and self-paced tutorials. See the
[Learning Ego](docs/LEARNING_EGO.md) guide for a from-scratch introduction.

**Rapid scripting for people who think in Go.** If your team already writes Go, Ego lets
you write throwaway tools, glue scripts, and automation without spinning up a full Go
project for every one-off task. Pipe a program in from a shell script, run a `.ego` file,
or drop into the interactive console to test an idea before committing it to a larger
codebase.

**A REST server you can stand up in minutes.** The same `ego` binary can run as an
HTTP/HTTPS server that dispatches requests to Ego programs as service endpoints —
complete with authentication, OAuth2 support, and a built-in web dashboard for
monitoring and administering a running server from your browser. No separate server
framework or deployment tooling required. See [Ego as a Web Server](docs/SERVER.md) and
the [Server Dashboard](docs/DASHBOARD.md).

**A REST-based database, out of the box.** Ego servers can expose PostgreSQL or SQLite3
tables as ACID-compliant REST endpoints, with per-user access control, directly from
configuration — no hand-written CRUD code. See
[Ego Table Services](docs/TABLES.md).

**An embeddable component.** Because Ego is small, self-contained, and has no external
runtime dependencies, it's well suited to being embedded as the scripting or rules layer
inside a larger Go application — giving your users a safe, sandboxed way to customize
behavior without exposing the full power (or risk) of Go itself.

## A taste of the language

```go
type Point struct {
    X, Y int
}

func (p Point) String() string {
    return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

points := []Point{{1, 2}, {3, 4}}
for _, p := range points {
    fmt.Println(p.String())
}
```

Structs, methods, interfaces, closures, goroutines, channels, error handling with
`try`/`catch`, and most of the Go standard library idioms you already know are supported.
Ego also adds a few conveniences on top of Go syntax — like the `print` statement shown
below — for use when you don't need strict Go compatibility.

```sh
$ echo 'print 3+5' | ego
8
```

## Getting started

Running `ego` with no arguments (or `ego run` with no file) drops you into the
interactive console. To run a program from a file:

```sh
ego run myprogram.ego
```

Use `ego help` at any time for a full list of commands.

## Learn more

* [Learning Ego](docs/LEARNING_EGO.md) — a ground-up introduction to the language, assuming no prior Go or Ego experience
* [Language Reference](docs/LANGUAGE.md) — the complete language specification
* [Ego as a Web Server](docs/SERVER.md) — running Ego programs as REST service endpoints
* [Server Dashboard](docs/DASHBOARD.md) — the built-in browser-based admin UI
* [Ego Table Services](docs/TABLES.md) — using Ego as a REST-based database
* [Server API Reference](docs/API.md) — connecting to an Ego server over REST
* [Command Line Interface](docs/CLI.md) — command line grammar and options
* [Building Ego](docs/BUILDING.md) — building the project from source, for developers

## AI usage in Ego development

The Ego programming language, command line interface, and REST server are all authored by
human beings. Any code proposed by AI is reviewed by a human before it is committed. AI
(Tabnine and Claude Code) has been used to generate test cases, and the web dashboard was
written entirely by Claude Code, with the resulting code reviewed to ensure it contains no
data leaks or misuse of Ego API endpoints.
