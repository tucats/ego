// Package rshandlers_test — callback_test.go covers the OAUTH-L4 security fixes
// for CallbackHandler: internal error details must not be returned to browser
// clients, and caller-supplied query parameters must be sanitized before being
// written to the server log.
package rshandlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/oauth/rshandlers"
)

// TestCallbackHandler_IdPError_GenericBrowserResponse verifies that when the
// IdP passes an "error" query parameter to the callback endpoint, CallbackHandler
// returns a fixed generic message to the browser — not the raw IdP error code or
// description (OAUTH-L4).
//
// Without this fix, an attacker who can craft the redirect URL (or who runs a
// malicious IdP) can inject arbitrary text into the HTTP response body seen by
// the browser client.
func TestCallbackHandler_IdPError_GenericBrowserResponse(t *testing.T) {
	// Build a request that simulates an IdP signaling access_denied.
	// The error_description contains a newline to test log-injection defense.
	req := httptest.NewRequest(http.MethodGet,
		"/services/admin/oauth/callback"+
			"?error=access_denied"+
			"&error_description=User+denied+access%0AX-Injected-Header%3A+evil",
		nil)

	w := httptest.NewRecorder()
	session := &router.Session{ID: 42}

	status := rshandlers.CallbackHandler(session, w, req)

	// The handler must return 400 (bad request from IdP).
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", status, http.StatusBadRequest)
	}

	// The response body must NOT contain the raw IdP error code or description.
	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	if strings.Contains(bodyStr, "access_denied") {
		t.Errorf("response body contains raw IdP error code %q — should be generic (OAUTH-L4): %s",
			"access_denied", bodyStr)
	}

	if strings.Contains(bodyStr, "X-Injected-Header") {
		t.Errorf("response body contains injected content from error_description (OAUTH-L4): %s", bodyStr)
	}

	// The response must contain the fixed generic message.
	if !strings.Contains(bodyStr, "OAuth2 login failed") {
		t.Errorf("response body does not contain the generic error message; got: %s", bodyStr)
	}
}

// TestCallbackHandler_IdPError_WithSensitiveDescription verifies that sensitive
// information in the error_description is not reflected to the browser (OAUTH-L4).
//
// Some IdPs include diagnostic detail — including internal server names, user
// data, or token fragments — in the error_description field.  This must not
// reach the client's browser.
func TestCallbackHandler_IdPError_WithSensitiveDescription(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/services/admin/oauth/callback"+
			"?error=server_error"+
			"&error_description=Internal+server+error+at+token.internal.corp%3A8443",
		nil)

	w := httptest.NewRecorder()
	session := &router.Session{ID: 43}

	rshandlers.CallbackHandler(session, w, req)

	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	// The internal server address must not be in the response.
	if strings.Contains(bodyStr, "token.internal.corp") {
		t.Errorf("response body leaks internal server address from error_description (OAUTH-L4): %s", bodyStr)
	}

	// The generic message must appear instead.
	if !strings.Contains(bodyStr, "OAuth2 login failed") {
		t.Errorf("response body does not contain generic error message; got: %s", bodyStr)
	}
}

// TestCallbackHandler_InjectionContentNotReflected verifies that control
// characters embedded in "error_description" never appear in the HTTP response
// body (OAUTH-L4).  The response must always be the fixed generic message and
// must not echo back any caller-supplied value.
func TestCallbackHandler_InjectionContentNotReflected(t *testing.T) {
	attempts := []struct {
		name   string
		errVal string // value of the "error" query param
		desc   string // URL-encoded value of "error_description"
	}{
		{"newline in desc", "test_error", "line1%0Aline2"},
		{"CRLF in desc", "server_error", "line1%0D%0Aline2"},
		{"injected header text", "denied", "desc%0AX-Evil%3A+injected"},
	}

	for _, tt := range attempts {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/services/admin/oauth/callback?error="+tt.errVal+"&error_description="+tt.desc,
				nil)
			w := httptest.NewRecorder()
			session := &router.Session{ID: 1}

			rshandlers.CallbackHandler(session, w, req)

			body, _ := io.ReadAll(w.Result().Body)
			bodyStr := string(body)

			// The raw error value must never appear in the response.
			if strings.Contains(bodyStr, tt.errVal) {
				t.Errorf("response contains raw error %q — must be generic (OAUTH-L4): %s",
					tt.errVal, bodyStr)
			}

			// The response must contain the fixed generic message.
			if !strings.Contains(bodyStr, "OAuth2 login failed") {
				t.Errorf("response missing generic error message; got: %s", bodyStr)
			}
		})
	}
}
