# LOGIN-L1 — Password supplied via environment variable

**Affected file:** `app-cli/app/logon.go:38` — `EgoPasswordEnv`

Environment variables are visible to all processes owned by the same user and
are inherited by child processes. Using `EGO_PASSWORD` to supply credentials is
a common CI convenience but should be noted in any threat model. Consider
supporting a credentials file with `0600` permissions or a secrets-manager
integration as a more secure alternative for automated contexts.

**Resolution (April 2026):**  
`app-cli/app/logon.go` now checks `os.Getenv(defs.EgoPasswordEnv)` after
reading the password option. When the variable is set, `ui.Say("logon.password.env")`
emits a visible warning to the user's console regardless of log level, and
`os.Unsetenv` clears the variable immediately so child processes do not inherit
the credential. Localized strings added to all three language files.

