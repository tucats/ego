package rest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// resetTLSConfiguration clears the cached TLS configuration, so the next call
// to GetTLSConfiguration builds a new one from the current environment and
// settings, and restores both the cache and the settings when the test ends.
func resetTLSConfiguration(t *testing.T) {
	t.Helper()

	saved := tlsConfiguration
	savedCert := settings.Get(defs.RestClientServerCert)

	tlsConfiguration = nil

	t.Cleanup(func() {
		tlsConfiguration = saved

		settings.Set(defs.RestClientServerCert, savedCert)
	})
}

// With --insecure/-k in effect, verification must be skipped even when no
// server certificate file is configured. It used to be honored only when
// one was, so -k was otherwise silently ignored.
func TestGetTLSConfiguration_InsecureWithoutCertSetting(t *testing.T) {
	resetTLSConfiguration(t)
	t.Setenv(defs.EgoInsecureClientEnv, defs.True)
	settings.Set(defs.RestClientServerCert, "")

	config, err := GetTLSConfiguration()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !config.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify with EGO_INSECURE_CLIENT set and no cert setting")
	}
}

// The insecure option also wins over a configured certificate file.
func TestGetTLSConfiguration_InsecureWithCertSetting(t *testing.T) {
	resetTLSConfiguration(t)
	t.Setenv(defs.EgoInsecureClientEnv, defs.True)
	settings.Set(defs.RestClientServerCert, filepath.Join(t.TempDir(), "missing.crt"))

	config, err := GetTLSConfiguration()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !config.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify with EGO_INSECURE_CLIENT set and a cert setting")
	}
}

// Without the insecure option, the server is verified.
func TestGetTLSConfiguration_SecureByDefault(t *testing.T) {
	resetTLSConfiguration(t)
	t.Setenv(defs.EgoInsecureClientEnv, "")
	settings.Set(defs.RestClientServerCert, "")

	config, err := GetTLSConfiguration()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.InsecureSkipVerify {
		t.Error("expected server verification when EGO_INSECURE_CLIENT is not set")
	}
}

// A certificate file that cannot be parsed is an error, and it must not leave
// the configuration mutex locked. That early return used to skip the unlock,
// so the next call deadlocked.
func TestGetTLSConfiguration_ParseErrorReleasesLock(t *testing.T) {
	resetTLSConfiguration(t)
	t.Setenv(defs.EgoInsecureClientEnv, "")

	badCert := filepath.Join(t.TempDir(), "bad.crt")
	if err := os.WriteFile(badCert, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}

	settings.Set(defs.RestClientServerCert, badCert)

	if _, err := GetTLSConfiguration(); err == nil {
		t.Fatal("expected a certificate parse error")
	}

	if !tlsConfigurationMutex.TryLock() {
		t.Fatal("GetTLSConfiguration returned with the configuration mutex still locked")
	}

	tlsConfigurationMutex.Unlock()
}
