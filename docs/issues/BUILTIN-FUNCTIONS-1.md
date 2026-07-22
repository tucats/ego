# BUILTIN-FUNCTIONS-1 — `AddFunction` and related helpers were not safe for concurrent use

**Affected functions:** `AddFunction`, `AddBuiltins`, `FindFunction`,
`FindName`, `CallBuiltin`
**File:** `builtins/functions.go`
**Risk:** Low
**Status: RESOLVED**

## Original FUNCTIONS-1 behavior

`FunctionDictionary` is a package-level `map`.  All five functions that read
or write it did so without holding any lock, producing data races detectable
by Go's `-race` flag when called concurrently.

## Fix for FUNCTIONS-1

A package-level `sync.RWMutex` named `functionDictionaryMu` was added.
All read operations (`FindFunction`, `FindName`, `CallBuiltin`, and the key
snapshot in `AddBuiltins`) hold `RLock`/`RUnlock`.  The write in `AddFunction`
holds `Lock`/`Unlock`, and the existence check + insert are performed
atomically under the same lock:

```go
functionDictionaryMu.Lock()
_, alreadyExists := FunctionDictionary[fd.Name]
if !alreadyExists {
    FunctionDictionary[fd.Name] = fd
}
functionDictionaryMu.Unlock()
```
