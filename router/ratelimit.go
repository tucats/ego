package router

import (
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

const (
	// defaultMaxAttempts is the number of consecutive failed logins allowed
	// before an account is temporarily locked. Overridden by AuthMaxAttemptsSetting.
	defaultMaxAttempts = 5

	// defaultLockoutDuration is how long an account stays locked.
	// Overridden by AuthLockoutDurationSetting.
	defaultLockoutDuration = 15 * time.Minute

	// scanInterval is how often the background goroutine prunes stale entries.
	rateLimitScanInterval = 5 * time.Minute
)

// loginRecord tracks failed login attempts for one username.
type loginRecord struct {
	failures    int
	lastFailure time.Time
	lockedUntil time.Time
}

var (
	loginAttempts   = map[string]*loginRecord{}
	loginAttemptsMu sync.Mutex
	scanOnce        sync.Once
)

// startRateLimitScan starts the single background goroutine that prunes stale
// entries. It is safe to call multiple times; only one goroutine is ever started.
func startRateLimitScan() {
	scanOnce.Do(func() {
		go func() {
			for {
				time.Sleep(rateLimitScanInterval)
				pruneLoginAttempts()
			}
		}()
	})
}

// pruneLoginAttempts removes entries for accounts that are no longer locked
// and have not failed recently.
func pruneLoginAttempts() {
	cutoff := time.Now().Add(-getLockoutDuration() * 2)

	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	for user, rec := range loginAttempts {
		if time.Now().After(rec.lockedUntil) && rec.lastFailure.Before(cutoff) {
			delete(loginAttempts, user)
		}
	}
}

// getLockoutDuration returns the configured lockout duration, falling back to
// the default when the setting is absent or unparsable.
func getLockoutDuration() time.Duration {
	if s := settings.Get(defs.AuthLockoutDurationSetting); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}

	return defaultLockoutDuration
}

// getMaxAttempts returns the configured failed-attempt threshold, falling back
// to the default. A configured value of 0 disables lockout entirely.
func getMaxAttempts() int {
	if n := settings.GetInt(defs.AuthMaxAttemptsSetting); n >= 0 {
		// GetInt returns 0 for a missing key; we treat that as "use default"
		// unless the key was explicitly set to 0.
		if settings.Get(defs.AuthMaxAttemptsSetting) != "" {
			return n
		}
	}

	return defaultMaxAttempts
}

// CheckRateLimit returns the number of seconds the client must wait before
// retrying (> 0 means locked out), or 0 if the attempt is allowed.
// username is the credential being checked.
func CheckRateLimit(username string) int {
	startRateLimitScan()

	// A max of 0 means lockout is disabled.
	if getMaxAttempts() == 0 {
		return 0
	}

	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	rec, found := loginAttempts[username]
	if !found {
		return 0
	}

	if time.Now().Before(rec.lockedUntil) {
		// Add 1 so the value is never 0 when locked (avoids ambiguity).
		return int(time.Until(rec.lockedUntil).Seconds()) + 1
	}

	return 0
}

// RecordFailure increments the failure counter for username. If the configured
// threshold is reached and the account is not already locked, it is locked for
// the configured duration and the event is logged.
func RecordFailure(session int, username string) {
	startRateLimitScan()

	maxAttempts := getMaxAttempts()
	if maxAttempts == 0 {
		return
	}

	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	rec, found := loginAttempts[username]
	if !found {
		rec = &loginRecord{}
		loginAttempts[username] = rec
	}

	rec.failures++
	rec.lastFailure = time.Now()

	// Lock the account if we just crossed the threshold and it isn't already locked.
	if rec.failures >= maxAttempts && time.Now().After(rec.lockedUntil) {
		rec.lockedUntil = time.Now().Add(getLockoutDuration())

		ui.Log(ui.AuthLogger, "auth.account.locked", ui.A{
			"session": session,
			"user":    username,
			"count":   rec.failures,
			"until":   rec.lockedUntil.Format(time.RFC3339),
		})
	}
}

// RecordSuccess clears any failure record for username after a successful login,
// allowing a fresh attempt counter on the next failure.
func RecordSuccess(username string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	delete(loginAttempts, username)
}
