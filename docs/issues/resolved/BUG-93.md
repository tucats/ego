# BUG-93 — A package organized as a directory of files cannot be found anywhere except `lib/packages`, unlike the identical package written as a single file

**Severity:** MEDIUM

**Discovered by:** manual testing while working on PERFORMANCE.md Finding 17 (`docs/internals/GLOBALS.md`), while checking that the new global-reference cache correctly handles the already-working imported-package proxy path; unrelated to that work itself.

**Status:** FIXED

**Description:**  
A package written as a single file (`import "foo"` finding `foo.ego`) can be found relative to
the process's current working directory when it isn't found under the `lib/packages` root either
— this is exactly how the project's own test suite's cross-package tests already work (see
`tests/packages/localfuncs.ego`'s `import "tests/packages/test1"`, resolved relative to the repo
root, which is the working directory `ego test tests/` is conventionally run from).

A package written as a **directory of one or more files** (`import "foo"` finding `foo/*.ego`) had
no such fallback at all — it could only ever be found under `lib/packages/foo/`. The exact same
package, organized as a directory instead of a single file, silently failed to import from
anywhere else, even when run from the same working directory that makes the single-file case work:

```ego
// main.ego
package main
import "fmt"
import "mypkg"

func main() {
    fmt.Println(mypkg.Value)
}
```

```text
mypkg.ego                      <- works: import "mypkg" finds this file
mypkg/mypkg.ego                <- fails: import "mypkg" cannot find this directory
```

```text
$ ego run main.ego
Error: at line 4:1, open /some/unrelated/path/mypkg.ego: no such file or directory
```

The error is also misleading: it names a path under whatever `EGO_PATH`/`lib/packages` happens to
resolve to on the machine running it (the *last* location tried), not the directory the user
might reasonably expect Ego to have looked in.

**How Go resolves import paths (for comparison):**  
Go's import resolution is intentionally never relative to the *file* doing the importing, and
never depends on the process's working directory at the point a *reference* is resolved. Every
import path is one of:

1. A **standard-library path** (`"fmt"`, `"os/exec"`), resolved against `GOROOT/src`.
2. A **module path** (`"github.com/user/repo/subpkg"`), resolved via the nearest enclosing
   `go.mod`: if the import path is a prefix-match of the current module's own declared module
   path, it maps directly to a subdirectory of that module's root (the directory containing
   `go.mod`); otherwise it's an external dependency, resolved via the module cache/`go.sum`.
3. Go has **no relative import syntax at all** — `import "./foo"` is rejected outright at compile
   time ("local import ... in non-local package"). A package is always "found" by mapping an
   import path onto a directory via one of the two mechanisms above, never by walking relative to
   whichever file wrote the `import` statement.
4. Critically, Go draws **no distinction between a single-file and a multi-file package** — a
   package *is* a directory (containing one or more `.go` files with the same `package` clause);
   there is no separate "single file" resolution rule at all.

**The reasonable Ego expectation:** Ego has no `go.mod`/module-path system, so there is no direct
analog to Go's module-root resolution — the closest fixed root Ego has is `lib/packages`
(playing roughly the role of `GOROOT/src` for "built-in" library packages), and the process's
current working directory plays the role Go's module root would for anything else (this is
already how the project's own test suite's cross-package imports work today, and predates this
fix). Given that, item 4 above is the one Go principle this bug actually violated: **Ego, like
Go, must not distinguish between a single-file package and a multi-file directory package** —
whatever resolution succeeds for one must succeed for the other. This fix makes that true; it
does not add relative-to-importing-file resolution (which Go itself does not support either, per
item 3 above).

**Root cause:** `directoryContents` (`internal/language/compiler/import.go`) — the function
responsible for reading every `.ego` file out of a package directory — only ever tried the name
joined onto the `lib/packages` root:

```go
dirname := name
if !strings.HasPrefix(dirname, path) {
    dirname = filepath.Join(path, name)
}
fi, err := os.ReadDir(dirname)
if err != nil {
    return "", errors.New(err)
}
```

Its caller, `readPackageFile`, tries `directoryContents` *first*, and only falls back to reading
`name`/`name.ego` as a **file** relative to the working directory (or as given, if absolute) when
that fails — a fallback `directoryContents` itself never had an equivalent of. Every package
referenced by path in the existing test suite (`tests/packages/employee`, `tests/packages/test1`,
etc.) happens to be a single `.ego` file, so this asymmetry was never exercised or noticed before.

**Fix:** `directoryContents` now falls back to `os.ReadDir(name)` (the given name, resolved
relative to the working directory, or as-is if absolute) when the `lib/packages`-rooted directory
isn't found — mirroring, for the directory case, exactly the fallback `readPackageFile` already
performs for the single-file case just below it:

```go
fi, err := os.ReadDir(dirname)
if err != nil {
    var fallbackErr error

    fi, fallbackErr = os.ReadDir(name)
    if fallbackErr != nil {
        return "", errors.New(err)
    }

    dirname = name
}
```

This is a minimal, targeted fix: it adds the missing fallback in the same style and at the same
precedence position the file-based fallback already established, without changing any other part
of the resolution order (`lib/packages` is still tried first for both files and directories;
`EGO_PATH` and the working-directory fallback for single files are all unchanged).

**Files modified:**

- `internal/language/compiler/import.go` — `directoryContents` gains the working-directory
  fallback described above.

**Tests added:**

- `internal/language/compiler/import_test.go` (new file) — `TestDirectoryContents_CWDFallback`
  (a package directory outside `lib/packages`, found via the working directory) and
  `TestDirectoryContents_NotFoundAnywhere` (confirms a genuinely missing package still reports an
  error rather than silently succeeding).
- `tests/packages/dirpkg/` (new, two-file package: `values.ego`, `funcs.ego`) and
  `tests/packages/dirpkg_test.ego` (new) — an end-to-end regression exercising a real multi-file
  directory package imported by path (`import "tests/packages/dirpkg"`), the same way the
  existing single-file package tests already do.

**Verification:** `go build ./...`, `go vet ./...` clean. `go test ./...` clean. The full
`ego test tests/` suite (1,709 cases, up from 1,708) passes, including the new directory-package
test. Manually verified: a single-level directory package, a nested (multi-segment path) directory
package, and a multi-file directory package all resolve correctly relative to the working
directory when run from the directory containing the importing script; an unrelated working
directory still correctly fails to find the package (consistent with — not a regression of — the
existing working-directory-anchored resolution model); all previously-working cases (`lib/packages`
single-segment, multi-segment, and dot-prefixed paths; single-file working-directory-relative
packages) continue to work unchanged.
