package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/cipher"
	auth "github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

const webAuthnChallengeCookie = "webauthn_challenge"

// ── helpers ───────────────────────────────────────────────────────────────────

// webAuthnInstance returns a configured *webauthn.WebAuthn or writes a 500
// response and returns nil on error. The RPID and allowed origin are derived
// from r when ego.server.webauthn.rpid is not explicitly configured, enabling
// zero-config local development with Apple passkeys.
func webAuthnInstance(w http.ResponseWriter, r *http.Request, sessionID int) *webauthn.WebAuthn {
	wau, err := auth.NewWebAuthnForRequest(r)
	if err != nil {
		msg := fmt.Sprintf("WebAuthn not available: %v", err)
		ui.Log(ui.AuthLogger, "auth.webauthn.config.error", ui.A{
			"session": sessionID,
			"error":   err})
		http.Error(w, msg, http.StatusInternalServerError)

		return nil
	}

	return wau
}

// challengeCookie returns a short-lived HttpOnly cookie carrying a session nonce.
func challengeCookie(nonce string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     webAuthnChallengeCookie,
		Value:    nonce,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		// Secure is not forced here so localhost dev still works; in production
		// the server should be behind HTTPS which is required by browsers anyway.
	}
}

// storeChallenge stores webauthn SessionData in the challenge cache keyed by
// the supplied nonce UUID and returns the nonce string.
func storeChallenge(sessionData *webauthn.SessionData) (string, error) {
	nonce := uuid.New().String()

	b, err := json.Marshal(sessionData)
	if err != nil {
		return "", err
	}

	caches.Add(caches.WebAuthnChallengeCache, nonce, b)

	return nonce, nil
}

// loadChallenge retrieves and removes SessionData from the challenge cache using
// the nonce stored in the request cookie.
func loadChallenge(r *http.Request) (*webauthn.SessionData, error) {
	cookie, err := r.Cookie(webAuthnChallengeCookie)
	if err != nil {
		return nil, fmt.Errorf("missing %s cookie", webAuthnChallengeCookie)
	}

	nonce := cookie.Value

	v, found := caches.Find(caches.WebAuthnChallengeCache, nonce)
	if !found {
		return nil, fmt.Errorf("WebAuthn session expired or not found")
	}

	caches.Delete(caches.WebAuthnChallengeCache, nonce)

	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid session data type in cache")
	}

	var sd webauthn.SessionData

	if err := json.Unmarshal(b, &sd); err != nil {
		return nil, err
	}

	return &sd, nil
}

// issueToken mints a bearer token for the named user and writes a LogonResponse.
// This mirrors the logic in LogonHandler so both paths produce identical responses.
func issueToken(w http.ResponseWriter, session *Session, username string) int {
	s := symbols.NewRootSymbolTable("webauthn logon")
	s.SetAlways("cipher", cipher.CipherPackage)
	s.SetAlways(defs.SessionVariable, session.ID)

	v, err := builtins.CallBuiltin(s, "cipher.New", username, "", "")
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	tokenStr, ok := v.(string)
	if !ok {
		msg := fmt.Sprintf("invalid internal token data type: %s", data.TypeOf(v).String())
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   msg})

		return util.ErrorResponse(w, session.ID, msg, http.StatusInternalServerError)
	}

	serverDurationString := settings.Get(defs.ServerTokenExpirationSetting)
	if serverDurationString == "" {
		serverDurationString = "15m"
		settings.SetDefault(defs.ServerTokenExpirationSetting, serverDurationString)
	}

	duration, _ := util.ParseDuration(serverDurationString)

	perms := auth.GetPermissions(session.ID, username)
	isAdmin := util.InListInsensitive(defs.RootPermission, perms...)
	canCode := isAdmin || util.InList(defs.CodeRunPermission, perms...)

	response := defs.LogonResponse{
		Identity: username,
		RestStatusResponse: defs.RestStatusResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			Status:     http.StatusOK,
		},
		Token:      tokenStr,
		Expiration: time.Now().Add(duration).Format(time.UnixDate),
		CanAdmin:   isAdmin,
		CanCode:    canCode,
	}

	if t, err := tokens.Unwrap(tokenStr, 0); err == nil {
		response.ID = t.TokenID.String()
	}

	w.Header().Add(defs.ContentTypeHeader, defs.LogonMediaType)

	_ = util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		response.Token = egostrings.TruncateMiddle(tokenStr, 10)

		b, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// findUserByHandle searches all users for the one whose UUID matches the bytes
// in userHandle (the value returned by webAuthnUser.WebAuthnID).
func findUserByHandle(sessionID int, userHandle []byte) (defs.User, error) {
	for _, u := range auth.AuthService.ListUsers(false) {
		b, _ := u.ID.MarshalBinary()
		if strings.EqualFold(fmt.Sprintf("%x", b), fmt.Sprintf("%x", userHandle)) {
			ui.Log(ui.AuthLogger, "auth.webauth.found.by.handle", ui.A{
				"session": sessionID,
				"user":    u.Name,
			})

			return u, nil
		}
	}

	ui.Log(ui.AuthLogger, "auth.webauth.not.found.by.handle", ui.A{
		"session": sessionID,
	})

	return defs.User{}, errors.ErrNoSuchUser
}

// ── handlers ──────────────────────────────────────────────────────────────────

// WebAuthnConfigHandler returns the server's passkey feature flag.
// GET /services/admin/webauthn/config — no authentication required.
func WebAuthnConfigHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	enabled := settings.GetBool(defs.WebAuthnAllowPasskeysSetting)

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)

	resp := struct {
		Server   defs.ServerInfo `json:"server"`
		Msg      string          `json:"msg"`
		Status   int             `json:"status"`
		Passkeys bool            `json:"passkeys"`
	}{
		Server:   util.MakeServerInfo(session.ID),
		Passkeys: enabled,
		Status:   200,
		Msg:      "",
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return http.StatusOK
}

// passkeyGuard returns a non-zero status code and writes a 404 response when
// the ego.server.allow.passkeys setting is false. Handlers call this first so
// that the ceremony endpoints are unreachable regardless of UI state.
func passkeyGuard(w http.ResponseWriter, session *Session) int {
	if !settings.GetBool(defs.WebAuthnAllowPasskeysSetting) {
		ui.Log(ui.AuthLogger, "auth.webauthn.disabled", ui.A{
			"session": session.ID})

		return util.ErrorResponse(w, session.ID, "passkeys not enabled", http.StatusNotFound)
	}

	return 0
}

// WebAuthnLoginBeginHandler starts the passkey authentication ceremony.
// POST /services/admin/webauthn/login/begin — no credentials required.
func WebAuthnLoginBeginHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	if status := passkeyGuard(w, session); status != 0 {
		return status
	}

	if status := webAuthnBeginGuard(w, session, r); status != 0 {
		return status
	}

	wau := webAuthnInstance(w, r, session.ID)
	if wau == nil {
		return http.StatusNotImplemented
	}

	options, sessionData, err := wau.BeginDiscoverableLogin()
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.begin.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	nonce, err := storeChallenge(sessionData)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	http.SetCookie(w, challengeCookie(nonce, 300))

	ui.Log(ui.AuthLogger, "auth.webauthn.login.begin", ui.A{
		"session": session.ID})

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)

	b, _ := json.Marshal(options)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return http.StatusOK
}

// WebAuthnLoginFinishHandler completes the passkey authentication ceremony and
// issues a bearer token on success.
// POST /services/admin/webauthn/login/finish — no credentials required.
func WebAuthnLoginFinishHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	if status := passkeyGuard(w, session); status != 0 {
		return status
	}

	wau := webAuthnInstance(w, r, session.ID)
	if wau == nil {
		return http.StatusNotImplemented
	}

	sessionData, err := loadChallenge(r)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.session.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Clear the challenge cookie regardless of outcome.
	http.SetCookie(w, challengeCookie("", -1))

	// userHandler is called by the library to resolve the user from the credential.
	var foundUser defs.User

	userHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		u, err := findUserByHandle(session.ID, userHandle)
		if err != nil {
			return nil, err
		}

		foundUser = u

		return auth.WebAuthnUserFrom(u), nil
	}

	credential, err := wau.FinishDiscoverableLogin(userHandler, *sessionData, r)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.login.fail", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, "passkey verification failed", http.StatusUnauthorized)
	}

	// Reject logins where the authenticator sign counter did not advance — a
	// strong indicator that the credential has been cloned.
	if credential.Authenticator.CloneWarning {
		ui.Log(ui.AuthLogger, "auth.webauthn.clone.warning", ui.A{
			"session": session.ID,
			"user":    foundUser.Name})

		return util.ErrorResponse(w, session.ID, "passkey verification failed", http.StatusUnauthorized)
	}

	// Verify the user has permission to log in.
	if auth.FindPermission(foundUser, defs.RootPermission) < 0 &&
		auth.FindPermission(foundUser, defs.LogonPermission) < 0 {
		ui.Log(ui.AuthLogger, "auth.webauthn.no.permission", ui.A{
			"session": session.ID,
			"user":    foundUser.Name})

		return util.ErrorResponse(w, session.ID, "account not permitted to log in", http.StatusForbidden)
	}

	// Persist the updated sign counter back to the user record.
	if err := updatePasskeyCounter(session.ID, foundUser, credential); err != nil {
		// Non-fatal: log and continue.
		ui.Log(ui.AuthLogger, "auth.webauthn.counter.error", ui.A{
			"session": session.ID,
			"error":   err})
	}

	ui.Log(ui.AuthLogger, "auth.webauthn.login.ok", ui.A{
		"session": session.ID,
		"user":    foundUser.Name})

	return issueToken(w, session, foundUser.Name)
}

// WebAuthnRegisterBeginHandler starts the passkey registration ceremony for an
// already-authenticated user.
// POST /services/admin/webauthn/register/begin — Bearer token required.
func WebAuthnRegisterBeginHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	if status := passkeyGuard(w, session); status != 0 {
		return status
	}

	if status := webAuthnBeginGuard(w, session, r); status != 0 {
		return status
	}

	wau := webAuthnInstance(w, r, session.ID)
	if wau == nil {
		return http.StatusNotImplemented
	}

	u, err := auth.AuthService.ReadUser(session.ID, session.User, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	options, sessionData, err := wau.BeginRegistration(auth.WebAuthnUserFrom(u))
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.begin.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	nonce, err := storeChallenge(sessionData)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	http.SetCookie(w, challengeCookie(nonce, 300))

	ui.Log(ui.AuthLogger, "auth.webauthn.register.begin", ui.A{
		"session": session.ID,
		"user":    session.User})

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)

	b := util.WriteJSON(w, options, &session.ResponseLength)
	ui.Log(ui.AuthLogger, "auth.webauth.options", ui.A{
		"session": session.ID,
		"body":    string(b),
	})

	return http.StatusOK
}

// WebAuthnRegisterFinishHandler completes the passkey registration ceremony and
// appends the new credential to the user record.
// POST /services/admin/webauthn/register/finish — Bearer token required.
func WebAuthnRegisterFinishHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	if status := passkeyGuard(w, session); status != 0 {
		return status
	}

	wau := webAuthnInstance(w, r, session.ID)
	if wau == nil {
		return http.StatusNotImplemented
	}

	u, err := auth.AuthService.ReadUser(session.ID, session.User, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	sessionData, err := loadChallenge(r)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.session.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Clear the challenge cookie regardless of outcome.
	http.SetCookie(w, challengeCookie("", -1))

	credential, err := wau.FinishRegistration(auth.WebAuthnUserFrom(u), *sessionData, r)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.webauthn.register.fail", ui.A{
			"session": session.ID,
			"user":    session.User,
			"error":   err})

		return util.ErrorResponse(w, session.ID, "passkey registration failed: "+err.Error(), http.StatusBadRequest)
	}

	// Append the new credential to the existing set and persist.
	existing := auth.WebAuthnUserFrom(u).WebAuthnCredentials()
	existing = append(existing, *credential)

	raw, err := auth.MarshalCredentials(existing)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	u.Passkeys = raw

	if err := auth.AuthService.WriteUser(session.ID, u); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_ = auth.AuthService.Flush()

	ui.Log(ui.AuthLogger, "auth.webauthn.register.ok", ui.A{
		"session": session.ID,
		"user":    session.User})

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(http.StatusOK)

	msg := `{"status":200,"msg":"passkey registered"}`
	_, _ = w.Write([]byte(msg))
	session.ResponseLength += len(msg)

	return http.StatusOK
}

// WebAuthnClearPasskeysHandler removes all stored passkeys for a user.
// DELETE /services/admin/webauthn/passkeys/{name} — Bearer token required.
// The caller must be the named user or have admin (root) permission.
func WebAuthnClearPasskeysHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	if status := passkeyGuard(w, session); status != 0 {
		return status
	}

	name := data.String(session.URLParts["name"])

	// Allow own account or admin.
	if !strings.EqualFold(name, session.User) {
		perms := auth.GetPermissions(session.ID, session.User)
		if !util.InListInsensitive(defs.RootPermission, perms...) {
			return util.ErrorResponse(w, session.ID, errors.ErrNoPermission.Error(), http.StatusForbidden)
		}
	}

	u, err := auth.AuthService.ReadUser(session.ID, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, "no such user: "+name, http.StatusNotFound)
	}

	u.Passkeys = nil

	if err := auth.AuthService.WriteUser(session.ID, u); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_ = auth.AuthService.Flush()

	ui.Log(ui.AuthLogger, "auth.webauthn.passkeys.cleared", ui.A{
		"session": session.ID,
		"user":    name})

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(http.StatusOK)

	msg := `{"status":200,"msg":"passkeys cleared"}`
	_, _ = w.Write([]byte(msg))
	session.ResponseLength += len(msg)

	return http.StatusOK
}

// updatePasskeyCounter persists the updated authenticator sign counter back into
// the user record after a successful login. This prevents replay attacks.
func updatePasskeyCounter(sessionID int, u defs.User, used *webauthn.Credential) error {
	creds := auth.WebAuthnUserFrom(u).WebAuthnCredentials()

	for i, c := range creds {
		if fmt.Sprintf("%x", c.ID) == fmt.Sprintf("%x", used.ID) {
			creds[i].Authenticator.SignCount = used.Authenticator.SignCount

			break
		}
	}

	raw, err := auth.MarshalCredentials(creds)
	if err != nil {
		return err
	}

	u.Passkeys = raw

	if err := auth.AuthService.WriteUser(sessionID, u); err != nil {
		return err
	}

	return auth.AuthService.Flush()
}

// // webauthnNotConfiguredCheck returns true (and writes a 501) when no RPID is set.
// func webauthnNotConfiguredCheck(w http.ResponseWriter, sessionID int) bool {
// 	if settings.Get(defs.WebAuthnRPIDSetting) == "" {
// 		http.Error(w, "WebAuthn not configured (ego.server.webauthn.rpid not set)", http.StatusNotImplemented)

// 		return true
// 	}

// 	return false
// }
