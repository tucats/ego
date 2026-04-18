package server

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/util"
)

const (
	// webAuthnRateWindow is the sliding window duration for per-IP rate limiting.
	webAuthnRateWindow = time.Minute

	// webAuthnRateMax is the maximum number of ceremony-begin requests allowed
	// per IP address within webAuthnRateWindow.
	webAuthnRateMax = 10

	// webAuthnMaxPending is the maximum number of concurrently pending WebAuthn
	// ceremonies (entries in the challenge cache). Requests that would exceed
	// this cap are rejected with 429.
	webAuthnMaxPending = 200
)

// webAuthnIPLimiter tracks per-IP request timestamps for sliding-window rate
// limiting of the WebAuthn ceremony-begin endpoints.
type webAuthnIPLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

var webAuthnLimiter = &webAuthnIPLimiter{
	windows: make(map[string][]time.Time),
}

// allow returns true if the given IP address is within its rate limit budget,
// and records the current request. It prunes timestamps outside the window on
// every call so the map does not grow without bound.
func (l *webAuthnIPLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-webAuthnRateWindow)

	// Prune timestamps that have fallen outside the window.
	prev := l.windows[ip]
	valid := prev[:0]

	for _, t := range prev {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= webAuthnRateMax {
		l.windows[ip] = valid

		return false
	}

	l.windows[ip] = append(valid, now)

	return true
}

// clientIP extracts the remote IP address from a request, honouring
// X-Forwarded-For when the direct peer is a loopback address (i.e. the server
// is behind a local reverse proxy).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if host == "127.0.0.1" || host == "::1" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// X-Forwarded-For may be a comma-separated list; take the first entry.
			ip, _, _ := net.SplitHostPort(fwd)
			if ip == "" {
				ip = fwd
			}

			if idx := len(ip); idx > 0 {
				return ip
			}
		}
	}

	return host
}

// webAuthnBeginGuard checks both the per-IP rate limit and the global pending-
// ceremony cap before a ceremony-begin handler allocates a new challenge nonce.
// It writes a 429 response and returns a non-zero status code when either limit
// is exceeded; otherwise it returns 0 and the caller proceeds normally.
func webAuthnBeginGuard(w http.ResponseWriter, session *Session, r *http.Request) int {
	ip := clientIP(r)

	if !webAuthnLimiter.allow(ip) {
		ui.Log(ui.AuthLogger, "auth.webauthn.rate.limited", ui.A{
			"session": session.ID,
			"ip":      ip,
		})

		return util.ErrorResponse(w, session.ID, "too many requests", http.StatusTooManyRequests)
	}

	pending := caches.Size(caches.WebAuthnChallengeCache)
	if pending >= webAuthnMaxPending {
		ui.Log(ui.AuthLogger, "auth.webauthn.capacity", ui.A{
			"session": session.ID,
			"count":   pending,
		})

		return util.ErrorResponse(w, session.ID, "server busy", http.StatusTooManyRequests)
	}

	return 0
}
