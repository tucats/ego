# MEMBERS-8 — `getNativePackageMemberValue` can never resolve an Ego-only method with no matching Go method name

**Affected function:** `getNativePackageMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** High — silently prevents *any* Ego-only extension method registered on a
native passthrough type (a type whose Ego methods are backed by a raw Go value, not
a `*data.Struct`) from ever being callable, with no compile-time warning; only
Ego-registered methods that happen to share a name with a genuine Go method on the
same concrete type work, purely by coincidence  
**Discovered by:** manual testing while adding `time.Time.SleepUntil()`, an
Ego-specific convenience method with no Go equivalent  
**Status: RESOLVED**

## MEMBERS-8: Description

`getNativePackageMemberValue` resolves a member name on a native Go value (one that
doesn't match the dedicated `*data.Struct`/`*data.Map`/`*data.Package` cases) in two
steps. Step 1 used to be gated behind a Go reflection check:

```go
gt := reflect.TypeOf(mv)
if _, found := gt.MethodByName(name); found {
    // ... derive pkg/typeName from gt.String(), walk
    // context symbols → *data.Package → *data.Type → Function ...
}
```

The intent of the package-registry walk is to find an Ego-registered `Function`
entry for `name` on the type (e.g. one added via `Type.DefineFunction` or
`Type.DefineNativeFunction`). But it only ever ran when Go's own reflection
*also* recognized `name` as a genuine method on the concrete native type
(`gt.MethodByName(name)`). This meant:

- An Ego method sharing a name with a real Go method (e.g.
  `time.Duration.String()`, which Ego re-implements with an extra
  `extendedFormat` argument) resolved correctly — the reflection check passed
  for an unrelated reason (Go really does have a `String()` method), and the
  subsequent registry walk found the Ego override.
- An Ego-only method with **no** corresponding Go method name at all (e.g. a
  new `time.Time.SleepUntil()`, added specifically because Go has no
  equivalent) could never resolve: `gt.MethodByName("SleepUntil")` correctly
  reports no such method on `time.Time`, so the registry walk — which
  otherwise would have found it — was never even attempted, and the call
  failed with `"unknown field or method name for this object type: SleepUntil"`.

**Reproducer (before the fix):**

```go
t := time.Now()
err := t.SleepUntil()
// Error: unknown field or method name for this object type: SleepUntil
```

## MEMBERS-8: Fix

Removed the `gt.MethodByName(name)` gate entirely; the package-registry walk (deriving
`pkg`/`typeName` from `gt.String()`, e.g. `"*time.Duration"` → `pkg="time"`,
`typeName="Duration"`) now always runs, regardless of whether Go's reflection
recognizes `name` as a real method. The walk's own existing checks (package lookup,
type lookup, then `FunctionByName(name)`) already provide the necessary safety net —
if `name` isn't actually registered, it falls through to step 2 and then the final
error, exactly as before. No other caller relies on the removed gate: it only ever
narrowed which names could be tried, it added no additional correctness check.

Regression tests `Test_getNativePackageMemberValue_EgoOnlyMethod_NoMatchingGoMethod`
and `Test_getNativePackageMemberValue_UnknownMethod_StillErrors`
(`bytecode/member_test.go`) register a synthetic `"time"` package with a
`Duration`-shaped `*data.Type` carrying a method (`TripleValue`) that does not exist
on Go's real `time.Duration`, and confirm it now resolves against a genuine
`time.Duration` receiver value (and that a truly unregistered name still errors).
Verified against the original reproducer (`time.Time.SleepUntil()`) and confirmed
the test fails when the gate is reintroduced.
