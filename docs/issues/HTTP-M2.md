# HTTP-M2 — Server UUID and session counter disclosed on every response

**Affected file:** `server/server/serve.go:65`

```go
w.Header()[defs.EgoServerInstanceHeader] = []string{
    fmt.Sprintf("%s:%d", defs.InstanceID, sessionID),
}
```

**Description:**  
Every non-lightweight response carries the `X-Ego-Server` header whose value
is `<UUID>:<sessionID>`. This discloses two pieces of information:

1. **Persistent server UUID** — `defs.InstanceID` is stable for the life of the
   server process. An attacker learns a fingerprint that lets them confirm they
   are talking to the same server instance across requests, enumerate multiple
   instances in a cluster, and correlate responses in log analysis.

2. **Monotonically increasing session counter** — `sessionID` is a global
   sequence number that increments for every request. An attacker can measure
   the exact rate of legitimate traffic, detect low-activity windows optimal
   for an attack, and infer whether a request they injected was processed by
   comparing counter values before and after.

Neither piece of information is needed by well-behaved clients; the dashboard
reads it only for display purposes. The same UUID is already in the
`server_info` JSON body for clients that genuinely need it.

**Recommendation:**  
Remove the header from production responses, or make it opt-in for clients
that need it (e.g. behind an `X-Ego-Debug` request header only recognized
when debug logging is active). At minimum, omit the session counter from the
header value so the UUID alone is disclosed — it is already available in the
response body for clients that need it.

**Resolution:**  
`X-Ego-Server` header removed entirely.

