# LOGIN-M3 — Token cache bypasses expiry and revocation checks

**Affected file:** `server/server/auth.go:117`

```go
if userItem, found := caches.Find(caches.TokenCache, token); found {
    isAuthenticated = true
    user = data.String(userItem)
    // expiry and blacklist not rechecked
}
```

**Description:**  
When a bearer token is found in the 60-second in-memory cache, it is accepted
as valid without rechecking whether it has expired or been added to the
revocation blacklist since it was first cached. A token that expires or is
revoked can continue to authenticate requests for up to 60 seconds.

**Recommendation:**  
On a cache hit, still verify `tokens.Validate()` for expiry and blacklist
status. The cache value should store the parsed `Token` struct (including
its expiry field) rather than just the username string, so these checks can
be performed without a full re-decryption on every request.

**Resolution:**  
Cache now stores `*tokens.Token`; cache hits check `Expires` directly (no
re-decryption). Blacklist is already handled: `tokens.Blacklist()` purges the
token cache at revocation time.

