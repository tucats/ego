# BUG-91 — Auto-addressed pointer-receiver method via `defer` mutates a copy, not the original

**Severity:** MEDIUM

**Discovered by:** manual testing while writing regression coverage for the
CALL-11 fix (see the CALL area, `CALL-11`); unrelated to that fix itself —
reproduces identically on code paths CALL-11 never touches.

**Status: OPEN**

**Description:**  
A pointer-receiver method (`func (r *T) Method()`) called via `defer` on a
*non-pointer* variable — the case where Go's (and Ego's) "auto-address" rule
implicitly takes `&r` at the call site — silently mutates a throwaway copy
instead of the original variable. The direct (non-deferred) call works
correctly; only the deferred form is affected:

```ego
type Recorder struct {
    out []string
}

func (r *Recorder) Record(extra string) {
    r.out = append(r.out, "recorded:" + extra)
}

var r Recorder
r.Record("hello")
fmt.Println(r.out)              // ["recorded:hello"] -- correct

var r2 Recorder
defer r2.Record("hello")
// ... after the enclosing function returns and the defer fires ...
fmt.Println(r2.out)             // [] -- WRONG, should also be ["recorded:hello"]
```

Explicitly-pointer-typed receivers are unaffected — `p := &Recorder{...}; defer
p.Record(...)` works correctly, which is why none of the existing `defer`
regression tests (`tests/defer/method_receivers.ego`, all of which use either
an explicit `*T` parameter or a value receiver) caught this: the bug requires
the specific combination of a non-pointer receiver variable *and* a
pointer-declared receiver method.

**Probable cause:**  
`hoistDeferReceiver` (`internal/language/compiler/defer.go`) is the BUG-43
fix's receiver-freezing logic: it compiles the deferred call's receiver chain
immediately (at `defer`-statement time, not when the deferred call eventually
runs), unconditionally emits `bytecode.ValueCopy`, and stores the result into
a generated temp variable — rewriting `defer r.Method(args)` into
`defer $tempName.Method(args)`. This is exactly correct for a *value*
receiver (the whole point of BUG-43: later mutations to the original `r` must
not be visible through the deferred call). But when `Method` has a *pointer*
receiver and `r` requires auto-addressing, copying `r`'s value into
`$tempName` and then auto-addressing `$tempName` at call time takes the
address of the **copy**, not of the original `r` — so the deferred call's
field writes land on a value that is discarded once the temp variable goes
out of scope, and the original variable is never touched.

**Suggested fix:**  
`hoistDeferReceiver` needs to know, at the point it decides whether to freeze
the receiver, whether the target method has a pointer or value receiver (this
information is available from the method's declaration, resolved via the same
type-lookup machinery `compileDotReference` already uses). When the receiver
is a pointer receiver, the correct freeze is to take the receiver's *address*
now (`&r`, stored into `$tempName` as a pointer) rather than copying its
value — mirroring, at defer time, exactly the auto-address Ego already
performs at ordinary (non-deferred) call time. A value receiver keeps the
existing `ValueCopy` behavior unchanged.

