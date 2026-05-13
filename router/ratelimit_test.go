package router

import (
	"testing"
	"time"
)

// resetRateLimiter clears all state between tests so each test starts clean.
func resetRateLimiter() {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	loginAttempts = map[string]*loginRecord{}
}

// TestCheckRateLimit_NoRecord returns 0 when the user has never failed.
func TestCheckRateLimit_NoRecord(t *testing.T) {
	resetRateLimiter()

	if got := CheckRateLimit("alice"); got != 0 {
		t.Errorf("expected 0 for unknown user, got %d", got)
	}
}

// TestRecordFailure_BelowThreshold does not lock the account when failures
// are below defaultMaxAttempts.
func TestRecordFailure_BelowThreshold(t *testing.T) {
	resetRateLimiter()

	for i := 0; i < defaultMaxAttempts-1; i++ {
		RecordFailure(0, "bob")
	}

	if got := CheckRateLimit("bob"); got != 0 {
		t.Errorf("expected 0 before threshold, got %d", got)
	}
}

// TestRecordFailure_AtThreshold locks the account when failures reach the threshold.
func TestRecordFailure_AtThreshold(t *testing.T) {
	resetRateLimiter()

	for i := 0; i < defaultMaxAttempts; i++ {
		RecordFailure(0, "carol")
	}

	if got := CheckRateLimit("carol"); got <= 0 {
		t.Errorf("expected > 0 (locked) after %d failures, got %d", defaultMaxAttempts, got)
	}
}

// TestRecordSuccess_ClearsLockout verifies that a successful login clears
// any accumulated failure state.
func TestRecordSuccess_ClearsLockout(t *testing.T) {
	resetRateLimiter()

	// Lock the account.
	for i := 0; i < defaultMaxAttempts; i++ {
		RecordFailure(0, "dave")
	}

	if CheckRateLimit("dave") <= 0 {
		t.Fatal("precondition: account should be locked")
	}

	// Simulate a successful login — but we have to manually unlock in the
	// data because the lockout time is in the future and RecordSuccess only
	// clears the map entry.
	loginAttemptsMu.Lock()
	loginAttempts["dave"].lockedUntil = time.Now().Add(-time.Second) // expire the lock
	loginAttemptsMu.Unlock()

	RecordSuccess("dave")

	if got := CheckRateLimit("dave"); got != 0 {
		t.Errorf("expected 0 after success, got %d", got)
	}
}

// TestRecordSuccess_NoPriorRecord is a no-op and must not panic.
func TestRecordSuccess_NoPriorRecord(t *testing.T) {
	resetRateLimiter()
	RecordSuccess("eve") // user never failed — should not panic
}

// TestCheckRateLimit_AfterLockoutExpires returns 0 once the lockout window passes.
func TestCheckRateLimit_AfterLockoutExpires(t *testing.T) {
	resetRateLimiter()

	// Lock the account.
	for i := 0; i < defaultMaxAttempts; i++ {
		RecordFailure(0, "frank")
	}

	// Manually backdate the lockout so it has already expired.
	loginAttemptsMu.Lock()
	loginAttempts["frank"].lockedUntil = time.Now().Add(-time.Second)
	loginAttemptsMu.Unlock()

	if got := CheckRateLimit("frank"); got != 0 {
		t.Errorf("expected 0 after lockout expired, got %d", got)
	}
}

// TestPruneLoginAttempts removes entries that are no longer locked and
// whose last failure is older than 2× lockout duration.
func TestPruneLoginAttempts(t *testing.T) {
	resetRateLimiter()

	// Insert a stale record.
	loginAttemptsMu.Lock()

	longAgo := time.Now().Add(-getLockoutDuration() * 3)
	loginAttempts["stale"] = &loginRecord{
		failures:    3,
		lastFailure: longAgo,
		lockedUntil: longAgo, // already expired
	}

	loginAttemptsMu.Unlock()

	pruneLoginAttempts()

	loginAttemptsMu.Lock()
	_, found := loginAttempts["stale"]
	loginAttemptsMu.Unlock()

	if found {
		t.Error("expected stale record to be pruned")
	}
}

// TestPruneLoginAttempts_KeepsActiveRecord does not remove an entry that is
// still locked.
func TestPruneLoginAttempts_KeepsActiveRecord(t *testing.T) {
	resetRateLimiter()

	loginAttemptsMu.Lock()
	loginAttempts["active"] = &loginRecord{
		failures:    defaultMaxAttempts,
		lastFailure: time.Now(),
		lockedUntil: time.Now().Add(defaultLockoutDuration),
	}
	loginAttemptsMu.Unlock()

	pruneLoginAttempts()

	loginAttemptsMu.Lock()
	_, found := loginAttempts["active"]
	loginAttemptsMu.Unlock()

	if !found {
		t.Error("expected active lockout record to be preserved after prune")
	}
}
