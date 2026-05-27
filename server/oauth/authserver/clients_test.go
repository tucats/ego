package authserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClients_MissingFile(t *testing.T) {
	err := loadClients("/nonexistent/path/clients.json")
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}

	if len(clients) != 0 {
		t.Errorf("expected empty client list, got %d clients", len(clients))
	}
}

func TestLoadClients_HashesPlaintextSecret(t *testing.T) {
	dir := t.TempDir()
	clientFile := filepath.Join(dir, "clients.json")

	rawClients := []map[string]any{
		{
			"client_id":     "testclient",
			"client_secret": "mysecret",
			"redirect_uris": []string{"https://example.com/cb"},
			"grant_types":   []string{"authorization_code"},
			"scopes":        []string{"openid"},
		},
	}

	data, _ := json.Marshal(rawClients)
	os.WriteFile(clientFile, data, 0600)

	if err := loadClients(clientFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	// The plaintext secret should have been cleared.
	if clients[0].ClientSecret != "" {
		t.Error("plaintext secret should be cleared after load")
	}

	// The hash should be populated.
	if clients[0].ClientSecretHash == "" {
		t.Error("client_secret_hash should be populated after load")
	}
}

func TestFindClient(t *testing.T) {
	clients = []OAuthClient{
		{ClientID: "alpha", Scopes: []string{"openid"}},
		{ClientID: "beta", Scopes: []string{"ego:read"}},
	}

	if c := findClient("alpha"); c == nil || c.ClientID != "alpha" {
		t.Error("findClient(alpha) failed")
	}

	if c := findClient("gamma"); c != nil {
		t.Error("findClient(gamma) should return nil")
	}
}

func TestValidateClientSecret_Public(t *testing.T) {
	c := &OAuthClient{ClientID: "pub", ClientSecretHash: ""}
	// Public client — any secret is accepted.
	if !validateClientSecret(c, "") {
		t.Error("public client should accept empty secret")
	}

	if !validateClientSecret(c, "anything") {
		t.Error("public client should accept any secret")
	}
}

func TestValidateClientSecret_Confidential(t *testing.T) {
	// Pre-hash "correct" using a known bcrypt hash (cost 10).
	dir := t.TempDir()
	clientFile := filepath.Join(dir, "clients.json")

	rawClients := []map[string]any{
		{
			"client_id":     "conf",
			"client_secret": "correct",
			"redirect_uris": []string{},
			"grant_types":   []string{"client_credentials"},
			"scopes":        []string{"openid"},
		},
	}

	data, _ := json.Marshal(rawClients)
	os.WriteFile(clientFile, data, 0600)
	_ = loadClients(clientFile)

	c := findClient("conf")
	if c == nil {
		t.Fatal("client not found")
	}

	if !validateClientSecret(c, "correct") {
		t.Error("expected correct password to be accepted")
	}

	if validateClientSecret(c, "wrong") {
		t.Error("expected wrong password to be rejected")
	}
}

func TestClientAllowsGrant(t *testing.T) {
	c := &OAuthClient{GrantTypes: []string{"authorization_code", "refresh_token"}}
	if !clientAllowsGrant(c, "authorization_code") {
		t.Error("should allow authorization_code")
	}

	if clientAllowsGrant(c, "client_credentials") {
		t.Error("should not allow client_credentials")
	}
}

func TestClientAllowsScope(t *testing.T) {
	c := &OAuthClient{Scopes: []string{"openid", "profile", "ego:read"}}

	if !clientAllowsScope(c, []string{"openid", "profile"}) {
		t.Error("should allow openid+profile")
	}

	if clientAllowsScope(c, []string{"openid", "ego:admin"}) {
		t.Error("should not allow ego:admin")
	}
}
