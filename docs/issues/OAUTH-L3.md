# OAUTH-L3 — `EGO_OAUTH_CLIENT_SECRET` environment variable not cleared after reading

**Affected file:** `server/oauth/config.go:130` — `loadConfig()`

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
}
```

**Description:**
The OAuth2 client secret can be supplied via the `EGO_OAUTH_CLIENT_SECRET`
environment variable. Unlike `EGO_PASSWORD`, which was fixed by LOGIN-L1 to call
`os.Unsetenv` and emit a visible warning immediately after reading, the OAuth
client secret is read and stored but the environment variable is never cleared.
The secret therefore remains accessible to any child process spawned after server
startup (for example, via Ego's built-in exec functions if `ExecPermittedSetting`
is enabled) and is visible in `/proc/<pid>/environ` on Linux for the lifetime of
the server process.

**Recommendation:**
Clear the environment variable and emit a visible warning immediately after
reading, matching the pattern from LOGIN-L1:

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
    os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")
    ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})
}
```

**Resolution (June 2026):**
`loadConfig()` in `server/oauth/config.go` was updated to match the LOGIN-L1
pattern:

- The `ui` package import was added to `config.go`.
- After copying `envSecret` into `clientSecret`, the code now calls
  `_ = os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")` to clear the variable from the
  process environment, and then calls
  `ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})` to emit a
  SERVER-level log entry that is always visible in the server log and the
  dashboard Log tab.
- New log message key `oauth.rs.client.secret.env` added to all three language
  files in alphabetical position before `oauth.rs.discovery.ok`.

Two tests added to `server/oauth/config_test.go`:
`TestLoadConfig_EnvVarSecretCleared` sets the env var to a known value, calls
`loadConfig()`, and confirms (a) the returned config carries the value and (b)
`os.Getenv` returns `""` afterward.  `TestLoadConfig_NoEnvVar` verifies that
the absent-env-var path does not panic.

