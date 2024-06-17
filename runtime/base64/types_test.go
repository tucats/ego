package base64

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestInitialize(t *testing.T) {
	// Test case 1: Initialize with a new symbol table
	s := symbols.NewRootSymbolTable("types test")
	Initialize(s)
	pkg, _ := s.Root().Get("base64")
	if pkg == nil {
		t.Error("Expected 'base64' package to be present in the symbol table")
	}

	p := pkg.(*data.Package)

	// Test case 2: Check if the 'Decode' function is present
	decodeFn, ok := p.Get("Decode")
	if !ok {
		t.Error("Expected 'Decode' function to be present in the 'base64' package")
	}
	decodeDecl := decodeFn.(data.Function).Declaration
	if !ok {
		t.Error("Expected 'Decode' function to have a declaration")
	}
	if decodeDecl.Name != "Decode" ||
		len(decodeDecl.Parameters) != 1 ||
		decodeDecl.Parameters[0].Name != "data" ||
		decodeDecl.Parameters[0].Type != data.StringType ||
		len(decodeDecl.Returns) != 1 ||
		decodeDecl.Returns[0] != data.StringType ||
		decodeDecl.ArgCount[0] != 1 ||
		decodeDecl.ArgCount[1] != 1 {
		t.Error("Unexpected 'Decode' function declaration")
	}

	// Test case 3: Check if the 'Encode' function is present
	encodeFn, ok := p.Get("Encode")
	if !ok {
		t.Error("Expected 'Encode' function to be present in the 'base64' package")
	}
	encodeDecl := encodeFn.(data.Function).Declaration
	if !ok {
		t.Error("Expected 'Encode' function to have a declaration")
	}
	if encodeDecl.Name != "Encode" ||
		len(encodeDecl.Parameters) != 1 ||
		encodeDecl.Parameters[0].Name != "data" ||
		encodeDecl.Parameters[0].Type != data.StringType ||
		len(encodeDecl.Returns) != 1 ||
		encodeDecl.Returns[0] != data.StringType ||
		encodeDecl.ArgCount[0] != 1 ||
		encodeDecl.ArgCount[1] != 1 {
		t.Error("Unexpected 'Encode' function declaration")
	}

}
