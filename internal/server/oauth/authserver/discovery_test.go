package authserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/router"
)

func TestBuildDiscoveryDoc_ContainsRequiredFields(t *testing.T) {
	issuer := "https://ego.test"

	if err := buildDiscoveryDoc(issuer); err != nil {
		t.Fatalf("buildDiscoveryDoc error: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(discoveryDoc, &doc); err != nil {
		t.Fatalf("discovery doc is not valid JSON: %v", err)
	}

	required := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"userinfo_endpoint",
		"jwks_uri",
		"revocation_endpoint",
		"response_types_supported",
		"grant_types_supported",
	}

	for _, field := range required {
		if _, ok := doc[field]; !ok {
			t.Errorf("discovery document missing required field: %q", field)
		}
	}

	if doc["issuer"] != issuer {
		t.Errorf("issuer mismatch: expected %q, got %v", issuer, doc["issuer"])
	}
}

func TestDiscoveryHandler_Returns200(t *testing.T) {
	if err := buildDiscoveryDoc("https://ego.test"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()

	session := &router.Session{ID: 1}
	status := DiscoveryHandler(session, w, req)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
}

func TestJWKSHandler_Returns200(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("key setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	w := httptest.NewRecorder()

	session := &router.Session{ID: 1}
	status := JWKSHandler(session, w, req)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
}
