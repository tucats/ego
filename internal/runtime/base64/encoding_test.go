package base64

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// decodeString is a test helper that calls decode and unwraps the
// (string, error) data.List result it returns.
func decodeString(t *testing.T, s *symbols.SymbolTable, text string) (string, error) {
	t.Helper()

	result, err := decode(s, data.NewList(text))
	if err != nil {
		t.Fatalf("Unexpected error return from decode(): %v", err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("decode() did not return a data.List, got %T", result)
	}

	value := ""
	if list.Get(0) != nil {
		value = data.String(list.Get(0))
	}

	listErr, _ := list.Get(1).(error)

	return value, listErr
}

func TestEncodeShortString(t *testing.T) {
	s := &symbols.SymbolTable{}
	args := data.NewList("a")

	encoded, err := encode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "YQ=="

	if encoded.(string) != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, encoded.(string))
	}

	// Change it back and confirm it decoded correctly.
	decoded, err := decodeString(t, s, expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decoded != "a" {
		t.Errorf("Expected 'a', but got '%s'", decoded)
	}
}

func TestEncodeEmptyString(t *testing.T) {
	s := &symbols.SymbolTable{}
	args := data.NewList("")

	encoded, err := encode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := ""
	if encoded.(string) != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, encoded.(string))
	}

	// Change it back and confirm it decoded correctly.
	decoded, err := decodeString(t, s, expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decoded != "" {
		t.Errorf("Expected '', but got '%s'", decoded)
	}
}

func TestEncodeLongString(t *testing.T) {
	s := &symbols.SymbolTable{}
	args := data.NewList("This is a very long string that should be encoded")

	encoded, err := encode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "VGhpcyBpcyBhIHZlcnkgbG9uZyBzdHJpbmcgdGhhdCBzaG91bGQgYmUgZW5jb2RlZA=="
	if encoded.(string) != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, encoded.(string))
	}

	// Change it back and confirm it decoded correctly.
	decoded, err := decodeString(t, s, expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decoded != "This is a very long string that should be encoded" {
		t.Errorf("Expected 'This is a very long string that should be encoded', but got '%s'", decoded)
	}
}

func TestEncodeSpecialCharacters(t *testing.T) {
	s := &symbols.SymbolTable{}
	args := data.NewList("!@#$%^&*()")

	encoded, err := encode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "IUAjJCVeJiooKQ=="
	if encoded.(string) != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, encoded.(string))
	}

	// Change it back and confirm it decoded correctly.
	decoded, err := decodeString(t, s, expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decoded != "!@#$%^&*()" {
		t.Errorf("Expected '%s', but got '%s'", "!@#$%^&*()", decoded)
	}
}

func TestEncodeNonASCIICharacters(t *testing.T) {
	s := &symbols.SymbolTable{}
	args := data.NewList("こんにちは世界")

	encoded, err := encode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "44GT44KT44Gr44Gh44Gv5LiW55WM"
	if encoded.(string) != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, encoded.(string))
	}

	// Change it back and confirm it decoded correctly.
	decoded, err := decodeString(t, s, expected)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decoded != "こんにちは世界" {
		t.Errorf("Expected '%s', but got '%s'", "こんにちは世界", decoded)
	}
}

// TestDecodeInvalidInput is a regression test for BUG-49: Decode's declared
// signature is (string, error), and invalid Base64 input must produce a
// non-nil error in the second list slot rather than an uncatchable abort.
func TestDecodeInvalidInput(t *testing.T) {
	s := &symbols.SymbolTable{}

	decoded, err := decodeString(t, s, "not-valid-base64!!!")
	if err == nil {
		t.Fatalf("Expected a non-nil error for invalid input, got nil")
	}

	if decoded != "" {
		t.Errorf("Expected empty string result on error, got '%s'", decoded)
	}
}
