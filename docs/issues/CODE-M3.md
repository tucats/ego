# CODE-M3 — Client-supplied session UUID not validated or bound to the authenticated user

**Affected file:** `server/admin/run.go:179` — `RunCodeHandler()`

```go
if req.Session == "" {
    req.Session = uuid.New().String()
}
```

**Description:**  
The `Session` field is taken verbatim from the JSON request body and used
directly as the key into both `codeSessions` (persistent symbol tables) and
`debugSessions` (active debugger contexts). No format validation is performed —
the field accepts any string. More importantly, there is no binding between a
session key and the authenticated user who created it.

Two consequences follow:

1. **Session fixation** — a malicious user can specify a session UUID they
   already know (e.g. one observed or guessed from another user's traffic) and
   interact with that user's persistent symbol table or inject commands into
   their active debug session.
2. **Log injection** — the raw UUID is written to the SERVER log via
   `ui.A{"id": uuid}`. A crafted value containing newline characters or
   log-format control sequences can corrupt structured log output.

**Recommendation:**  
Validate that the client-supplied `Session` value conforms to UUID v4 format
before accepting it (reject with 400 otherwise). Additionally, bind each session
entry to the authenticated username at creation time and enforce that the
requesting user matches the session owner on every subsequent call:

```go
if entry.owner != session.User {
    return util.ErrorResponse(w, session.ID, "session not found", http.StatusNotFound)
}
```

Returning 404 rather than 403 avoids confirming the existence of another user's
session.

**Resolution (April 2026):**  
`RunCodeHandler` now calls `uuid.Parse(req.Session)` before using the value
and returns 400 for any non-UUID string. `codeSessionEntry` and `debugSession`
both gained an `owner string` field set to `session.User` at creation time.
`getOrCreateSymbolTable` and `executeAdminDebug` each check `entry.owner != user`
on session lookup and return an opaque error (`run.not.found` /
`ErrNoPrivilegeForOperation`) that does not reveal whether the session belongs
to another user. The localized `run.not.found` key has been added to all three
language files.

