package cluster

// Tests for authorizeClusterOrAdmin, added as part of the REST-3 audit
// (docs/issues/REST-3.md, section 3): ClusterShutdownHandler and
// ClusterRemoveHandler used to fold "no credentials presented at all" and
// "authenticated, but not an admin" into a single 403 -- these tests pin the
// corrected split: 401 for the former (with a WWW-Authenticate header, per
// RFC 7235 §3.1), 403 for the latter.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
)

func TestAuthorizeClusterOrAdmin_ValidClusterToken_Allowed(t *testing.T) {
	clusterTestMode(t)

	req, _ := http.NewRequest(http.MethodPost, "/services/cluster/shutdown", nil)
	req.Header.Set("Authorization", ClusterAuthHeader())

	rr := httptest.NewRecorder()
	status := authorizeClusterOrAdmin(&router.Session{ID: 1}, rr, req)

	if status != 0 {
		t.Errorf("expected 0 (allowed), got %d -- body: %s", status, rr.Body.String())
	}
}

func TestAuthorizeClusterOrAdmin_AdminSession_Allowed(t *testing.T) {
	clusterTestMode(t)

	req, _ := http.NewRequest(http.MethodPost, "/services/cluster/shutdown", nil)

	rr := httptest.NewRecorder()
	status := authorizeClusterOrAdmin(&router.Session{ID: 1, Authenticated: true, Admin: true}, rr, req)

	if status != 0 {
		t.Errorf("expected 0 (allowed), got %d -- body: %s", status, rr.Body.String())
	}
}

func TestAuthorizeClusterOrAdmin_NoCredentials_ReturnsUnauthorized(t *testing.T) {
	clusterTestMode(t)

	req, _ := http.NewRequest(http.MethodPost, "/services/cluster/shutdown", nil)

	rr := httptest.NewRecorder()
	status := authorizeClusterOrAdmin(&router.Session{ID: 1}, rr, req)

	if status != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d -- body: %s", status, rr.Body.String())
	}

	if got := rr.Header().Get(defs.AuthenticateHeader); got == "" {
		t.Error("expected a WWW-Authenticate header on a 401, got none")
	}
}

func TestAuthorizeClusterOrAdmin_AuthenticatedNonAdmin_ReturnsForbidden(t *testing.T) {
	clusterTestMode(t)

	req, _ := http.NewRequest(http.MethodPost, "/services/cluster/shutdown", nil)

	rr := httptest.NewRecorder()
	status := authorizeClusterOrAdmin(&router.Session{ID: 1, Authenticated: true, Admin: false}, rr, req)

	if status != http.StatusForbidden {
		t.Errorf("expected 403, got %d -- body: %s", status, rr.Body.String())
	}

	if got := rr.Header().Get(defs.AuthenticateHeader); got != "" {
		t.Errorf("expected no WWW-Authenticate header on a 403 (caller is identified, just not authorized), got %q", got)
	}
}
