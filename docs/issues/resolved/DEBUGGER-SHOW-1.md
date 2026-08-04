# DEBUGGER-SHOW-1 — showSource silently swallows invalid-range parse errors

**File:** `debugger/show.go`, `debugger/source.go`  
**Functions:** `showSource`, `showCommand`  
**Risk:** Medium — `show source bad-arg` silently does nothing; the user
receives no feedback that the argument was invalid  
**Status: RESOLVED**

## DEBUGGER-SHOW-1: Original behavior

`showSource` was declared as returning nothing:

```go
func showSource(tx *tokenizer.Tokenizer, tokens *tokenizer.Tokenizer, err error, sessionContext *session)
```

It received the caller's `err` **by value** — a copy.  When an invalid integer
argument was found in the range spec:

```go
if e2 != nil {
    err = errors.New(errors.ErrInvalidInteger)
}
```

…the assignment modified only the local copy.  The caller in `showCommand`
used:

```go
case "source":
    showSource(tx, tokens, err, sessionContext)
```

After the call, the caller's `err` was still `nil`.  The command handler
returned `nil` to the user, giving no indication that the argument was
unparseable.

Additionally, `showSource` received the incoming `err` parameter as an input
gate (`if err == nil { /* list source */ }`), but the caller always passed
`nil`, making that parameter pointless as an input.

## DEBUGGER-SHOW-1: Fix

Changed `showSource` to return an `error`:

```go
func showSource(...) error {
    ...
    if e2 != nil {
        return errors.New(errors.ErrInvalidInteger)
    }
    ...
    return nil
}
```

Removed the now-redundant `err error` input parameter (the caller always
passed `nil`).  Updated the caller in `showCommand`:

```go
case "source":
    err = showSource(tx, tokens, sessionContext)
```
