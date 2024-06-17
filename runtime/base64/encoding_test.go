package base64

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

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
	args = data.NewList(expected)
	decoded, err := decode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoded.(string) != "a" {
		t.Errorf("Expected 'a', but got '%s'", decoded.(string))
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
	args = data.NewList(expected)
	decoded, err := decode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoded.(string) != "" {
		t.Errorf("Expected '', but got '%s'", decoded.(string))
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
	args = data.NewList(expected)
	decoded, err := decode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoded.(string) != "This is a very long string that should be encoded" {
		t.Errorf("Expected 'This is a very long string that should be encoded', but got '%s'", decoded.(string))
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
	args = data.NewList(expected)
	decoded, err := decode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoded.(string) != "!@#$%^&*()" {
		t.Errorf("Expected '%s', but got '%s'", "!@#$%^&*()", decoded.(string))
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
	args = data.NewList(expected)
	decoded, err := decode(s, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoded.(string) != "こんにちは世界" {
		t.Errorf("Expected '%s', but got '%s'", "こんにちは世界", decoded.(string))
	}

}
