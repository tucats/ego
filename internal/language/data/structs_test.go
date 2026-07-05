package data

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

func TestNewStruct(t *testing.T) {
	type fields struct {
		typeDef  *Type
		static   bool
		fields   map[string]any
		typeName string
	}

	type args struct {
		t *Type
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Struct
	}{
		{
			name: "test with empty struct",
			fields: fields{
				typeDef: &Type{
					kind:   StructKind,
					fields: map[string]*Type{},
				},
				static:   false,
				fields:   map[string]any{},
				typeName: "",
			},
			args: args{
				t: &Type{
					kind:   StructKind,
					fields: map[string]*Type{},
				},
			},
			want: &Struct{
				typeDef:  &Type{kind: StructKind, fields: map[string]*Type{}},
				static:   false,
				fields:   map[string]any{},
				typeName: "",
			},
		},
		{
			name: "test with struct with fields",
			fields: fields{
				typeDef: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
				static:   true,
				fields:   map[string]any{"field1": 0, "field2": ""},
				typeName: "",
			},
			args: args{
				t: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
			},
			want: &Struct{
				typeDef: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
				static:   true,
				fields:   map[string]any{"field1": 0, "field2": ""},
				typeName: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStruct(tt.args.t); !got.DeepEqual(tt.want) {
				t.Errorf("NewStruct(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// newNamedEmployeeType builds a *Type that mimics what the compiler produces
// for the Ego declaration:
//
//	type Employee struct {
//	    Age int
//	}
//
// This is a "named type" -- a TypeKind wrapper (kind: TypeKind) whose
// valueType points at the real StructKind type that carries the field
// definitions (see data.TypeDefinition and typeCompiler.typeCompiler in the
// compiler package). The wrapper itself has a nil `fields` map; only the
// wrapped StructKind type's `fields` map is populated.
//
// BUG-33 was caused by code that read the wrapper's own (always nil) fields
// map instead of unwrapping to the base type first, so field-type
// declarations on named struct types were never enforced. These tests
// exercise data.Struct.Set() directly against a struct built from exactly
// this shape.
func newNamedEmployeeType() *Type {
	structKind := &Type{
		kind: StructKind,
		fields: map[string]*Type{
			"Age": IntType,
		},
	}

	return &Type{
		kind:      TypeKind,
		name:      "Employee",
		valueType: structKind,
	}
}

// TestStructSet_NamedTypeCoercion is the core BUG-33 regression test for
// dynamic (default) mode: assigning a coercible value of a different Go type
// ("42", a string) to a field declared as int on a struct built from a named
// type must succeed and convert the value to the field's declared type,
// exactly like a typed map value assignment already does.
//
// Before the fix, Struct.Set looked up the field's declared type via
// s.typeDef.fields, which is nil for a TypeKind wrapper -- so this
// assignment would have silently stored the raw string "42" with no
// conversion at all.
func TestStructSet_NamedTypeCoercion(t *testing.T) {
	s := NewStruct(newNamedEmployeeType())

	// NewStruct already zero-initializes "Age" to 0; overwrite it with a
	// distinct value first so a later "unchanged" assertion (in the sibling
	// test below) can't accidentally pass just because 0 happened to match.
	s.SetAlways("Age", 30)

	if err := s.Set("Age", "42"); err != nil {
		t.Fatalf("Set(\"Age\", \"42\") returned unexpected error: %v", err)
	}

	got, ok := s.Get("Age")
	if !ok {
		t.Fatal("field 'Age' not found after Set")
	}

	if got != 42 {
		t.Errorf("Age = %v (%T), want int 42", got, got)
	}
}

// TestStructSet_NamedTypeNonCoercibleValuePreservesField verifies two things
// at once for dynamic mode:
//
//  1. Assigning a value that cannot be converted to the field's declared
//     type at all (the string "old" to an int field) is reported as an
//     error, rather than being silently accepted (the original BUG-33
//     symptom).
//  2. The field keeps its previous, valid value instead of being reset to
//     the zero value of its declared type. An earlier version of this fix
//     let a failed Coerce() fall through to the final field write, which
//     zeroed the field even though an error was also returned -- a subtler
//     variant of "the type declaration is not enforced".
func TestStructSet_NamedTypeNonCoercibleValuePreservesField(t *testing.T) {
	s := NewStruct(newNamedEmployeeType())
	s.SetAlways("Age", 30)

	err := s.Set("Age", "old")
	if err == nil {
		t.Fatal("expected an error assigning the non-numeric string \"old\" to an int field")
	}

	got, _ := s.Get("Age")
	if got != 30 {
		t.Errorf("Age = %v, want unchanged 30 after a failed coercion", got)
	}
}

// TestStructSet_NamedTypeStrictModeRejectsMismatch verifies strict-mode
// behavior: when SetStrictTypeChecks(true) is in effect (the data-package
// mechanism the bytecode layer's checkStructFieldStrictType relies on for
// --types strict), a value whose type does not already match the field's
// declared type is rejected immediately with ErrInvalidType -- no coercion
// is attempted, even though "42" could have been converted to an int.
func TestStructSet_NamedTypeStrictModeRejectsMismatch(t *testing.T) {
	s := NewStruct(newNamedEmployeeType())
	s.SetStrictTypeChecks(true)
	s.SetAlways("Age", 30)

	err := s.Set("Age", "42")
	if !errors.Equals(err, errors.ErrInvalidType) {
		t.Fatalf("Set error = %v, want ErrInvalidType", err)
	}

	got, _ := s.Get("Age")
	if got != 30 {
		t.Errorf("Age = %v, want unchanged 30 after a rejected strict-mode assignment", got)
	}
}

// TestStructSet_NamedTypeStrictModeAllowsMatchingType verifies that strict
// mode still permits an assignment whose value already matches the field's
// declared type -- strict mode enforces the type, it does not forbid all
// writes.
func TestStructSet_NamedTypeStrictModeAllowsMatchingType(t *testing.T) {
	s := NewStruct(newNamedEmployeeType())
	s.SetStrictTypeChecks(true)

	if err := s.Set("Age", 31); err != nil {
		t.Fatalf("Set(\"Age\", 31) returned unexpected error: %v", err)
	}

	got, _ := s.Get("Age")
	if got != 31 {
		t.Errorf("Age = %v, want 31", got)
	}
}
