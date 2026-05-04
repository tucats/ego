package reflect

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func getString(s *symbols.SymbolTable, args data.List) (any, error) {
	r := getThis(s)
	t := strings.Builder{}

	t.WriteString("Reflection{")

	first := true

	// field writes a "Label: value" pair, skipping empty strings.
	field := func(label, value string) {
		if value == "" {
			return
		}

		if !first {
			t.WriteString(", ")
		}

		t.WriteString(label)
		t.WriteString(": ")
		t.WriteString(value)

		first = false
	}

	// fieldTrue writes "Label: true" only when the stored bool value is true.
	// False values are the default and are never printed.
	fieldTrue := func(label string, v any) {
		if !data.BoolOrFalse(v) {
			return
		}

		if !first {
			t.WriteString(", ")
		}

		t.WriteString(label)
		t.WriteString(": true")

		first = false
	}

	// Name — set for named type definitions and named functions.
	if v := r.GetAlways(data.NameMDName); v != nil {
		field("Name", data.String(v))
	}

	// Type — the primary type descriptor; always present.
	if v := r.GetAlways(data.TypeMDName); v != nil {
		field("Type", data.String(v))
	}

	// IsType — only shown when true (i.e., the reflected item IS a type definition).
	if v := r.GetAlways(data.IsTypeMDName); v != nil {
		fieldTrue("IsType", v)
	}

	// BaseType — the underlying type (struct layout, element type, etc.).
	if v := r.GetAlways(data.BaseTypeMDName); v != nil {
		field("BaseType", data.String(v))
	}

	// Native — only shown when true (wraps a native Go type or function).
	if v := r.GetAlways(data.NativeMDName); v != nil {
		fieldTrue("Native", v)
	}

	// Builtins — only shown when true; meaningful only for packages.
	if v := r.GetAlways(data.BuiltinsMDName); v != nil {
		fieldTrue("Builtins", v)
	}

	// Imports — only shown when true; meaningful only for packages.
	if v := r.GetAlways(data.ImportsMDName); v != nil {
		fieldTrue("Imports", v)
	}

	// Error — the error key string; set only for error reflections.
	if v := r.GetAlways(data.ErrorMDName); v != nil {
		field("Error", data.String(v))
	}

	// Text — the human-readable error message; set only for error reflections.
	if v := r.GetAlways(data.TextMDName); v != nil {
		field("Text", data.String(v))
	}

	// Context — error context details; set only for errors with attached context.
	if v := r.GetAlways(data.ContextMDName); v != nil {
		field("Context", data.String(v))
	}

	// Declaration — function signature; set only for function reflections.
	if v := r.GetAlways(data.DeclarationMDName); v != nil {
		field("Declaration", data.String(v))
	}

	// Members — field names (structs), keys (maps), or exported names (packages).
	if v := r.GetAlways(data.MembersMDName); v != nil {
		field("Members", data.String(v))
	}

	// Functions — method list for types or struct instances with methods.
	if v := r.GetAlways(data.FunctionsMDName); v != nil {
		if _, ok := v.(data.Function); !ok {
			field("Functions", data.String(v))
		}
	}

	// Size — byte size (scalars) or element count (arrays, packages).
	// Only shown when non-zero.
	if v := r.GetAlways(data.SizeMDName); v != nil {
		if n := data.IntOrZero(v); n != 0 {
			field("Size", data.String(v))
		}
	}

	t.WriteString("}")

	return t.String(), nil
}
