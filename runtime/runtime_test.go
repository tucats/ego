package runtime

import (
	"testing"

	"github.com/tucats/ego/data"
)

func TestTypeCompiler_SimpleType(t *testing.T) {
	typeString := "int"
	expectedType := data.NewTypeInstance(data.IntKind)

	result := TypeCompiler(typeString)
	if result.String() != expectedType.String() {
		t.Errorf("Expected type name %s, but got %s", expectedType.String(), result.String())
	}
}

func TestTypeCompiler_StructType(t *testing.T) {
	typeString := `struct {
        ID   int
        Name string
    }`
	expectedType := data.NewTypeInstance(data.StructKind).
		DefineField("ID", data.IntType).
		DefineField("Name", data.StringType)

	result := TypeCompiler(typeString)
	if result.String() != expectedType.String() {
		t.Errorf("Expected type %v, but got %v", expectedType, result)
	}
}

func TestTypeCompiler_PointerType(t *testing.T) {
	typeString := "*int"
	expectedType := data.NewPointerTypeInstance(data.IntType)

	result := TypeCompiler(typeString)
	if result.String() != expectedType.String() {
		t.Errorf("Expected type %v, but got %v", expectedType, result)
	}
}
