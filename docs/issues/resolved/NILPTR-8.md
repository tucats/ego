# NILPTR-8 — Typed-nil pointers surviving a successful type assertion

**Affected functions:** `(*Session).Authenticate` (token cache path),
`validateClientSecret`
**Files:** `router/auth.go`, `server/oauth/authserver/clients.go`
**Risk:** Medium (auth.go — panics in the authentication path of every request
presenting the affected token); Low (clients.go — not reachable today, but the
failure mode would be an auth bypass)
**Status: RESOLVED**

## NILPTR-8: Description

Both findings turn on the same Go detail: **the `ok` result of a type assertion
tells you the type matched, not that the pointer inside is non-nil.**

A `(*T)(nil)` stored in an `any` — what Go calls a *typed nil* — is not the same
as a nil `any`. It satisfies `v.(*T)` and yields `ok == true`, handing back a nil
pointer that the caller then treats as valid.

### auth.go — token cache

```go
if v, found := caches.Find(caches.TokenCache, token); found {
    tok, isFull := v.(*tokens.Token)

    if isFull && !tok.Expires.IsZero() && time.Now().After(tok.Expires) {
```

The rest of the block reads `isFull` as meaning "we have a usable full token" and
dereferences `tok` on that basis, in several places. A cache entry holding a typed
nil makes `isFull` true with `tok == nil`, and `tok.Expires` panics — in the
authentication path, for every request presenting that token, until the cache
entry ages out.

### clients.go — client secret validation

```go
func validateClientSecret(client *OAuthClient, secret string) bool {
    if client.ClientSecretHash == "" {
        // A client with no secret hash is a public client; any (or no) secret
        // is accepted.
        return true
    }
    ...
```

`findClient` returns nil for an unknown client ID, and this function dereferences
its parameter with no check.

Every current caller is correct — all six write
`client == nil || !validateClientSecret(client, clientSecret)`, and Go's `||`
short-circuits, so a nil client never arrives. This is documented and fixed
regardless because of what happens if a future caller forgets that guard: the nil
dereference lands on the `client.ClientSecretHash == ""` test, and the "obvious"
fix for that panic — treating a nil client as the empty-hash case — returns
**true** and authenticates an unregistered client. The safe answer needs to be the
automatic one.

## NILPTR-8: Fix

`auth.go` folds the nil test into the flag, so `isFull` means what the code below
assumes:

```go
tok, isFull := v.(*tokens.Token)

isFull = isFull && tok != nil
```

`clients.go` rejects a nil client outright:

```go
func validateClientSecret(client *OAuthClient, secret string) bool {
    if client == nil {
        return false
    }
    ...
```

Returning false rather than panicking means a future caller that omits its nil
guard gets a failed authentication — the conservative outcome — instead of either
a crash or a bypass.
