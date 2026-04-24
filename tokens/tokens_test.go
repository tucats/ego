package tokens_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	errs "github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokens"
)

// testKey is a fixed encryption key that matches the auto-generated key format
// (two UUID strings joined by "-", 73 characters total). Setting the
// EGO_SERVER_TOKEN_KEY environment variable before any test runs ensures that
// all encrypt and decrypt calls use this exact key, making the tests
// deterministic regardless of any persisted settings.
const testKey = "00000000-0000-0000-0000-000000000001-00000000-0000-0000-0000-000000000002"

// altKey is a second fixed key used in tests that verify encryption-key mismatches.
const altKey = "11111111-1111-1111-1111-111111111111-22222222-2222-2222-2222-222222222222"

// testUUID is a valid UUID string used as the instanceUUID argument to New().
// Using a fixed value keeps tests readable and avoids UUID-parse failures.
const testUUID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

// TestMain wires up the fixed encryption key before any test executes.
// Without this, the package would generate (and persist) a random key on the
// first call, which can produce non-deterministic results when the test binary
// is run repeatedly in different environments.
func TestMain(m *testing.M) {
	os.Setenv("EGO_SERVER_TOKEN_KEY", testKey)
	os.Exit(m.Run())
}

// =============================================================================
// Helper
// =============================================================================

// ensureNoDatabase sets the blacklist database path to "" (disabled). This is
// effective only when the handle has never been set; once SetDatabasePath has
// been called with a real path the handle is not cleared by a subsequent
// empty-path call (see documented design issue). Nil-handle tests therefore
// MUST run before any test that calls SetDatabasePath with a real path.
func ensureNoDatabase(t *testing.T) {
	t.Helper()

	if err := tokens.SetDatabasePath(""); err != nil {
		t.Fatalf("SetDatabasePath(\"\") error = %v", err)
	}
}

// =============================================================================
// tokens.New
// =============================================================================

func TestNew_Valid(t *testing.T) {
	tok, err := tokens.New("alice", "payload", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if tok == "" {
		t.Fatal("New() returned empty string")
	}
	// The returned string must consist entirely of hexadecimal characters.
	for _, c := range tok {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			t.Errorf("New() = %q: non-hex character %q", tok, c)

			break
		}
	}
}

func TestNew_EmptyInterval_DefaultsTo15m(t *testing.T) {
	// An empty interval string should be treated as "15m".
	tok, err := tokens.New("alice", "data", "", testUUID, 0)
	if err != nil {
		t.Fatalf("New() empty interval error = %v", err)
	}

	valid, e := tokens.Validate(tok, 0)
	if !valid || e != nil {
		t.Errorf("token with empty interval failed validation: valid=%v, err=%v", valid, e)
	}
}

func TestNew_InvalidUUID(t *testing.T) {
	_, err := tokens.New("alice", "data", "15m", "not-a-uuid", 0)
	if err == nil {
		t.Error("New() with invalid UUID: expected error, got nil")
	}
}

func TestNew_EmptyUUID(t *testing.T) {
	// An empty string is not a valid UUID.
	_, err := tokens.New("alice", "data", "15m", "", 0)
	if err == nil {
		t.Error("New() with empty UUID: expected error, got nil")
	}
}

func TestNew_InvalidInterval(t *testing.T) {
	_, err := tokens.New("alice", "data", "not-a-duration", testUUID, 0)
	if err == nil {
		t.Error("New() with invalid interval: expected error, got nil")
	}
}

func TestNew_ZeroInterval(t *testing.T) {
	// "0" is technically a valid duration (zero), producing an already-expired token.
	tok, err := tokens.New("alice", "data", "0", testUUID, 0)
	if err != nil {
		t.Fatalf("New() zero interval error = %v", err)
	}
	// Token expires immediately; validation should fail.
	valid, _ := tokens.Validate(tok, 0)
	if valid {
		t.Error("Validate() = true for zero-interval (immediately-expired) token, want false")
	}
}

func TestNew_EmptyName(t *testing.T) {
	// The Name field has no server-side validation; an empty name should succeed.
	tok, err := tokens.New("", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() with empty name error = %v", err)
	}

	if tok == "" {
		t.Error("New() with empty name returned empty token")
	}
}

func TestNew_EmptyData(t *testing.T) {
	tok, err := tokens.New("alice", "", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() with empty data error = %v", err)
	}

	if tok == "" {
		t.Error("New() with empty data returned empty token")
	}
}

func TestNew_LongInterval(t *testing.T) {
	tok, err := tokens.New("alice", "data", "8760h", testUUID, 0) // approximately one year
	if err != nil {
		t.Fatalf("New() with long interval error = %v", err)
	}

	valid, e := tokens.Validate(tok, 0)
	if !valid || e != nil {
		t.Errorf("long-interval token: valid=%v, err=%v; want (true, nil)", valid, e)
	}
}

func TestNew_UniqueTokenIDs(t *testing.T) {
	// Each call must generate a distinct TokenID even when all other parameters
	// are identical, because uuid.New() is called internally.
	tok1, err1 := tokens.New("alice", "data", "15m", testUUID, 0)
	tok2, err2 := tokens.New("alice", "data", "15m", testUUID, 0)

	if err1 != nil || err2 != nil {
		t.Fatalf("New() errors: %v, %v", err1, err2)
	}

	if tok1 == tok2 {
		t.Error("two consecutive New() calls produced identical tokens; TokenID must be unique")
	}
}

// =============================================================================
// tokens.Unwrap
// =============================================================================

func TestUnwrap_Valid(t *testing.T) {
	const name, data = "bob", "my-payload"

	tok, err := tokens.New(name, data, "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := tokens.Unwrap(tok, 0)
	if err != nil {
		t.Fatalf("Unwrap() error = %v", err)
	}

	if result == nil {
		t.Fatal("Unwrap() returned nil Token")
	}

	if result.Name != name {
		t.Errorf("Name = %q, want %q", result.Name, name)
	}

	if result.Data != data {
		t.Errorf("Data = %q, want %q", result.Data, data)
	}

	if result.TokenID == (uuid.UUID{}) {
		t.Error("TokenID is the zero UUID")
	}

	if result.Expires.IsZero() {
		t.Error("Expires is zero")
	}

	if result.Expires.Before(time.Now()) {
		t.Error("Expires is in the past for a freshly-created 15m token")
	}
}

func TestUnwrap_FieldsRoundtrip(t *testing.T) {
	// Verify that every field written by New() is faithfully recovered by Unwrap().
	name, data, authUUID := "carol", "specific-data", testUUID

	before := time.Now().Truncate(time.Second)

	tok, err := tokens.New(name, data, "30m", authUUID, 99)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	after := time.Now().Add(time.Second)

	result, err := tokens.Unwrap(tok, 99)
	if err != nil {
		t.Fatalf("Unwrap() error = %v", err)
	}

	if result.Name != name {
		t.Errorf("Name = %q, want %q", result.Name, name)
	}

	if result.Data != data {
		t.Errorf("Data = %q, want %q", result.Data, data)
	}

	if result.Created.Before(before) || result.Created.After(after) {
		t.Errorf("Created = %v, want in [%v, %v]", result.Created, before, after)
	}

	if result.AuthID.String() != authUUID {
		t.Errorf("AuthID = %v, want %v", result.AuthID, authUUID)
	}
}

func TestUnwrap_InvalidHex(t *testing.T) {
	_, err := tokens.Unwrap("not-hex-!!!!", 0)
	if err == nil {
		t.Error("Unwrap() with invalid hex: expected error, got nil")
	}
}

func TestUnwrap_OddLengthHex(t *testing.T) {
	// Hex strings must have an even number of characters.
	_, err := tokens.Unwrap("abc", 0)
	if err == nil {
		t.Error("Unwrap() with odd-length hex: expected error, got nil")
	}
}

func TestUnwrap_EmptyString(t *testing.T) {
	// An empty hex string decodes to zero bytes; decryption of zero bytes
	// either fails or produces an empty plaintext, both of which are rejected.
	_, err := tokens.Unwrap("", 0)
	if err == nil {
		t.Error("Unwrap() with empty string: expected error, got nil")
	}
}

func TestUnwrap_ValidHexGarbageContent(t *testing.T) {
	// Valid hex but not an encrypted token; decryption or JSON parsing must fail.
	_, err := tokens.Unwrap("DeadBeefCafe", 0)
	if err == nil {
		t.Error("Unwrap() with hex-encoded garbage: expected error, got nil")
	}
}

func TestUnwrap_TruncatedToken(t *testing.T) {
	tok, err := tokens.New("user", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Keep only the first half; ensure the result is still even-length hex.
	half := tok[:len(tok)/2]
	if len(half)%2 != 0 {
		half = half[:len(half)-1]
	}

	_, err = tokens.Unwrap(half, 0)
	if err == nil {
		t.Error("Unwrap() with truncated token: expected error, got nil")
	}
}

func TestUnwrap_ExpiredToken(t *testing.T) {
	tok, err := tokens.New("user", "data", "1ms", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	time.Sleep(20 * time.Millisecond) // ensure the 1 ms token has expired

	_, err = tokens.Unwrap(tok, 0)
	if err == nil {
		t.Error("Unwrap() of expired token: expected error, got nil")
	}
}

func TestUnwrap_WrongEncryptionKey(t *testing.T) {
	// Create the token with testKey, then try to unwrap with altKey.
	tok, err := tokens.New("user", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	os.Setenv("EGO_SERVER_TOKEN_KEY", altKey)
	defer os.Setenv("EGO_SERVER_TOKEN_KEY", testKey)

	_, err = tokens.Unwrap(tok, 0)
	if err == nil {
		t.Error("Unwrap() with wrong encryption key: expected error, got nil")
	}
}

// =============================================================================
// tokens.Validate
// =============================================================================

func TestValidate_ValidToken(t *testing.T) {
	tok, err := tokens.New("dave", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	valid, err := tokens.Validate(tok, 0)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !valid {
		t.Error("Validate() = false for valid token, want true")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	tok, err := tokens.New("user", "data", "1ms", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	valid, err := tokens.Validate(tok, 0)
	if valid {
		t.Error("Validate() = true for expired token, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil for expired token, want non-nil")
	}
}

// TestValidate_InvalidHex verifies that a non-hex token string returns
// (false, error). Previously, a dead reportErr flag caused this case to
// return (false, nil), silently swallowing the error.
func TestValidate_InvalidHex(t *testing.T) {
	valid, err := tokens.Validate("not-hex!!!!", 0)
	if valid {
		t.Error("Validate() = true for invalid hex, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil for invalid hex, want non-nil error")
	}
}

func TestValidate_EmptyString(t *testing.T) {
	// An empty string decodes as zero bytes (no hex error), then decryption
	// fails or produces empty plaintext → returns (false, error).
	valid, err := tokens.Validate("", 0)
	if valid {
		t.Error("Validate() = true for empty token string, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil for empty token (decryption of zero bytes should fail)")
	}
}

func TestValidate_ValidHexGarbageContent(t *testing.T) {
	valid, err := tokens.Validate("DeadBeefCafe", 0)
	if valid {
		t.Error("Validate() = true for hex garbage, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil for hex garbage, want non-nil")
	}
}

func TestValidate_TruncatedToken(t *testing.T) {
	tok, err := tokens.New("user", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	half := tok[:len(tok)/2]
	if len(half)%2 != 0 {
		half = half[:len(half)-1]
	}

	valid, err := tokens.Validate(half, 0)
	if valid {
		t.Error("Validate() = true for truncated token, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil for truncated token, want non-nil")
	}
}

func TestValidate_WrongEncryptionKey(t *testing.T) {
	tok, err := tokens.New("user", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	os.Setenv("EGO_SERVER_TOKEN_KEY", altKey)
	defer os.Setenv("EGO_SERVER_TOKEN_KEY", testKey)

	valid, err := tokens.Validate(tok, 0)
	if valid {
		t.Error("Validate() = true with wrong decryption key, want false")
	}

	if err == nil {
		t.Error("Validate() error = nil with wrong key, want non-nil")
	}
}

// =============================================================================
// Blacklist — nil handle (no database configured)
//
// IMPORTANT: These tests rely on the package-level handle being nil, which is
// only true before any call to SetDatabasePath with a non-empty path. They
// must run before the database tests below.
// =============================================================================

func TestSetDatabasePath_EmptyPath(t *testing.T) {
	err := tokens.SetDatabasePath("")
	if err != nil {
		t.Errorf("SetDatabasePath(\"\") error = %v, want nil", err)
	}
}

func TestBlacklist_NilHandle_IsNoop(t *testing.T) {
	ensureNoDatabase(t)

	err := tokens.Blacklist("some-token-id")
	if err != nil {
		t.Errorf("Blacklist() with no database error = %v, want nil", err)
	}
}

// TestDelete_NilHandle_ReturnsNotFound verifies that Delete() returns ErrNotFound
// when no database is configured. The entry cannot exist, so ErrNotFound is the
// correct and consistent response.
func TestDelete_NilHandle_ReturnsNotFound(t *testing.T) {
	ensureNoDatabase(t)

	err := tokens.Delete("nonexistent-id")
	if err == nil {
		t.Error("Delete() with no database: expected ErrNotFound, got nil")
	}

	if !errs.Equal(err, errs.ErrNotFound) {
		t.Errorf("Delete() with no database: error = %v, want ErrNotFound", err)
	}
}

func TestIsBlacklisted_NilHandle_ReturnsFalse(t *testing.T) {
	ensureNoDatabase(t)

	tok := tokens.Token{
		Name:    "user",
		TokenID: uuid.New(),
	}

	blacklisted, err := tokens.IsBlacklisted(tok)
	if err != nil {
		t.Errorf("IsBlacklisted() with no database error = %v, want nil", err)
	}

	if blacklisted {
		t.Error("IsBlacklisted() with no database = true, want false")
	}
}

func TestFlush_NilHandle_ReturnsZero(t *testing.T) {
	ensureNoDatabase(t)

	count, err := tokens.Flush()
	if err != nil {
		t.Errorf("Flush() with no database error = %v, want nil", err)
	}

	if count != 0 {
		t.Errorf("Flush() with no database count = %d, want 0", count)
	}
}

func TestList_NilHandle_ReturnsEmptySlice(t *testing.T) {
	ensureNoDatabase(t)

	items, err := tokens.List()
	if err != nil {
		t.Errorf("List() with no database error = %v, want nil", err)
	}

	if items == nil {
		t.Error("List() with no database returned nil, want empty slice")
	}

	if len(items) != 0 {
		t.Errorf("List() with no database len = %d, want 0", len(items))
	}
}

// =============================================================================
// Blacklist — with a real database (runs after nil-handle tests)
// =============================================================================

func TestBlacklist_WithDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "blacklist_test.db")

	if err := tokens.SetDatabasePath(dbPath); err != nil {
		t.Skipf("database setup failed (%v) — skipping database blacklist tests", err)
	}

	// When we're done, free up the blacklist database resource. The actual database
	// (a sqlite3 database in the temp directory) will remain, and will be cleaned
	// up when /tmp directory purging happens by the system.
	defer tokens.Close()

	tokenID := uuid.New()
	tok := tokens.Token{
		Name:    "testUser",
		TokenID: tokenID,
		Created: time.Now(),
	}

	t.Run("IsBlacklisted_before_entry", func(t *testing.T) {
		blacklisted, err := tokens.IsBlacklisted(tok)
		if err != nil {
			t.Errorf("IsBlacklisted() error = %v, want nil", err)
		}

		if blacklisted {
			t.Error("IsBlacklisted() = true before blacklisting, want false")
		}
	})

	t.Run("Blacklist_adds_entry", func(t *testing.T) {
		if err := tokens.Blacklist(tokenID.String()); err != nil {
			t.Fatalf("Blacklist() error = %v", err)
		}
	})

	t.Run("IsBlacklisted_after_entry", func(t *testing.T) {
		blacklisted, err := tokens.IsBlacklisted(tok)
		if err != nil {
			t.Errorf("IsBlacklisted() error = %v", err)
		}

		if !blacklisted {
			t.Error("IsBlacklisted() = false after blacklisting, want true")
		}
	})

	t.Run("IsBlacklisted_cache_hit", func(t *testing.T) {
		// Second call hits the cache populated by the previous IsBlacklisted call.
		blacklisted, err := tokens.IsBlacklisted(tok)
		if err != nil {
			t.Errorf("IsBlacklisted() cache hit error = %v", err)
		}

		if !blacklisted {
			t.Error("IsBlacklisted() cache hit = false, want true")
		}
	})

	t.Run("List_contains_entry", func(t *testing.T) {
		items, err := tokens.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		found := false

		for _, item := range items {
			if item.ID == tokenID.String() {
				found = true

				if !item.Active {
					t.Error("blacklisted item has Active=false, want true")
				}

				break
			}
		}

		if !found {
			t.Errorf("List() did not include blacklisted token %v", tokenID)
		}
	})

	t.Run("Delete_removes_entry", func(t *testing.T) {
		if err := tokens.Delete(tokenID.String()); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// After deletion the token should no longer be blacklisted.
		blacklisted, err := tokens.IsBlacklisted(tok)
		if err != nil {
			t.Errorf("IsBlacklisted() after Delete() error = %v", err)
		}

		if blacklisted {
			t.Error("IsBlacklisted() = true after Delete(), want false")
		}
	})

	t.Run("Delete_nonexistent_returns_ErrNotFound", func(t *testing.T) {
		err := tokens.Delete("id-that-was-never-blacklisted")
		if err == nil {
			t.Error("Delete() of nonexistent ID: expected error, got nil")
		}

		if !errs.Equal(err, errs.ErrNotFound) {
			t.Errorf("Delete() of nonexistent ID: error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Flush_clears_all_entries", func(t *testing.T) {
		// Add two more entries.
		id1, id2 := uuid.New().String(), uuid.New().String()
		if err := tokens.Blacklist(id1); err != nil {
			t.Fatalf("Blacklist(id1) error = %v", err)
		}

		if err := tokens.Blacklist(id2); err != nil {
			t.Fatalf("Blacklist(id2) error = %v", err)
		}

		count, err := tokens.Flush()
		if err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		if count == 0 {
			t.Error("Flush() count = 0, want > 0")
		}

		items, err := tokens.List()
		if err != nil {
			t.Fatalf("List() after Flush() error = %v", err)
		}

		if len(items) != 0 {
			t.Errorf("List() after Flush() len = %d, want 0", len(items))
		}
	})
}

// =============================================================================
// Full token lifecycle: New → Validate → Blacklist → Validate
// =============================================================================

// TestLifecycle_BlacklistInvalidatesToken creates a token, confirms it is valid,
// blacklists it, and then confirms validation fails. This requires a real
// database and is skipped if one cannot be set up.
func TestLifecycle_BlacklistInvalidatesToken(t *testing.T) {
	dbPath := "sqlite3://" + filepath.Join(t.TempDir(), "lifecycle_test.db")
	if err := tokens.SetDatabasePath(dbPath); err != nil {
		t.Skipf("database setup failed (%v) — skipping lifecycle test", err)
	}

	tok, err := tokens.New("user", "data", "15m", testUUID, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Fresh token must validate.
	valid, err := tokens.Validate(tok, 0)
	if !valid || err != nil {
		t.Fatalf("Validate() before blacklisting: valid=%v, err=%v; want (true, nil)", valid, err)
	}

	// Unwrap to obtain the TokenID.
	result, err := tokens.Unwrap(tok, 0)
	if err != nil {
		t.Fatalf("Unwrap() error = %v", err)
	}

	// Blacklist the token by its ID.
	if err = tokens.Blacklist(result.TokenID.String()); err != nil {
		t.Fatalf("Blacklist() error = %v", err)
	}

	// The same token string must now be rejected by both Validate and Unwrap.
	valid, err = tokens.Validate(tok, 0)
	if valid {
		t.Error("Validate() = true after blacklisting, want false")
	}

	if !errs.Equal(err, errs.ErrBlacklisted) {
		t.Errorf("Validate() after blacklisting: error = %v, want ErrBlacklisted", err)
	}

	_, err = tokens.Unwrap(tok, 0)
	if !errs.Equal(err, errs.ErrBlacklisted) {
		t.Errorf("Unwrap() after blacklisting: error = %v, want ErrBlacklisted", err)
	}
}
