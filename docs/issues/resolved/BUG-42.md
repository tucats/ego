# BUG-42 — `io.ReadFile`/`io.WriteFile` are documented but do not exist

**Severity:** HIGH

**Description:**  
`docs/LANGUAGE.md`'s `io` package section documents `io.ReadFile(filename)` and
`io.WriteFile(filename, string)`. Neither function exists on the `io` package. The actual
file read/write functions live on the **`os`** package instead, under different names and
with different signatures than documented.

**Reproducer:**

```go
import "io"

func main() {
    io.WriteFile("/tmp/x.txt", "hello")
}
```

**Actual output:**

```text
Error: at main(line 4), unknown package member: WriteFile
```

**Expected output:**

Either the function should exist on `io` as documented, or the documentation should point
to the actual location.

**Notes:**  
`internal/runtime/io/types.go` only defines `Expand`, `Open`, `ReadDir`, `Prompt` (plus
`io.DirList`, implemented separately in `lib/packages/io/dirlist.ego`). The real read/write
functions are in `internal/runtime/os/types.go`:
`os.ReadFile(filename)` takes exactly 1 argument (matches Go, not the `io.ReadFile` doc's
package), and the write function is spelled `os.Writefile` (lowercase `f`, inconsistent
with every other exported PascalCase name in the codebase) and requires **3** arguments —
`Writefile(filename string, mode int, data []byte)` — not the 2-argument
`io.WriteFile(filename, string)` shown in `docs/LANGUAGE.md`'s `io` section
(~lines 2891-2944).

**Resolution:***

- Removed the bogus references to the functions in the io package.
- Added references to Go-compliant forms in LANGUAGES.md
- Fixed mispelled `Writefile` function name (should be `WriteFile`)
- Fixed parameter order to match Go, so mode is last parameter
- Added documentation note in LANGUAGE.md about using `0o` radix prefix for octal
- Updated Ego unit tests to use correct package and argument lists.

