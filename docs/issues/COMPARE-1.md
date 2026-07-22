# COMPARE-1 — `notEqualByteCode` and `greaterThanOrEqualByteCode` had no tests

**Affected operators:** `!=`, `>=`  
**File:** `bytecode/compare_test.go`  
**Risk:** Low — untested paths; the absence of tests allowed the COMPARE-2 and
COMPARE-3 bugs to go undetected  
**Discovered by:** audit of `compare_test.go`  
**Status: RESOLVED**

## COMPARE-1: Description

The original `compare_test.go` contained a single `TestComparisons` table-driven
function that covered `==`, `<`, `>`, and `<=`.  `notEqualByteCode` (`!=`) and
`greaterThanOrEqualByteCode` (`>=`) had zero test cases.

Additionally, the original tests used raw `Context{}` struct literals with direct
`ctx.stack[0]` index reads, bypassing the `newTestContext` / `withStack` helpers
established in all other bytecode test files.

## COMPARE-1: Fix

`compare_test.go` was rewritten with 79 flat test functions following the
established helper pattern.  All six comparison operators are now covered across
the full set of scalar types, composite types, nil values, strict/dynamic mode,
and unsigned integers.
