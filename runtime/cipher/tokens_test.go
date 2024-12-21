package cipher

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestTokens(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	// Generate a new token.
	token, err := newToken(s, data.NewList("user", "data"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if token == nil {
		t.Fatalf("Unexpected result: %v", token)
	}

	if _, ok := token.(string); !ok {
		t.Fatalf("Unexpected result: %v", token)
	}

	// Validate the token.
	valid, err := validate(s, data.NewList(token))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !data.BoolOrFalse(valid) {
		t.Fatalf("Unexpected result: %v", valid)
	}

	// Extract the data value from the token.
	extracted, err := extract(s, data.NewList(token))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if extracted == nil {
		t.Fatalf("Unexpected result: %v", extracted)
	}

	if tokenStruct, ok := extracted.(*data.Struct); !ok {
		t.Fatalf("Unexpected result: %v", extracted)
	} else {
		// Validate the user and data values.
		if user, ok := tokenStruct.Get("Name"); !ok {
			t.Fatalf("Unexpected result: %v", user)
		} else if user != "user" {
			t.Fatalf("Unexpected result: %v", user)
		}

		if data, ok := tokenStruct.Get("Data"); !ok {
			t.Fatalf("Unexpected result: %v", data)
		} else if data != "data" {
			t.Fatalf("Unexpected result: %v", data)
		}
	}
}
