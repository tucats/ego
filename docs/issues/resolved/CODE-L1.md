# CODE-L1 — Full user-submitted code body written to REST log

**Affected file:** `server/admin/run.go:167` — `RunCodeHandler()`

```go
if ui.IsActive(ui.RestLogger) {
    b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
    ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
        "session": session.ID,
        "body":    string(b),
    })
}
```

**Description:**  
When the REST logger is active, the entire deserialized request — including the
`Code` field — is written to the server log. If a user submits code that
contains sensitive values (database credentials, API keys, personal data
embedded in test scripts), those values are persisted in the server log files
for the duration of the log retention period. This is particularly notable
because log files are typically accessible to a broader audience than the
dashboard session itself.

**Recommendation:**  
Redact or truncate the `Code` field before logging. A reasonable approach is to
log the first 120 characters and append an ellipsis when the field is longer:

```go
logReq := req
if len(logReq.Code) > 120 {
    logReq.Code = logReq.Code[:120] + "…"
}
b, _ := json.MarshalIndent(logReq, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
```

This preserves enough context to identify the request in the log without
capturing the full script content.

**Resolution (April 2026):**  
`RunCodeHandler` now copies the request into a local `logReq` variable and
truncates `logReq.Code` to 120 characters (appending `"..."`) before passing
it to `json.MarshalIndent`. The original `req.Code` is unmodified and used
for execution as before.

