package router

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
)

const localHost = "localhost"

// ─── challengeCookie ─────────────────────────────────────────────────────────

func TestChallengeCookie_Fields(t *testing.T) {
	nonce := "test-nonce-abc"
	c := challengeCookie(nonce, 300, false)

	if c.Name != webAuthnChallengeCookie {
		t.Errorf("Name = %q, want %q", c.Name, webAuthnChallengeCookie)
	}

	if c.Value != nonce {
		t.Errorf("Value = %q, want %q", c.Value, nonce)
	}

	if c.MaxAge != 300 {
		t.Errorf("MaxAge = %d, want 300", c.MaxAge)
	}

	if !c.HttpOnly {
		t.Error("expected HttpOnly == true")
	}

	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want SameSiteStrictMode", c.SameSite)
	}
}

func TestChallengeCookie_ExpiryMaxAge(t *testing.T) {
	c := challengeCookie("", -1, false)

	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1 for expiry cookie", c.MaxAge)
	}
}

func TestChallengeCookie_SecureFlag(t *testing.T) {
	if c := challengeCookie("n", 60, true); !c.Secure {
		t.Error("expected Secure == true when secure=true")
	}

	if c := challengeCookie("n", 60, false); c.Secure {
		t.Error("expected Secure == false when secure=false")
	}
}

func TestIsSecureRequest_TLS(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.TLS = &tls.ConnectionState{}

	if !isSecureRequest(r) {
		t.Error("expected isSecureRequest == true for TLS connection")
	}
}

func TestIsSecureRequest_ForwardedProto(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")

	if !isSecureRequest(r) {
		t.Error("expected isSecureRequest == true for X-Forwarded-Proto: https")
	}
}

func TestIsSecureRequest_PlainHTTP(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)

	if isSecureRequest(r) {
		t.Error("expected isSecureRequest == false for plain HTTP")
	}
}

// ─── storeChallenge / loadChallenge ──────────────────────────────────────────

// minimalSessionData returns a webauthn.SessionData with enough fields to
// survive a JSON round-trip through storeChallenge / loadChallenge.
func minimalSessionData() *webauthn.SessionData {
	return &webauthn.SessionData{
		Challenge: "dGVzdC1jaGFsbGVuZ2U", // base64url "test-challenge"
		UserID:    []byte("test-user-id"),
	}
}

func TestStoreLoadChallenge_RoundTrip(t *testing.T) {
	sd := minimalSessionData()

	nonce, err := storeChallenge(sd)
	if err != nil {
		t.Fatalf("storeChallenge: %v", err)
	}

	if nonce == "" {
		t.Fatal("expected non-empty nonce")
	}

	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.AddCookie(challengeCookie(nonce, 300, false))

	got, err := loadChallenge(r)
	if err != nil {
		t.Fatalf("loadChallenge: %v", err)
	}

	if got.Challenge != sd.Challenge {
		t.Errorf("Challenge = %q, want %q", got.Challenge, sd.Challenge)
	}
}

func TestLoadChallenge_MissingCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)

	_, err := loadChallenge(r)
	if err == nil {
		t.Fatal("expected error for missing cookie")
	}
}

func TestLoadChallenge_UnknownNonce(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.AddCookie(challengeCookie(uuid.New().String(), 300, false))

	_, err := loadChallenge(r)
	if err == nil {
		t.Fatal("expected error for unknown nonce")
	}
}

func TestStoreLoadChallenge_SingleUse(t *testing.T) {
	nonce, err := storeChallenge(minimalSessionData())
	if err != nil {
		t.Fatalf("storeChallenge: %v", err)
	}

	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.AddCookie(challengeCookie(nonce, 300, false))

	if _, err := loadChallenge(r); err != nil {
		t.Fatalf("first loadChallenge: %v", err)
	}

	// Nonce is consumed; second call must fail.
	if _, err := loadChallenge(r); err == nil {
		t.Fatal("expected error on second load of the same nonce")
	}
}

// ─── findUserByHandle ────────────────────────────────────────────────────────

func TestFindUserByHandle_Found(t *testing.T) {
	admin, err := auth.AuthService.ReadUser(0, defs.DefaultAdminUsername, true)
	if err != nil {
		t.Fatalf("ReadUser: %v", err)
	}

	handle, _ := admin.ID.MarshalBinary()

	found, err := findUserByHandle(0, handle)
	if err != nil {
		t.Fatalf("findUserByHandle: %v", err)
	}

	if found.Name != defs.DefaultAdminUsername {
		t.Errorf("Name = %q, want %q", found.Name, defs.DefaultAdminUsername)
	}
}

func TestFindUserByHandle_NotFound(t *testing.T) {
	bogusID := uuid.New()
	handle, _ := bogusID.MarshalBinary()

	_, err := findUserByHandle(0, handle)
	if err == nil {
		t.Fatal("expected error for unknown handle")
	}
}

// ─── clientIP ────────────────────────────────────────────────────────────────

func TestClientIP_DirectAddress(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "192.168.1.42:54321"

	if got := clientIP(r); got != "192.168.1.42" {
		t.Errorf("clientIP = %q, want 192.168.1.42", got)
	}
}

func TestClientIP_LoopbackUsesForwardedFor(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "10.0.0.5")

	if got := clientIP(r); got != "10.0.0.5" {
		t.Errorf("clientIP = %q, want 10.0.0.5", got)
	}
}

func TestClientIP_LoopbackNoForwarded(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"

	if got := clientIP(r); got != "127.0.0.1" {
		t.Errorf("clientIP = %q, want 127.0.0.1", got)
	}
}

// ─── webAuthnIPLimiter ────────────────────────────────────────────────────────

func freshLimiter() *webAuthnIPLimiter {
	return &webAuthnIPLimiter{windows: make(map[string][]time.Time)}
}

func TestIPLimiter_AllowsUnderLimit(t *testing.T) {
	l := freshLimiter()

	for i := 0; i < webAuthnRateMax; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("expected allow on request %d", i+1)
		}
	}
}

func TestIPLimiter_BlocksOverLimit(t *testing.T) {
	l := freshLimiter()

	for i := 0; i < webAuthnRateMax; i++ {
		l.allow("1.2.3.4")
	}

	if l.allow("1.2.3.4") {
		t.Error("expected deny after rate limit reached")
	}
}

func TestIPLimiter_DifferentIPsAreIndependent(t *testing.T) {
	l := freshLimiter()

	for i := 0; i < webAuthnRateMax; i++ {
		l.allow("1.1.1.1")
	}

	if !l.allow("2.2.2.2") {
		t.Error("rate limit on 1.1.1.1 should not affect 2.2.2.2")
	}
}

// ─── webAuthnBeginGuard ───────────────────────────────────────────────────────

func TestWebAuthnBeginGuard_RateLimitBlocks(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	// Replace the package-level limiter with a fresh one and exhaust the budget.
	saved := webAuthnLimiter
	webAuthnLimiter = freshLimiter()

	defer func() { webAuthnLimiter = saved }()

	const ip = "5.5.5.5"

	for i := 0; i < webAuthnRateMax; i++ {
		webAuthnLimiter.allow(ip)
	}

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = ip + ":9999"
	s := &Session{ID: 1}

	code := webAuthnBeginGuard(w, s, r)
	if code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", code)
	}
}

func TestWebAuthnBeginGuard_CapacityBlocks(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")
	caches.Purge(caches.WebAuthnChallengeCache)

	// Fill the cache above the cap.
	for i := 0; i < webAuthnMaxPending; i++ {
		caches.Add(caches.WebAuthnChallengeCache, uuid.New().String(), []byte("x"))
	}

	defer caches.Purge(caches.WebAuthnChallengeCache)

	// Use a fresh limiter so rate limit does not interfere.
	saved := webAuthnLimiter
	webAuthnLimiter = freshLimiter()

	defer func() { webAuthnLimiter = saved }()

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "6.6.6.6:1234"
	s := &Session{ID: 1}

	code := webAuthnBeginGuard(w, s, r)
	if code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", code)
	}
}

// ─── passkeyGuard ────────────────────────────────────────────────────────────

func TestPasskeyGuard_DisabledBlocksLoginBegin(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "false")
	defer settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/services/admin/webauthn/login/begin", nil)
	r.Host = localHost
	s := &Session{ID: 1}

	code := WebAuthnLoginBeginHandler(s, w, r)
	if code != http.StatusNotFound {
		t.Errorf("login/begin with passkeys disabled: status = %d, want 404", code)
	}
}

func TestPasskeyGuard_DisabledBlocksRegisterBegin(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "false")
	defer settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/services/admin/webauthn/register/begin", nil)
	r.Host = localHost
	s := &Session{ID: 1, User: defs.DefaultAdminUsername}

	code := WebAuthnRegisterBeginHandler(s, w, r)
	if code != http.StatusNotFound {
		t.Errorf("register/begin with passkeys disabled: status = %d, want 404", code)
	}
}

func TestPasskeyGuard_DisabledBlocksClearPasskeys(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "false")
	defer settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodDelete, "/services/admin/webauthn/passkeys/"+defs.DefaultAdminUsername, nil)
	s := &Session{
		ID:       1,
		User:     defs.DefaultAdminUsername,
		URLParts: map[string]any{"name": defs.DefaultAdminUsername},
	}

	code := WebAuthnClearPasskeysHandler(s, w, r)
	if code != http.StatusNotFound {
		t.Errorf("clear passkeys with passkeys disabled: status = %d, want 404", code)
	}
}

// ─── WebAuthnConfigHandler ───────────────────────────────────────────────────

func TestWebAuthnConfigHandler_PasskeysEnabled(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/services/admin/webauthn/config", nil)
	s := &Session{ID: 1}

	code := WebAuthnConfigHandler(s, w, r)
	if code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}

	var resp struct {
		Passkeys bool `json:"passkeys"`
		Status   int  `json:"status"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !resp.Passkeys {
		t.Error("expected passkeys == true")
	}

	if resp.Status != 200 {
		t.Errorf("status field = %d, want 200", resp.Status)
	}
}

func TestWebAuthnConfigHandler_PasskeysDisabled(t *testing.T) {
	settings.Set(defs.WebAuthnAllowPasskeysSetting, "false")
	defer settings.Set(defs.WebAuthnAllowPasskeysSetting, "true")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/services/admin/webauthn/config", nil)
	s := &Session{ID: 1}

	WebAuthnConfigHandler(s, w, r)

	var resp struct {
		Passkeys bool `json:"passkeys"`
	}

	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Passkeys {
		t.Error("expected passkeys == false")
	}
}

// ─── WebAuthnClearPasskeysHandler ────────────────────────────────────────────

const clearTestUser = "webauthn-clear-test-user"

func addWebAuthnTestUser(t *testing.T, name string, perms []string) {
	t.Helper()

	u := defs.User{
		ID:          uuid.New(),
		Name:        name,
		Password:    "x",
		Permissions: perms,
	}

	if err := auth.AuthService.WriteUser(0, u); err != nil {
		t.Fatalf("addWebAuthnTestUser(%q): %v", name, err)
	}
}

func removeWebAuthnTestUser(name string) {
	_ = auth.AuthService.DeleteUser(0, name)
}

func TestWebAuthnClearPasskeysHandler_OwnAccount(t *testing.T) {
	addWebAuthnTestUser(t, clearTestUser, []string{defs.LogonPermission})
	defer removeWebAuthnTestUser(clearTestUser)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodDelete, "/services/admin/webauthn/passkeys/"+clearTestUser, nil)
	s := &Session{
		ID:       1,
		User:     clearTestUser,
		URLParts: map[string]any{"name": clearTestUser},
	}

	code := WebAuthnClearPasskeysHandler(s, w, r)
	if code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", code, w.Body.String())
	}
}

func TestWebAuthnClearPasskeysHandler_AdminClearsOtherUser(t *testing.T) {
	addWebAuthnTestUser(t, clearTestUser, []string{defs.LogonPermission})
	defer removeWebAuthnTestUser(clearTestUser)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodDelete, "/services/admin/webauthn/passkeys/"+clearTestUser, nil)
	s := &Session{
		ID:       1,
		User:     defs.DefaultAdminUsername,
		URLParts: map[string]any{"name": clearTestUser},
	}

	code := WebAuthnClearPasskeysHandler(s, w, r)
	if code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", code, w.Body.String())
	}
}

func TestWebAuthnClearPasskeysHandler_NonAdminForbidden(t *testing.T) {
	const victimUser = "webauthn-clear-victim"

	addWebAuthnTestUser(t, clearTestUser, []string{defs.LogonPermission})
	addWebAuthnTestUser(t, victimUser, []string{defs.LogonPermission})

	defer removeWebAuthnTestUser(clearTestUser)
	defer removeWebAuthnTestUser(victimUser)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodDelete, "/services/admin/webauthn/passkeys/"+victimUser, nil)
	s := &Session{
		ID:       1,
		User:     clearTestUser, // non-admin trying to clear another user
		URLParts: map[string]any{"name": victimUser},
	}

	code := WebAuthnClearPasskeysHandler(s, w, r)
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", code)
	}
}

func TestWebAuthnClearPasskeysHandler_UserNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodDelete, "/services/admin/webauthn/passkeys/nosuchuser", nil)
	s := &Session{
		ID:       1,
		User:     defs.DefaultAdminUsername,
		URLParts: map[string]any{"name": "nosuchuser"},
	}

	code := WebAuthnClearPasskeysHandler(s, w, r)
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// ─── WebAuthnLoginBeginHandler ───────────────────────────────────────────────

func TestWebAuthnLoginBeginHandler_HappyPath(t *testing.T) {
	caches.Purge(caches.WebAuthnChallengeCache)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/services/admin/webauthn/login/begin", nil)
	r.Host = localHost
	s := &Session{ID: 1}

	code := WebAuthnLoginBeginHandler(s, w, r)
	if code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := body["publicKey"]; !ok {
		t.Error("expected publicKey field in login begin response")
	}

	// A challenge cookie must be set on the response.
	cookieFound := false

	for _, c := range w.Result().Cookies() {
		if c.Name == webAuthnChallengeCookie {
			cookieFound = true

			break
		}
	}

	if !cookieFound {
		t.Errorf("expected %s cookie in response", webAuthnChallengeCookie)
	}
}

// ─── WebAuthnRegisterBeginHandler ────────────────────────────────────────────

func TestWebAuthnRegisterBeginHandler_UserNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/services/admin/webauthn/register/begin", nil)
	r.Host = localHost
	s := &Session{
		ID:   1,
		User: "nosuchuser",
	}

	code := WebAuthnRegisterBeginHandler(s, w, r)
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", code)
	}
}

func TestWebAuthnRegisterBeginHandler_HappyPath(t *testing.T) {
	caches.Purge(caches.WebAuthnChallengeCache)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/services/admin/webauthn/register/begin", nil)
	r.Host = localHost
	s := &Session{
		ID:   1,
		User: defs.DefaultAdminUsername,
	}

	code := WebAuthnRegisterBeginHandler(s, w, r)
	if code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := body["publicKey"]; !ok {
		t.Error("expected publicKey field in register begin response")
	}

	// A challenge cookie must be set on the response.
	cookieFound := false

	for _, c := range w.Result().Cookies() {
		if c.Name == webAuthnChallengeCookie {
			cookieFound = true

			break
		}
	}

	if !cookieFound {
		t.Errorf("expected %s cookie in response", webAuthnChallengeCookie)
	}
}
