# STORE-3 — Scalar pointer helpers check `d.(string)` instead of target type in strict/relaxed mode

**Affected functions:** `storeBoolViaPointer`, `storeByteViaPointer`,
`storeInt32ViaPointer`, `storeIntViaPointer`, `storeInt64ViaPointer`,
`storeFloat64ViaPointer`, `storeFloat32ViaPointer`  
**File:** `bytecode/store.go`  
**Risk:** Medium — in strict or relaxed type-enforcement mode, storing a
correctly-typed value through its own native pointer returns `ErrInvalidVarType`
instead of succeeding; storing a string value through a numeric pointer would
then panic at the type assertion  
**Discovered by:** `Test_storeViaPointerByteCode_Float32Pointer_StrictMode`  
**Status: RESOLVED**

## STORE-3: Original behavior

Each scalar pointer helper followed the same copy-paste pattern:

```go
func storeFloat32ViaPointer(c *Context, name string, src any, destinationPointer *float32) error {
    var err error
    d := src
    if c.typeStrictness > defs.RelaxedTypeEnforcement {
        // NoTypeEnforcement (2 > 1): coerce to target type — correct.
        d, err = data.Coerce(src, float32(0))
        if err != nil { return c.runtimeError(err) }
    } else if _, ok := d.(string); !ok {   // ← BUG: should be d.(float32)
        return c.runtimeError(errors.ErrInvalidVarType).Context(name)
    }
    *destinationPointer = d.(float32)  // panics if d is a string
    return nil
}
```

The `else` branch checked `d.(string)` regardless of the helper's target type:

| Scenario | Expected | Actual (buggy) |
| :------- | :------- | :----- |
| Store `float32(3.14)` through `*float32` in strict mode | success | `ErrInvalidVarType` (float32 ≠ string) |
| Store `string("3.14")` through `*float32` in strict mode | `ErrInvalidVarType` | panic on `d.(float32)` |

## STORE-3: Fix

The `d.(string)` assertion was replaced with the correct target type in each
helper:

```go
// storeFloat32ViaPointer:
} else if _, ok := d.(float32); !ok { ... }

// storeFloat64ViaPointer:
} else if _, ok := d.(float64); !ok { ... }

// storeBoolViaPointer:
} else if _, ok := d.(bool); !ok { ... }

// storeByteViaPointer:
} else if _, ok := d.(byte); !ok { ... }

// storeInt32ViaPointer:
} else if _, ok := d.(int32); !ok { ... }

// storeIntViaPointer:
} else if _, ok := d.(int); !ok { ... }

// storeInt64ViaPointer:
} else if _, ok := d.(int64); !ok { ... }
```

The original documentation test was replaced with four targeted tests:

- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode` — float32 accepted in strict mode
- `Test_storeViaPointerByteCode_Float32Pointer_RelaxedMode` — float32 accepted in relaxed mode
- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode_WrongType` — float64 rejected in strict mode
- `Test_storeViaPointerByteCode_BoolPointer_StrictMode` and `_IntPointer_StrictMode` — additional type coverage
