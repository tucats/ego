# STRUCT-2 — `storeIndexByteCode` mutates a struct field before checking package visibility

**Affected function:** `storeIndexByteCode`  
**File:** `bytecode/structs.go`  
**Risk:** Medium — an unexported field write from outside the owning package
modifies the struct in memory and then returns an error, leaving the struct
in a partially-modified state with no rollback  
**Discovered by:** `Test_storeIndexByteCode_StructPackageVisibility_STRUCT2`  
**Status: RESOLVED**

## STRUCT-2: Original behavior

The `*data.Struct` case in `storeIndexByteCode` called `a.Set(key, v)` to
write the field value and **then** checked whether the field was visible from
the current package:

```go
case *data.Struct:
    key := data.String(index)

    if err = a.Set(key, v); err != nil {   // ← write happened here
        return c.runtimeError(err)
    }

    // Visibility check ran AFTER the write — buggy ordering.
    if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
        if !egostrings.HasCapitalizedName(key) {
            return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
        }
    }
```

When an unexported field from a different package was written, the error was
correctly returned — but the `a.Set` call had already committed the change.
Because `*data.Struct` is a pointer type, the modification was visible to all
callers that held a reference to the same struct.  The same ordering problem
affected the `*any` wrapping a `*data.Struct` path immediately below it.

## STRUCT-2: Fix

The package visibility check was moved to run **before** `a.Set` in both the
`*data.Struct` case and the `*any → *data.Struct` case:

```go
case *data.Struct:
    key := data.String(index)

    // Check package visibility before modifying the struct.
    if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
        if !egostrings.HasCapitalizedName(key) {
            return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
        }
    }

    if err = a.Set(key, v); err != nil {
        return c.runtimeError(err)
    }

    _ = c.push(a)
```

`Test_storeIndexByteCode_StructPackageVisibility_STRUCT2` now asserts that the
field retains its original value (`0`) after the rejected write, confirming
that the struct is not modified before the error is returned.
