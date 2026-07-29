# NILPTR-3 — `database.Open` tests session for nil, then dereferences it anyway

**Affected function:** `Open`
**File:** `server/tables/database/open.go`
**Risk:** Medium — a nil session panics on the line after the nil check
**Status: RESOLVED**

## NILPTR-3: Description

```go
if session != nil {
    user = session.User
}

dsnName, err := dsns.DSNService.ReadDSN(session.ID, user, name, false)
if err != nil {
    ui.Log(ui.DBLogger, "db.dsn.error", ui.A{
        "session": session.ID,
        ...

if !session.Admin {
    if !dsns.DSNService.AuthDSN(session.ID, user, name, action) {
```

The function guards `session != nil` before reading `session.User`, and then
dereferences `session.ID` on the very next statement with no guard at all —
followed by `session.Admin` and six more `session.ID` reads.

Either the nil test is unnecessary or the eight dereferences after it are a crash
waiting to happen. The two cannot both be correct, and that internal disagreement
is the whole finding: a reader cannot tell from the code whether a nil session is
a supported input.

`Open` is reachable from every `/tables` endpoint, so guessing wrong here is
expensive.

## NILPTR-3: Fix

Rather than delete the nil test — which would silently commit the function to
"session is never nil" — the values actually needed are extracted once, under a
single nil check, and the rest of the function uses locals that are always safe
to read:

```go
sessionID := 0
isAdmin := false

if session != nil {
    user = session.User
    sessionID = session.ID
    isAdmin = session.Admin
}
```

The defaults are chosen so that a missing session can only ever *deny* access:

- `sessionID` is used purely for log correlation, so zero is a harmless stand-in.
- `isAdmin` must default to **false**. Defaulting it to true, or skipping the
  admin branch entirely, would let a nil session bypass the `AuthDSN` check
  below — turning a crash into an authorization bypass, which is strictly worse.

## NILPTR-3: Related

See NILPTR-4 for `Database.Close` and `Database.CloseTX` in the same file, which
had the mirror-image problem on the object `Open` returns.
