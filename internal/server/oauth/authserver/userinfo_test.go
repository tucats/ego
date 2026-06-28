package authserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tucats/ego/internal/router"
)

func TestUserinfoHandler_MissingBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	w := httptest.NewRecorder()

	session := &router.Session{ID: 1}
	status := UserinfoHandler(session, w, req)

	if status != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing Bearer, got %d", status)
	}
}

func TestUserinfoHandler_ValidToken(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("key setup: %v", err)
	}

	cfg := asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	tokenStr, _, err := createAccessToken(cfg, "myclient", "alice", "myclient", "openid profile email")
	if err != nil {
		t.Fatalf("token creation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()

	session := &router.Session{ID: 1}
	status := UserinfoHandler(session, w, req)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	var resp UserinfoResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	if resp.Sub != "alice" {
		t.Errorf("expected sub=alice, got %q", resp.Sub)
	}

	if resp.Name != "alice" {
		t.Errorf("expected name=alice, got %q", resp.Name)
	}
}

func TestUserinfoHandler_ClientCredentialToken(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("key setup: %v", err)
	}

	cfg := asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	// client_credentials token has empty subject.
	tokenStr, _, err := createAccessToken(cfg, "myclient", "", "myclient", "ego:read")
	if err != nil {
		t.Fatalf("token creation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	
	w := httptest.NewRecorder()

	session := &router.Session{ID: 1}
	status := UserinfoHandler(session, w, req)

	// Client credentials tokens have no subject — should be forbidden.
	if status != http.StatusForbidden {
		t.Errorf("expected 403 for client credentials token, got %d", status)
	}
}
