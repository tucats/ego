# LOGIN-H1 — Timing attack in password comparison

**Affected file:** `server/auth/validate.go:27`

```go
ok = realPass == hashPass
```

**Description:**  
The built-in `==` operator on strings returns as soon as it finds the first
differing byte. An attacker making many requests can measure response-time
variations to determine how many leading bytes of their guess match the stored
hash — a well-known side-channel attack that can significantly narrow the
search space for an offline crack.

**Recommendation:**  
Replace the equality check with `crypto/subtle.ConstantTimeCompare`:

```go
ok = subtle.ConstantTimeCompare([]byte(realPass), []byte(hashPass)) == 1
```

**Resolution:**  
Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`.

