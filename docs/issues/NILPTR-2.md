# NILPTR-2 — Nil route-handler check was gated behind a logging flag

**Affected function:** `(*Router).ServeHTTP`
**File:** `router/serve.go`
**Risk:** High — with REST logging off (the normal production setting) a route
with no handler panicked on a nil function call
**Status: RESOLVED**

## NILPTR-2: Description

`ServeHTTP` checked for a missing route handler like this:

```go
// Log which route we're using. This is helpful for debugging service route
// declaration errors.
if ui.IsActive(ui.RestLogger) {
    // No route handler found, log it and report the error to the caller.
    if route.handler == nil {
        msg := fmt.Sprintf("invalid route selected: %#v", route)
        ...
        util.ErrorResponse(w, sessionID, msg, http.StatusInternalServerError)

        return
    }

    functionName := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()
    ...
}
```

The nil check is nested inside `if ui.IsActive(ui.RestLogger)`. It appears to
have been written to protect the `reflect.ValueOf(route.handler).Pointer()` call
on the following line, which does need a non-nil handler — but as a result the
check only runs when REST logging happens to be switched on.

With logging off, the nil handler flowed through to:

```go
if status == http.StatusOK {
    status = session.handler(session, w, r)
}
```

In Go a `func`-typed struct field defaults to nil, so an incompletely
constructed `Route` has exactly this shape, and calling through a nil function
value panics.

This is the more general lesson of the finding: **a safety check must never be
conditional on a diagnostic setting being enabled.** Gating one behind a log
level means the code is only correct when someone is watching, and the failure
appears exclusively in production, where logging is off.

A second gap: the block containing this check is itself inside
`if route != nil && !route.lightweight`, so even after hoisting the check out of
the logging conditional, a *lightweight* route with a nil handler would still
reach the call site.

## NILPTR-2: Fix

The nil check is hoisted out of the logging conditional so it always runs, and
reports a generic internal error rather than dumping the route struct to the
client:

```go
if route.handler == nil {
    ui.Log(ui.InternalLogger, "route.handler.nil", ui.A{
        "route": fmt.Sprintf("%#v", route)})

    util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.server.error"), http.StatusInternalServerError)

    return
}
```

The call site is guarded as well, which covers the lightweight-route path that
the block above does not reach:

```go
if status == http.StatusOK {
    if session.handler == nil {
        ...
        status = util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.server.error"), http.StatusInternalServerError)
    } else {
        status = session.handler(session, w, r)
    }
}
```

The NILPTR-1 recovery handler would now turn this panic into a 500 anyway, but
reporting the specific problem is far more useful than a generic recovered panic,
and the two fixes are independent — either alone leaves a gap.

## NILPTR-2: Tests

`TestNilHandlerDoesNotPanic_NILPTR2` registers a route with a nil handler and
issues a request with REST logging **off**, which is the configuration that used
to be unprotected. It runs with `ego.server.panic.recovery` disabled, so the test
proves the nil check itself works rather than being masked by the NILPTR-1 safety
net, and asserts an HTTP 500 is returned.
