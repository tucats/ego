package authserver

import (
	"crypto/rand"
	"crypto/subtle" // constant-time comparison for CSRF token (H3)
	"encoding/hex"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
)

const csrfCookieName = "ego_oauth_csrf"

// generateCSRFToken returns a 128-bit cryptographically random hex string
// used as the CSRF nonce for the login form.
func generateCSRFToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// loginFormTemplate is the HTML login form shown to users during the
// Authorization Code flow.  It uses Go's html/template package so that all
// substituted values are automatically HTML-escaped, preventing XSS injection
// through client-supplied query parameters.
//
// The form is self-contained (no external CSS or JS dependencies) so it works
// regardless of whether Ego's asset server is running.
var loginFormTemplate = template.Must(template.New("login").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign In — Ego OAuth2</title>
  <style>
    body { font-family: system-ui, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
    .card { background: #fff; border-radius: 8px; box-shadow: 0 2px 12px rgba(0,0,0,.15); padding: 2rem; width: 100%; max-width: 380px; }
    h1 { margin: 0 0 1.5rem; font-size: 1.4rem; color: #222; }
    label { display: block; margin-bottom: .25rem; font-size: .85rem; color: #555; }
    input[type=text], input[type=password] { width: 100%; padding: .55rem .75rem; border: 1px solid #ccc; border-radius: 4px; font-size: 1rem; box-sizing: border-box; margin-bottom: 1rem; }
    input[type=submit] { width: 100%; padding: .65rem; background: #1a73e8; color: #fff; border: none; border-radius: 4px; font-size: 1rem; cursor: pointer; }
    input[type=submit]:hover { background: #1558b0; }
    .err { color: #c62828; font-size: .85rem; margin-bottom: 1rem; }
    .hint { font-size: .8rem; color: #666; margin-top: 1rem; }
  </style>
</head>
<body>
<div class="card">
  <h1>Sign in to continue</h1>
  {{if .Error}}<p class="err">{{.Error}}</p>{{end}}
  <form method="POST" action="/oauth2/authorize">
    <input type="hidden" name="client_id"             value="{{.ClientID}}">
    <input type="hidden" name="redirect_uri"          value="{{.RedirectURI}}">
    <input type="hidden" name="scope"                 value="{{.Scope}}">
    <input type="hidden" name="state"                 value="{{.State}}">
    <input type="hidden" name="code_challenge"        value="{{.CodeChallenge}}">
    <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
    <input type="hidden" name="csrf_token"            value="{{.CSRFToken}}">
    <label for="username">Username</label>
    <input type="text" id="username" name="username" autocomplete="username" required autofocus>
    <label for="password">Password</label>
    <input type="password" id="password" name="password" autocomplete="current-password" required>
    <input type="submit" value="Sign in">
  </form>
  <p class="hint">Signing in authorizes <strong>{{.ClientID}}</strong> to access your account.</p>
</div>
</body>
</html>`))

// loginFormData is passed to loginFormTemplate for rendering.
type loginFormData struct {
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	CSRFToken           string // random nonce; also set as a cookie
	Error               string // non-empty when the previous login attempt failed
}

// AuthorizeGetHandler serves the login form for GET /oauth2/authorize.
//
// The handler validates the required query parameters (client_id, redirect_uri,
// response_type) and renders the login form.  If any parameter is invalid the
// handler returns an error directly (without redirecting) to avoid open redirect
// vulnerabilities.
func AuthorizeGetHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	if clientID == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.missing.client_id"), http.StatusBadRequest)
	}

	client := findClient(clientID)
	if client == nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if redirectURI == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.missing.redirect_uri"), http.StatusBadRequest)
	}

	if !clientAllowsRedirect(client, redirectURI) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.redirect"), http.StatusBadRequest)
	}

	if responseType == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.missing.response_type"), http.StatusBadRequest)
	}

	if responseType != "code" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.unsupported.response_type"), http.StatusBadRequest)
	}

	// M2: public clients must supply a code_challenge at the authorization
	// step, not just at the token-exchange step.
	//
	// Why enforce PKCE here rather than only at the token endpoint?
	//
	//   The token endpoint (handleAuthorizationCodeGrant) already enforces
	//   PKCE for public clients: if pending.CodeChallenge is empty it returns
	//   400.  But that rejection arrives after the user has already entered
	//   their credentials, seen a success page, and been redirected back —
	//   a confusing dead end.
	//
	//   Enforcing the requirement here, before the login form is ever shown,
	//   gives the client developer an immediate, unambiguous error at the
	//   authorization step where the mistake actually occurred.  It also
	//   closes a narrow window: without this gate, a public client can obtain
	//   a valid authorization code (storing it in the AS cache) even though
	//   that code can never be exchanged.
	//
	// How to detect a public client:
	//
	//   A client registered without a ClientSecretHash has no shared secret and
	//   is treated as a public client (RFC 6749 §2.1).  Public clients MUST use
	//   PKCE (RFC 9700 §2.1.1 / OAuth 2.0 Security BCP).  Confidential clients
	//   (non-empty ClientSecretHash) may omit PKCE; their shared secret provides
	//   equivalent proof of identity.
	if client.ClientSecretHash == "" && codeChallenge == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.pkce.required"), http.StatusBadRequest)
	}

	// Generate a CSRF nonce, embed it in the form, and set it as an HttpOnly
	// cookie. The POST handler validates that both values match before accepting
	// the form submission, preventing cross-site request forgery attacks.
	csrfToken, csrfErr := generateCSRFToken()
	if csrfErr != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.csrf.generate"), http.StatusInternalServerError)
	}

	// OAUTH-L1: set the Secure flag when the connection is HTTPS so that
	// browsers enforce the cookie only over encrypted transport.  We detect
	// HTTPS via router.IsSecureRequest, which checks both r.TLS (direct TLS)
	// and the X-Forwarded-Proto: https header (TLS terminated by a proxy).
	// On a plain-HTTP development server the flag is omitted so the form still
	// functions, matching the behavior of the WebAuthn challenge cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/oauth2/authorize",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   router.IsSecureRequest(r),
	})

	data := loginFormData{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		CSRFToken:           csrfToken,
	}

	w.Header().Set(defs.ContentTypeHeader, "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = loginFormTemplate.Execute(w, data)

	return http.StatusOK
}

// reRenderWithError re-renders the login form with the given error message and
// a freshly generated CSRF token.  The old CSRF cookie is replaced so that the
// user can immediately submit the form again after seeing the error.
//
// This helper is the sole exit path for all failure cases inside
// AuthorizePostHandler that need to keep the user on the login page.  Keeping
// the regeneration logic in one place avoids the OAUTH-H4 class of bugs where
// the re-rendered form has an empty csrf_token field and the next submission is
// permanently rejected.
//
// If CSRF token generation fails (an extreme edge case caused by the OS random
// source being exhausted), the handler returns 500 rather than rendering a form
// without a CSRF nonce.
func reRenderWithError(
	session *router.Session,
	w http.ResponseWriter,
	r *http.Request,
	clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod, errMsg string,
) int {
	// Generate a fresh CSRF nonce for the re-rendered form.  The original nonce
	// is already consumed by the failed POST, so we must issue a new one;
	// leaving CSRFToken empty would make every re-rendered form permanently
	// un-submittable (OAUTH-H4).
	newCSRF, csrfErr := generateCSRFToken()
	if csrfErr != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.csrf.generate"), http.StatusInternalServerError)
	}

	// Replace the CSRF cookie with the same Secure-flag logic as the initial
	// GET handler: set Secure when the connection is HTTPS, omit on plain HTTP
	// (OAUTH-L1).  Both the first-load cookie and every re-render cookie must
	// agree so that a user on HTTPS always gets Secure cookies throughout the
	// entire authorization flow.
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    newCSRF,
		Path:     "/oauth2/authorize",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   router.IsSecureRequest(r),
	})

	data := loginFormData{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		CSRFToken:           newCSRF,
		Error:               errMsg,
	}

	w.Header().Set(defs.ContentTypeHeader, "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = loginFormTemplate.Execute(w, data)

	return http.StatusOK
}

// AuthorizePostHandler processes the submitted login form for POST /oauth2/authorize.
//
// On success it generates an authorization code, stores it in the cache, and
// redirects the browser to redirect_uri with the code and state appended.
//
// On failure it calls reRenderWithError, which generates a fresh CSRF token
// before re-rendering the form.  This ensures that a user who mistypes their
// password can correct it and resubmit without having to restart the entire
// authorization flow (OAUTH-H4).
//
// Rate limiting (OAUTH-H1): before attempting credential validation the handler
// checks the per-username failure counter used by the native auth path.  A
// locked account receives the same lockout message that the browser-based form
// would show, and the failure is recorded so the counter stays accurate.
func AuthorizePostHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if err := r.ParseForm(); err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.form.parse"), http.StatusBadRequest)
	}

	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	scope := r.FormValue("scope")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")
	username := r.FormValue("username")
	password := r.FormValue("password")
	csrfFormToken := r.FormValue("csrf_token")

	// Validate the CSRF token: the form value must match the HttpOnly cookie
	// that was set when the login page was served.  A mismatch means the POST
	// did not originate from our login form (cross-site request forgery).
	//
	// Audit: constant-time comparison:
	//
	//   A plain string equality check (==) is NOT constant-time in Go. The
	//   runtime can short-circuit the comparison as soon as it finds the first
	//   differing byte, which means requests with a correct prefix return
	//   slightly faster than requests with a wrong first byte.  An attacker who
	//   can send thousands of forged POSTs and measure the server response time
	//   for each can use those timing differences to recover the CSRF nonce one
	//   byte at a time — a "timing oracle" attack.
	//
	//   subtle.ConstantTimeCompare always inspects every byte of both inputs
	//   before returning, so all comparisons take the same wall-clock time
	//   regardless of where the values first differ.  This removes the timing
	//   signal entirely.
	//
	//   The attack is harder to mount against an HTTP server than against a
	//   local process (network jitter adds noise), but it becomes practical with
	//   enough samples and statistical averaging.  The fix is a one-line change
	//   that costs nothing and closes the channel permanently.
	//
	//   subtle.ConstantTimeCompare returns 1 (not the Go bool true) when the
	//   byte slices are equal, so the condition reads: "if not equal → reject".
	csrfCookie, cookieErr := r.Cookie(csrfCookieName)
	if cookieErr != nil || csrfCookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(csrfFormToken)) != 1 {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.csrf.invalid"), http.StatusForbidden)
	}

	// Re-validate client and redirect_uri on every POST.  These fields come from
	// hidden form inputs and could be tampered with by a malicious user.
	if clientID == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.missing.client_id"), http.StatusBadRequest)
	}

	client := findClient(clientID)
	if client == nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsRedirect(client, redirectURI) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.redirect"), http.StatusBadRequest)
	}

	// OAUTH-H1: Check the per-username rate-limit counter before attempting any
	// credential validation.  The native auth path (router/auth.go) maintains
	// this counter; by sharing it here, a brute-force attack through the OAuth2
	// login form consumes the same budget as one through the native logon
	// endpoint, and vice versa.  The two paths therefore enforce a single,
	// consistent lockout policy regardless of which front door the attacker uses.
	if retryAfter := router.CheckRateLimit(username); retryAfter > 0 {
		ui.Log(ui.AuthLogger, "oauth.as.authorize.locked", ui.A{
			"session": session.ID,
			"client":  clientID,
			"user":    username,
			"seconds": retryAfter,
		})

		// Re-render the form with a lockout message so the user knows to wait.
		// We do not return 429 directly here because the authorization endpoint
		// is browser-facing and a JSON error response is unhelpful; an in-form
		// message is clearer UX.  The fresh CSRF token ensures the user can
		// retry once the lockout expires.
		return reRenderWithError(session, w, r,
			clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod,
			"Account temporarily locked. Please wait before trying again.")
	}

	// Validate the user's credentials against Ego's user database.
	if !validatePassword(session.ID, username, password) {
		// Record the failure so the rate limiter can enforce lockout once the
		// threshold is reached (OAUTH-H1).
		router.RecordFailure(session.ID, username)

		ui.Log(ui.ServerLogger, "oauth.as.authorize.denied", ui.A{
			"client": clientID,
			"user":   username,
			"reason": "invalid credentials",
		})

		// Re-render the form with a fresh CSRF token (OAUTH-H4) and an error
		// message.  The old CSRF nonce is replaced so the user can correct their
		// credentials and resubmit without restarting the authorization flow.
		return reRenderWithError(session, w, r,
			clientID, redirectURI, scope, state, codeChallenge, codeChallengeMethod,
			"Invalid username or password.")
	}

	// Successful authentication — clear the failure counter so a subsequent
	// mistake does not carry over the failures from the previous session
	// (OAUTH-H1, matching the RecordSuccess call in router/auth.go).
	router.RecordSuccess(username)

	// Split the raw scope string into individual tokens before any validation.
	// splitScope("openid ego:admin") returns ["openid", "ego:admin"].
	// An empty scope string produces a nil slice, which both checks below handle
	// correctly (zero-length slice means "no scopes requested").
	requestedScopes := splitScope(scope)

	// Audit enforce the client's registered scope allowlist.
	//
	// Why this check is necessary:
	//
	//   Every registered OAuth2 client has a Scopes list that declares the
	//   maximum set of scopes it is ever permitted to request.  This is an
	//   administrator-controlled policy: a client registered for
	//   ["openid","ego:read"] must never obtain "ego:admin" tokens even if the
	//   authenticated user has root permission.
	//
	//   Without this check, intersectScopes (which runs next) only compares
	//   against the user's Ego permissions.  That is correct from the user's
	//   perspective but misses the client dimension: a badly-behaved or
	//   compromised client could request elevated scopes and receive them when a
	//   privileged user logs in.
	//
	// Why we return 400 rather than re-rendering the form:
	//
	//   This is a client misconfiguration error, not a user mistake.  The scope
	//   field comes from a hidden form input populated by the original GET query
	//   parameter.  A legitimate client never sends a scope it was not
	//   registered for; if we see one, either the client registration is wrong
	//   or the hidden field was tampered with.  Re-rendering the login form
	//   would be confusing (the user cannot fix a registration bug) and
	//   pointless (the same scope would be submitted again).
	//
	// Note: clientAllowsScope returns true when requestedScopes is empty, so
	// there is no penalty for clients that omit the scope parameter.
	if len(requestedScopes) > 0 && !clientAllowsScope(client, requestedScopes) {
		ui.Log(ui.ServerLogger, "oauth.as.authorize.denied", ui.A{
			"client": clientID,
			"user":   username,
			"reason": "client requested scope(s) not in its registered allowlist",
		})

		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.scope"), http.StatusBadRequest)
	}

	// Compute the intersection of the allowlist-validated requested scopes and
	// what the authenticated user's Ego permissions entitle them to.  This is
	// the second gate: the client may only obtain scopes it is registered for
	// AND that the user actually has.
	grantedScopes := intersectScopes(requestedScopes, auth.GetPermissions(session.ID, username))

	// Generate and store the authorization code.
	code, err := generateCode()
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.code.generate"), http.StatusInternalServerError)
	}

	storeCode(code, PendingAuthorization{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scopes:              grantedScopes,
		Username:            username,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	})

	// Redirect the browser back to the client's redirect_uri with the code.
	redirectURL, _ := url.Parse(redirectURI)
	q := redirectURL.Query()
	q.Set("code", code)

	if state != "" {
		q.Set("state", state)
	}

	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)

	return http.StatusFound
}

// splitScope splits a space-delimited scope string into a slice of scope strings.
func splitScope(scope string) []string {
	if scope == "" {
		return nil
	}

	parts := strings.Fields(scope)

	return parts
}

// intersectScopes returns the subset of requestedScopes that are allowed given
// the user's Ego permission strings.  The mapping from Ego permissions to
// OAuth2 scopes is:
//
//	root    → ego:admin
//	tables  → ego:write
//	logon   → ego:read
//	code_run → ego:code
//
// All users with any of the above permissions also receive openid, profile, email.
func intersectScopes(requested []string, permissions []string) []string {
	// Build the set of scopes this user is entitled to based on their permissions.
	entitled := make(map[string]bool)
	entitled["openid"] = true
	entitled["profile"] = true
	entitled["email"] = true

	for _, p := range permissions {
		switch p {
		case defs.RootPermission:
			entitled["ego:admin"] = true
		case defs.TableWritePermission, defs.TableUpdatePermission, defs.TableDeletePermission, defs.TableAdminPermission:
			entitled["ego:write"] = true
		case defs.TableReadPermission:
			entitled["ego:read"] = true
		case defs.LogonPermission:
			entitled["ego:read"] = true
		case defs.CodeRunPermission:
			entitled["ego:code"] = true
		}
	}

	// Always include openid/profile/email unless the user has zero permissions.
	if len(permissions) == 0 {
		delete(entitled, "openid")
		delete(entitled, "profile")
		delete(entitled, "email")
	}

	// The granted set is the intersection of what was requested and what the user
	// is entitled to.
	var granted []string

	for _, s := range requested {
		if entitled[s] {
			granted = append(granted, s)
		}
	}

	return granted
}
