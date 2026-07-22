# BUG-78 — `profile.Set()` on an `ego.*` setting leaks to the persisted profile file

**Severity:** LOW

**Description:**  
Found while building test coverage for the new `ego.compiler.type.shadowing` setting (BUG-75).
`internal/runtime/profile/profile.go`'s `setKey` (the implementation behind the Ego-visible
`profile.Set()` function) contains an explicit comment and code path stating that settings with
the `ego.*` prefix are updated only in memory, never persisted to disk:

```go
// Ego settings can only be updated in the in-memory copy, not in the persisted data.
if isEgoSetting {
    return err, nil
}

// Otherwise, store the value back to the file system.
return err, settings.Save()
```

In practice this guarantee does not hold: calling `profile.Set("ego.compiler.type.shadowing",
"false")` from Ego code and then exiting the process leaves the on-disk profile file
(`~/.ego/<profile>.profile`) permanently changed to `"false"`.

**Root cause:**  
The leak does not come from `setKey` itself — its own `Save()` call is correctly skipped for
`ego.*` keys, exactly as the comment describes. It comes from a *different*, unconditional
`settings.Save()` call in the top-level CLI dispatcher: `internal/cli/app/run.go`'s
`RunFromArgs`-equivalent entry point calls `settings.Save()` once, after every successful command
(`ego test`, `ego run`, or any other verb), regardless of what that command did:

```go
if err := context.Parse(); err != nil {
    return err
} else {
    // If no errors, then write out an updated profile as needed.
    if err = settings.Save(); err != nil {
        return err
    }
}
```

`settings.Set()` (called unconditionally by `setKey`, before the `isEgoSetting` check decides
whether to *also* call `Save()` itself) marks the active `Configuration.Dirty = true` regardless
of the key's prefix. `settings.Save()` (`internal/cli/settings/files.go`) persists *any*
configuration with `Dirty == true` to disk — it has no awareness of `setKey`'s `isEgoSetting`
distinction at all. So the very next time the top-level command's own `Save()` runs (which happens
unconditionally, for every command, not just `ego config set`), the `ego.*` change gets flushed to
disk anyway, completely undoing the in-memory-only guarantee `setKey` tried to provide.

**Reproducer:**

```sh
ego set config ego.compiler.type.shadowing=true
grep type.shadowing ~/.ego/default.profile   # shows "true"

cat > /tmp/leak_test.ego <<'EGO'
@test "leak"
{
    import "profile"
    profile.Set("ego.compiler.type.shadowing", "false")
    @pass
}
EGO
ego test /tmp/leak_test.ego

grep type.shadowing ~/.ego/default.profile   # now shows "false" -- leaked
```

**Expected output:**  
The on-disk profile should be unaffected by `profile.Set()` calls on `ego.*`-prefixed keys; only
the in-memory value for the remainder of that process should change, matching the existing code
comment's stated intent.

**Notes:**  
This affects every `ego.*` setting, not just the new `ego.compiler.type.shadowing` one — it is a
general gap in the `isEgoSetting` protection, not something introduced by BUG-75's work. Not fixed
when BUG-75 was written, to keep that work scoped; the regression tests added for BUG-75 avoided
exercising `profile.Set()` on `ego.compiler.type.shadowing` for exactly this reason, using
`@compile`'s own `typeShadowing=true|false` flag instead — see the Resolution below for how that
flag interacts with the fix.

**Root cause (confirmed):**  
The leak is not in `setKey`'s own logic — its decision to skip calling `settings.Save()` for
`ego.*` keys is correct as far as it goes. The problem is that `settings.Set()` (called
unconditionally by `setKey`, regardless of key prefix, to perform the actual "in-memory" update)
marks the active `Configuration.Dirty = true` — and `internal/cli/app/run.go`'s top-level CLI
dispatcher calls `settings.Save()` **unconditionally** after every successful command, with no
awareness of *why* something became dirty. `settings.Save()` persists any configuration with
`Dirty == true`, so the very next time any command completes — not just `ego config set` — the
`ego.*` change gets flushed to disk anyway, regardless of `setKey` never calling `Save()` itself.
`settings.Delete()` has the identical problem for the empty-value (delete) case.

**Resolution:**  
`setKey` (`internal/runtime/profile/profile.go`) now routes `ego.*` keys through
`settings.SetDefault()`/a new `settings.DeleteDefault()` instead of `settings.Set()`/`Delete()`.
Both only touch the transient "explicit values" overlay (`internal/cli/settings/values.go`) that
`Get()`/`Exists()` already check *first*, before falling back to the persisted `Configuration` —
so a `profile.Set()` call still takes effect immediately, and is still visible to any code that
runs later in the same process (including a later `.ego` file in the same `ego test` invocation,
since `ego test` compiles and runs one file at a time within a single process). Critically,
neither function ever sets `Configuration.Dirty` or touches `Configuration.Items`, so `run.go`'s
unconditional `Save()` has nothing to flush for that key, no matter when it runs.

`DeleteDefault(key)` (new) is the ephemeral counterpart to the pre-existing `SetDefault(key,
value)`: it removes the key from the explicit-values overlay only. Deleting an override that was
never set is a harmless no-op (unlike `Delete()`, which errors if the key isn't found anywhere) —
appropriate here since `setKey` already requires `settings.Exists(key)` to be true for `ego.*`
keys before reaching this code at all, so "key not found anywhere" can't happen in practice, and
the simpler no-error contract keeps `setKey`'s two branches symmetric.

Non-`ego.*` keys are completely unaffected by this fix: `setKey` still calls `settings.Set()`/
`Delete()` and `settings.Save()` for those, exactly as before, since a user setting stored via
`profile.Set()` is *supposed* to persist.

**Regression tests:**

- Go-level: `TestSetDefault` and `TestDeleteDefault`, added to
  `internal/cli/settings/values_test.go`, directly verify the ephemeral-overlay contract —
  `Get()`/`Exists()` reflect the change immediately, but the persisted `Configuration.Items` and
  `Dirty` flag are never touched, including when deleting a key that was never overridden.
- Ego-level: a new file, `tests/packages/profile_ego_settings.ego`, with 2 `@test` blocks
  verifying `profile.Set()`/`profile.Get()` and `profile.Delete()` round-trip correctly for an
  `ego.*` key from within Ego code itself (the process-visibility half of the contract — a single
  `.ego` test can't easily assert on `~/.ego/<profile>.profile`'s on-disk contents without
  depending on the developer's home directory layout, so that half is covered by the Go-level
  tests above). Both tests restore the setting to its original value before finishing, since the
  ephemeral overlay is process-global, not scoped to one test or file.
- Manual end-to-end verification: `ego set config ego.compiler.type.shadowing=true` (persisted),
  then `ego test` on a file that calls `profile.Set(..., "false")` and reads it back via
  `profile.Get()` (confirms in-process visibility), then checked `~/.ego/default.profile` and a
  **fresh** `ego show config` process afterward (both still show `"true"`, confirming no leak).
  Also re-verified the cross-file scenario from BUG-75's testing notes (file A sets the value via
  `profile.Set()`, file B's `@compile` block observes the change) still works, since that relies
  on the same in-process visibility this fix had to preserve.

Verified against `go build ./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under
`--types dynamic`, `--types strict`, and `--types relaxed` (1383 `@test` blocks, up from 1381,
with no regressions).

