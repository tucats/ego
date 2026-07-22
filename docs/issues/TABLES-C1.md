# TABLES-C1 — `CommitHandler` inverted guard panics on every valid commit

**Affected file:** `server/tables/transactions.go:207` — `CommitHandler()`

```go
parameters := session.Parameters[defs.TransactionIDParameterName]
if len(parameters) != 0 {          // ← should be != 1
    return util.ErrorResponse(...)  // rejects requests WITH a transaction ID
}
id := data.String(parameters[0])   // panics: index 0 on empty slice
```

**Description:**  
The guard condition is inverted relative to `RollbackHandler` (line 170, which
correctly uses `!= 1`). The result is that any request that arrives *with* a
valid transaction ID parameter — i.e., every legitimate commit — is immediately
rejected with "missing transaction ID". Any request that arrives *without* a
transaction ID parameter — i.e., every malformed commit — passes the guard and
then panics on line 212 (`parameters[0]` on an empty slice), crashing the
goroutine handling that connection.

**Recommendation:**  
Change `!= 0` to `!= 1` on line 207, matching `RollbackHandler`.

**Resolution:**  
Change `CommitHandler` guard from `len(parameters) != 0` to `!= 1`; matches
`RollbackHandler` and prevents panic on `parameters[0]` with empty slice.

