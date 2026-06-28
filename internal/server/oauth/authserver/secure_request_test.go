// Package authserver — secure_request_test.go verifies that router.IsSecureRequest
// is accessible from the authserver package (i.e., that it was correctly exported
// from the router package as part of the OAUTH-L1 fix).
//
// Before the OAUTH-L1 fix the helper was named isSecureRequest (unexported) and
// could only be called from within the router package.  Renaming it to
// IsSecureRequest made it available to authserver and any other package that
// needs to set the Secure cookie attribute consistently.
package authserver

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/router"
)

// TestIsSecureRequest_ExportedAndCallable verifies that router.IsSecureRequest
// can be called from outside the router package (OAUTH-L1 prerequisite).
//
// This test is intentionally simple: if the function were still unexported, the
// file would fail to compile, catching the regression immediately.
func TestIsSecureRequest_ExportedAndCallable(t *testing.T) {
	// Plain HTTP request — r.TLS is nil, no X-Forwarded-Proto header.
	plainReq := httptest.NewRequest(http.MethodGet, "/", nil)

	if router.IsSecureRequest(plainReq) {
		t.Error("IsSecureRequest should return false for a plain-HTTP request")
	}

	// HTTPS request — r.TLS is non-nil.
	tlsReq := httptest.NewRequest(http.MethodGet, "/", nil)
	tlsReq.TLS = &tls.ConnectionState{}

	if !router.IsSecureRequest(tlsReq) {
		t.Error("IsSecureRequest should return true when r.TLS is non-nil")
	}

	// Proxy-forwarded HTTPS — r.TLS is nil but X-Forwarded-Proto says https.
	proxyReq := httptest.NewRequest(http.MethodGet, "/", nil)
	proxyReq.Header.Set("X-Forwarded-Proto", "https")

	if !router.IsSecureRequest(proxyReq) {
		t.Error("IsSecureRequest should return true when X-Forwarded-Proto: https")
	}
}
