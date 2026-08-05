# Building Ego

This guide is for developers who have cloned the repository and want to build the
`ego` binary themselves — for the machine they're on, for another platform, or as
part of making changes to the project.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [The first build](#first-build)
3. [Building with the build script](#build-script)
4. [Building for other platforms](#cross-build)
5. [Building on Windows](#windows)
6. [Running the tests](#tests)
7. [Where to look to configure things](#configure)
8. [Troubleshooting](#troubleshooting)

&nbsp;

## Prerequisites <a name="prerequisites"></a>

* Go, matching the version in [go.mod](../go.mod) (currently 1.26.1) or newer.
* A `bash`-compatible shell for `tools/build` (macOS and Linux; on Windows use
  `tools/build.ps1` instead, or WSL/Git Bash).

No other tools are required — Ego has no C dependencies and doesn't need `cgo`.

&nbsp;

## The first build <a name="first-build"></a>

Ego ships with a `lib` directory containing its runtime library, sample services, and
the web dashboard's static assets. Rather than requiring that directory to be installed
next to the executable, the whole tree is compressed into a zip archive at build time and
embedded directly into the binary via `go:embed`. That archive (`internal/cli/app/lib.zip`)
is a build product, not something checked into source control, so it has to be generated
before the first compile:

```sh
go generate ./...
go build
```

This is exactly what `go build` alone will *not* do for you — see
[Troubleshooting](#troubleshooting) below if you skip this step. The resulting `ego`
binary in the repository root will report itself as a "developer build" with no version
number, which is fine for local iteration.

In practice, you'll normally use `tools/build` instead of calling `go generate` and
`go build` by hand, since it does this for you and also stamps in a real version number.

&nbsp;

## Building with the build script <a name="build-script"></a>

`tools/build` is the normal way to build Ego on macOS or Linux. Run it from the
repository root:

```sh
tools/build
```

By default this builds for the platform and architecture you're running on (using
`go env GOARCH`/`GOOS`), runs `go generate ./...` first, and injects the current build
version (from `tools/buildver.txt`) into the binary via linker flags. The resulting
`ego` binary is written to the repository root.

Useful options:

| Option | Effect |
| :----- | :----- |
| `-i`, `--increment` | Increment the build number in `tools/buildver.txt` before building. By convention, do this *after* finishing a related set of changes, not before. |
| `--bin` | Copy the built binary to `~/bin` after a successful build. |
| `-r` | Enable the Go race detector (`-race`). Slower, but useful when chasing concurrency bugs. |
| `-v`, `--verbose` | Print the full `go build` command before running it. |
| `-b`, `--build <flags>` | Pass additional flags straight through to `go build`. |
| `-h`, `--help` | List all options. |

For example, to build with an incremented build number and the race detector enabled:

```sh
tools/build -i -r
```

&nbsp;

## Building for other platforms <a name="cross-build"></a>

`tools/build` can cross-compile using Go's standard `GOOS`/`GOARCH` cross-compilation
support — no special toolchain is needed. You can target one specific platform, or build
all of them at once.

Single target:

| Option | Target |
| :----- | :----- |
| `-a`, `--apple`, `--arm` | macOS, Apple Silicon (`darwin`/`arm64`) |
| `-l`, `--linux` | Linux, x86-64 (`linux`/`amd64`) |
| `-w`, `--windows` | Windows, x86-64 (`windows`/`amd64`) |

All platforms at once:

```sh
tools/build --all
```

This builds every supported combination — Linux (x86-64 and arm64), macOS (x86-64 and
Apple Silicon), and Windows (x64 and arm64) — and places each binary under `builds/`,
for example `builds/macos/applesilicon/ego` or `builds/windows/x64/ego.exe`. This is
the same layout used to produce release artifacts.

&nbsp;

## Building on Windows <a name="windows"></a>

Use the PowerShell script instead:

```powershell
.\tools\build.ps1
```

This runs `go generate ./...`, then `go build`, injecting the version string from
`tools/buildver.txt`. It's intentionally simpler than `tools/build` — it doesn't support
the increment/race/cross-build options above. If you need those on Windows, use WSL or
Git Bash to run `tools/build` instead.

&nbsp;

## Running the tests <a name="tests"></a>

* `tools/gotests.sh` runs the Go unit test suite (`go test ./...`) and prints a concise
  pass/fail summary. Pass `-a`/`--all` to bypass the test cache and force every test to
  re-run.
* `tools/test.sh` runs the Go unit tests followed by the Ego-language test suite (under
  [tests/](../tests)) at several compiler strictness levels.

See [docs/internals/TESTING.md](internals/TESTING.md) for more detail on the test
layout and conventions.

&nbsp;

## Where to look to configure things <a name="configure"></a>

A few files are worth knowing about before you start making changes:

* **[lib/defaults.json](../lib/defaults.json)** — the default settings baked into every
  newly created configuration profile. If you want Ego, out of the box, to start with
  compiler options, console behavior, or runtime settings different from the built-in
  defaults, this is the file to edit. It ships as part of the embedded `lib` archive, so
  changes here take effect the next time `go generate` packages it (i.e. your next
  build). See [docs/CONFIG.md](CONFIG.md) for the full configuration/profile model this
  feeds into.

* **[internal/defs/config.go](../internal/defs/config.go)** — the master list of every
  configuration key Ego recognizes (`ego.compiler.*`, `ego.server.*`, `ego.runtime.*`,
  and so on), each documented with its default and meaning, plus the `ValidSettings` map
  that determines what `ego config set` will accept. If you're adding a new setting, or
  trying to find out what one does, start here.

* **[internal/cli/app/library.go](../internal/cli/app/library.go)** — documents (and
  contains) the `go:generate` directive that packages the `lib` directory into
  `lib.zip` and embeds it into the binary. If you're changing anything under `lib/`
  (including `defaults.json`), read the comment at the top of this file to understand
  how those changes actually get into a built binary.

* **[internal/i18n/strings.go](../internal/i18n/strings.go)** — has the `go:generate`
  directive that turns the `messages*.txt` files into the generated `messages.go` used
  for localized strings. Also regenerated by `go generate ./...`.

* **[golangci-lint.yml](../golangci-lint.yml)** — the linter configuration used for this
  project; run `golangci-lint run` before sending changes for review.

* **[tools/](../tools)** — home to the build scripts covered above, plus test runners,
  the `zipgo` and `lang` code generators, and assorted helper scripts (API testing,
  dashboard checks, container entrypoints, etc.). Worth a browse if you're looking for
  an existing tool before writing a new one.

&nbsp;

## Troubleshooting <a name="troubleshooting"></a>

**`go build` fails with `pattern lib.zip: no matching files found`**

This means you ran `go build` directly without running `go generate ./...` first.
`internal/cli/app/lib.zip` is a generated build product (excluded from source control by
`.gitignore`), and the `//go:embed` directive in `internal/cli/app/library.go` needs it
to exist before the package will compile. Run:

```sh
go generate ./...
```

and then build again. `tools/build` and `tools/build.ps1` both do this for you
automatically, so this only comes up if you invoke `go build` yourself.
