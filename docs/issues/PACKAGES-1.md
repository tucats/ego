# PACKAGES-1 — `inPackageByteCode` panics when the symbol table holds a non-package value under the package name

**Affected function:** `inPackageByteCode`  
**File:** `bytecode/package.go`  
**Risk:** High — any local variable whose name matches a package name (e.g. a
loop counter named `math`) causes an unrecoverable nil-pointer panic rather
than a clean error or graceful fallthrough to the package cache  
**Discovered by:** `Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1`  
**Status: RESOLVED**

## PACKAGES-1: Original behavior

`inPackageByteCode` searched the scope chain with `GetAnyScope` and, on any
hit, immediately passed the result to `GetPackageSymbolTable` and called
`NewChildProxy` on the return value:

```go
if pkg, found := c.symbols.GetAnyScope(c.pkg); found {
    c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)
    // ↑ GetPackageSymbolTable returns nil when pkg is not *data.Package,
    //   so NewChildProxy is called on a nil *SymbolTable → panic
    return nil
}
```

`GetPackageSymbolTable` type-asserts its argument to `*data.Package` and
returns `nil` when the assertion fails.  Calling `.NewChildProxy(...)` on a
nil `*SymbolTable` receiver panics with a nil pointer dereference.

In practice this can be triggered by Ego code that uses a local variable with
the same name as a package (e.g. `math := 3.14`) and then enters a `package
math` scope inside the same function.

## PACKAGES-1: Fix

A type assertion was added between the `GetAnyScope` call and the proxy
creation.  When the found value is not a `*data.Package`, the code falls
through to the global package-cache lookup instead of panicking:

```go
if found, ok := c.symbols.GetAnyScope(c.pkg); ok {
    // Guard: must actually be a *data.Package.  A local variable that
    // shadows the package name must not cause a panic (PACKAGES-1 fix).
    if pkg, isPkg := found.(*data.Package); isPkg {
        c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)
        return nil
    }
    // Fall through: the shadowing value is not a package; try the cache.
}
```

Two tests cover this fix:

- `Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1` — confirms that a
  non-package shadow returns `ErrInvalidPackageName` (no panic) when the cache
  also has no entry.
- `Test_inPackageByteCode_NonPackageInSymbolTable_CacheFallback_PACKAGES1` —
  confirms that when the cache does have the real package the call succeeds
  despite the shadowing variable.
