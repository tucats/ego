package rest

import (
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"gopkg.in/resty.v1"
)

// unwrapClient extracts the *data.Struct and its underlying *resty.Client
// from a New() result, which (per the runtime's (value, error) convention
// for fallible functions) is a data.List of (client, error) rather than the
// bare struct.
func unwrapClient(t *testing.T, result any) (*data.Struct, *resty.Client) {
	t.Helper()

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("New() returned %T, want data.List", result)
	}

	r, ok := list.Get(0).(*data.Struct)
	if !ok {
		t.Fatalf("New() list[0] is %T, want *data.Struct", list.Get(0))
	}

	client, ok := r.GetAlways(clientFieldName).(*resty.Client)
	if !ok {
		t.Fatalf("client field is %T, want *resty.Client", r.GetAlways(clientFieldName))
	}

	return r, client
}

// Regression tests for a security fix: New() used to attach the server's own
// logon token to any client created with no explicit credentials, even when
// called from a running Ego program. Since a rest.Client's Base() can point
// anywhere, this let any Ego script -- trusted or not -- exfiltrate a valid
// bearer token to an arbitrary third party simply by creating a client with
// no arguments and pointing it elsewhere. The token is now only ever
// attached automatically when isUserCodeRunning() is false (i.e. from the
// CLI's own native commands, never from an executing Ego program); a script
// that needs the ambient token must opt in explicitly with UseToken(true).
// UseToken() itself additionally refuses to run at all when the current
// execution context is sandboxed.

// Test_isUserCodeRunning reports the flag correctly in both states, saving
// and restoring the shared root symbol table's value around the test.
func Test_isUserCodeRunning(t *testing.T) {
	orig, hadOrig := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable)

	defer func() {
		if hadOrig {
			symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, orig)
		} else {
			_ = symbols.RootSymbolTable.Delete(defs.UserCodeRunningVariable, true)
		}
	}()

	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, true)

	if !isUserCodeRunning() {
		t.Error("expected isUserCodeRunning() to be true")
	}

	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, false)

	if isUserCodeRunning() {
		t.Error("expected isUserCodeRunning() to be false")
	}
}

// Test_New_TokenAttachmentRespectsUserCodeRunning verifies that New(),
// called with no arguments, only attaches the stored logon token when
// isUserCodeRunning() is false.
func Test_New_TokenAttachmentRespectsUserCodeRunning(t *testing.T) {
	const testToken = "test-token-value-12345"

	origToken := settings.Get(defs.LogonTokenSetting)
	origRunning, hadOrigRunning := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable)

	defer func() {
		settings.SetDefault(defs.LogonTokenSetting, origToken)

		if hadOrigRunning {
			symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, origRunning)
		} else {
			_ = symbols.RootSymbolTable.Delete(defs.UserCodeRunningVariable, true)
		}
	}()

	settings.SetDefault(defs.LogonTokenSetting, testToken)

	st := symbols.NewSymbolTable("test")

	// While an Ego program is "running", the token must NOT be attached.
	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, true)

	result, err := New(st, data.NewList())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_, client := unwrapClient(t, result)
	if client.Token != "" {
		t.Errorf("expected no token attached while user code running, got %q", client.Token)
	}

	// Outside a running Ego program, the token IS attached automatically.
	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, false)

	result2, err := New(st, data.NewList())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_, client2 := unwrapClient(t, result2)
	if client2.Token != testToken {
		t.Errorf("expected token %q attached while not user code running, got %q", testToken, client2.Token)
	}
}

// Test_setUseToken_BlockedWhenSandboxed verifies that UseToken() refuses to
// run at all in a sandboxed context, regardless of the flag argument, before
// it ever touches the underlying client.
func Test_setUseToken_BlockedWhenSandboxed(t *testing.T) {
	root := symbols.NewRootSymbolTable("test root")
	local := symbols.NewChildSymbolTable("test local", root)
	local.SetAlways(defs.SandboxedIOSymbolName, true)

	_, err := setUseToken(local, data.NewList(true))
	if !errors.Equals(err, errors.ErrNoPrivilegeForOperation) {
		t.Fatalf("expected ErrNoPrivilegeForOperation, got %v", err)
	}
}

// Test_setUseToken_WorksWhenNotSandboxed verifies the normal
// attach/clear behavior of UseToken() outside a sandboxed context.
func Test_setUseToken_WorksWhenNotSandboxed(t *testing.T) {
	const testToken = "test-token-value-67890"

	origToken := settings.Get(defs.LogonTokenSetting)
	defer settings.SetDefault(defs.LogonTokenSetting, origToken)
	settings.SetDefault(defs.LogonTokenSetting, testToken)

	root := symbols.NewRootSymbolTable("test root")
	local := symbols.NewChildSymbolTable("test local", root)

	st := symbols.NewSymbolTable("test")

	result, err := New(st, data.NewList())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	structValue, client := unwrapClient(t, result)
	local.SetAlways(defs.ThisVariable, structValue)

	if _, err := setUseToken(local, data.NewList(true)); err != nil {
		t.Fatalf("UseToken(true) error: %v", err)
	}

	if client.Token != testToken {
		t.Errorf("expected token %q attached after UseToken(true), got %q", testToken, client.Token)
	}

	if _, err := setUseToken(local, data.NewList(false)); err != nil {
		t.Fatalf("UseToken(false) error: %v", err)
	}

	if client.Token != "" {
		t.Errorf("expected no token attached after UseToken(false), got %q", client.Token)
	}
}
