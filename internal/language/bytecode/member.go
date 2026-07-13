package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// memberByteCode implements the Member opcode: it loads a named member
// (field or method) from an object and pushes the result onto the stack.
//
// The member name can be provided in two ways:
//
//  1. From the instruction operand (i):  memberByteCode(ctx, "FieldName")
//     — the object is the single value popped from the stack.
//
//  2. From the top of the stack (i == nil):
//     — the stack must look like  ... | object | name (top)
//     — the name is popped first, then the object.
//
// Supported object types (handled by getMemberValue):
//   - *data.Struct  — field or method on a struct value
//   - *data.Package — exported symbol from a package
//   - *data.Map     — keyed value (language extensions must be enabled)
//   - *data.Type    — registered function lookup; ErrUnknownMember if absent
//   - *any          — dereferenced to reach the actual value
//   - other         — native Go type; reflection-based method lookup
func memberByteCode(c *Context, i any) error {
	var (
		v    any
		m    any
		name string
		err  error
	)

	if i != nil {
		// Fast path: the operand already holds the member name as a value.
		// data.String converts any type to its string representation.
		name = data.String(i)
	} else {
		// The name must be popped from the stack (it was pushed by the
		// compiler before emitting the Member opcode).
		v, err = c.Pop()
		if err != nil {
			// MEMBERS-1 fix: decorate with c.runtimeError so the error carries
			// the current module name and source line, matching every other Pop
			// error path in the bytecode package.
			return c.runtimeError(err)
		}

		// A StackMarker here means a function call produced no value and the
		// compiler left a void sentinel on the stack instead of a real name.
		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		name = data.String(v)
	}

	// Pop the object whose member we are reading.
	m, err = c.Pop()
	if err != nil {
		// MEMBERS-1 fix: decorate the object-pop error the same way as the
		// name-pop error above.
		return c.runtimeError(err)
	}

	// Dispatch to the type-specific helper.  On success push the result.
	v, err = getMemberValue(c, m, name)
	if err == nil {
		return c.push(v)
	}

	return err
}

// getMemberValue resolves the named member of the object m and returns its
// value.  It is the central dispatch function for the Member opcode and is
// also called recursively when dereferencing *any or *data.Type values.
//
// The function handles the following cases in priority order:
//
//  1. *data.Type  — looks up name as a registered function on the type;
//     returns ErrUnknownMember if no such function exists (MEMBERS-6 fix).
//  2. StackMarker — returns ErrFunctionReturnedVoid (MEMBERS-2 fix).
//  3. *any        — dereferences the pointer and recurses.
//  4. *data.Map   — key lookup; only allowed with language extensions on.
//  5. *data.Struct — field or method via getStructMemberValue.
//  6. *data.Package — package symbol via getPackageMemberValue.
//  7. default     — native Go type via getNativePackageMemberValue.
func getMemberValue(c *Context, m any, name string) (any, error) {
	var (
		v   any
		err error
	)

	// Handle Ego type descriptor objects (*data.Type).
	//
	// MEMBERS-6 fix: the previous code returned t.String() for every member
	// name, silently ignoring what was actually requested.  The corrected
	// behavior is:
	//
	//   1. If the type has a registered Ego function under 'name'
	//      (added via Type.DefineFunction), return that function descriptor.
	//   2. If 'name' is "String", return t.String().  This is the one native
	//      Go method that Ego code legitimately calls on *data.Type values —
	//      the pattern reflect.Type(x).String() is used throughout tests and
	//      library code to obtain the type name as a printable string.
	//   3. Any other name returns ErrUnknownMember.
	if t, ok := m.(*data.Type); ok {
		if fn := t.Function(name); fn != nil {
			return data.UnwrapConstant(fn), nil
		}

		if name == "String" {
			// Preserve the native String() behavior for *data.Type so that
			// reflect.Type(x).String() continues to return the type name.
			return t.String(), nil
		}

		return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
	}

	// A StackMarker in the object position means no real object was produced
	// (e.g., a void-returning function call appeared as the LHS of a member
	// access expression).
	//
	// MEMBERS-2 fix: was returning errors.ErrFunctionReturnedVoid without
	// c.runtimeError() wrapping, so it lacked module/line location info.
	if isStackMarker(m) {
		return nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	switch mv := m.(type) {
	case *any:
		// The object is a pointer-to-interface (*any).  Dereference it and
		// dispatch again based on the concrete type stored inside.
		interfaceValue := *mv
		switch actual := interfaceValue.(type) {
		case *data.Struct:
			return getStructMemberValue(c, actual, name)

		case *data.Scalar:
			return getScalarMemberValue(c, actual, name)

		case *data.Type:
			// For a TypeDefinition, BaseType() returns the underlying type.
			// If actual is a nil *data.Type, BaseType() returns nil — add an
			// explicit error to prevent a silent (nil, nil) return (MEMBERS-7 fix).
			if bv := actual.BaseType(); bv != nil {
				return getMemberValue(c, bv, name)
			}
			// MEMBERS-7 fix: nil *data.Type receiver has no base type. Return
			// an error instead of falling through and pushing nil onto the stack.
			return nil, c.runtimeError(errors.ErrInvalidType).Context("nil type")

		default:
			// Any other type wrapped in *any is treated as a native Go type
			// (e.g., a Go struct that implements an Ego interface).
			return getNativePackageMember(c, actual, name, interfaceValue)
		}

	case *data.Map:
		// Map member access is only allowed when language extensions are on.
		// Standard Ego uses map["key"] indexing, not map.key dot access.
		if !c.extensions {
			return nil, c.runtimeError(errors.ErrInvalidTypeForOperation).Context(data.TypeOf(mv).String())
		}

		// Get the value at the given key.  The bool "found" return is
		// discarded, so a missing key silently returns nil (not an error).
		v, _, err = mv.Get(name)

	case *data.Struct:
		return getStructMemberValue(c, mv, name)

	case *data.Scalar:
		return getScalarMemberValue(c, mv, name)

	case *data.Package:
		// MEMBERS-5 fix: the dead v and found parameters were removed from
		// getPackageMemberValue, so the call is now simpler.
		return getPackageMemberValue(name, mv, c)

	default:
		// The object is a native Go value.  Reflection is used to look for
		// a method or a registered Ego function descriptor.
		return getNativePackageMemberValue(mv, name, c)
	}

	return data.UnwrapConstant(v), err
}

// getNativePackageMemberValue resolves a named member on a native Go value that
// does not match any of the dedicated Ego type cases (*data.Struct, *data.Map,
// *data.Package).  It tries two lookup strategies in order:
//
//  1. Ego package registry: derive the package/type name from the Go value's
//     reflected type string (e.g., "*time.Duration" → pkg="time",
//     typeName="Duration") and look up a Function registered under name on
//     that package's Type (e.g., via Type.DefineFunction or
//     Type.DefineNativeFunction). This is intentionally not gated on Go's own
//     reflect.MethodByName(name): an earlier version required the name to
//     also be a genuine Go method on the concrete type, which meant an
//     Ego-only extension method with no Go equivalent (e.g.
//     time.Time.SleepUntil) could never be found -- only Ego-registered
//     methods that happened to share a name with a real Go method (like
//     time.Duration.String) worked, purely by coincidence of the gate.
//
//  2. Registered Ego function: if the Ego type for mv has a Function entry
//     registered under name via data.TypeOf(mv), return it. This mainly
//     covers scalar/interface fallback kinds that step 1 doesn't reach.
//
// If neither lookup succeeds the result depends on the kind of the type:
//   - Scalar types (kind < MaximumScalarType): ErrInvalidTypeForOperation.
//   - Non-scalar types: ErrUnknownNativeField.
func getNativePackageMemberValue(mv any, name string, c *Context) (any, error) {
	// Step 1: derive the package/type name from the Go value's reflected type
	// string and look up name directly in the Ego package registry for that
	// type, regardless of whether Go's own reflection recognizes it as a
	// real method.
	gt := reflect.TypeOf(mv)
	text := gt.String()

	if parts := strings.Split(text, "."); len(parts) == 2 {
		pkg := strings.TrimPrefix(parts[0], "*")
		typeName := parts[1]

		// Walk: context symbols → *data.Package → *data.Type → Function.
		if pkgData, found := c.get(pkg); found {
			if pkg, ok := pkgData.(*data.Package); ok {
				if typeInterface, ok := pkg.Get(typeName); ok {
					if typeData, ok := typeInterface.(*data.Type); ok {
						fd := typeData.FunctionByName(name)
						if fd != nil {
							return *fd, nil
						}
					}
				}
			}
		}
	}

	// Step 2: check if the Ego type system has a Function registered under name.
	kind := data.TypeOf(mv)

	fnx := kind.Function(name)
	if fnx != nil {
		return fnx, nil
	}

	// Neither lookup found anything.  For scalar types (int, float, string, …)
	// member access is simply not defined → ErrInvalidTypeForOperation.
	// For complex types (structs wrapped in interface{}, etc.) it is more
	// accurate to say the field does not exist → ErrUnknownNativeField.
	if kind.Kind() < data.MaximumScalarType {
		return nil, c.runtimeError(errors.ErrInvalidTypeForOperation).Context(kind.String())
	}

	return nil, c.runtimeError(errors.ErrUnknownNativeField).Context(name)
}

// getPackageMemberValue looks up name inside the Ego package mv and returns its
// value, stripping any Immutable (read-only constant) wrapper.
//
// Lookup order for capitalized names (exported identifiers):
//  1. The package's embedded symbol table (populated at import time).
//  2. The package's items map (set via pkg.Set).
//  3. A function registered on the Ego type for the package.
//
// For lowercase names only steps 2 and 3 are attempted (the symbol-table check
// is skipped because lowercase names are never stored there in Ego).
//
// If no match is found in any location, ErrUnknownPackageMember is returned.
//
// MEMBERS-5 fix: the dead parameters v any and found bool were removed.
// They were always passed as zero values by the caller and immediately
// overwritten on the first use inside this function, making them unreachable.
// The function now declares its own local variables instead.
func getPackageMemberValue(name string, mv *data.Package, c *Context) (any, error) {
	// For capitalized (exported) names, check the embedded symbol table first.
	// GetPackageSymbolTable lazily creates the table if it doesn't exist yet.
	if egostrings.HasCapitalizedName(name) {
		syms := symbols.GetPackageSymbolTable(mv)
		if v, ok := syms.Get(name); ok {
			return data.UnwrapConstant(v), nil
		}
	}

	// Get the Ego type that describes this package.  In practice data.TypeOf
	// creates a new PackageType without registered functions, so the fallback
	// tt.Function(name) below is effectively dead code for standard packages.
	tt := data.TypeOf(mv)

	// Check the package's items map.
	v, found := mv.Get(name)
	if !found {
		// Last resort: see if the Ego type has a function registered under this
		// name (e.g., a type-level method added via Type.DefineFunction).
		if fv := tt.Function(name); fv == nil {
			return nil, c.runtimeError(errors.ErrUnknownPackageMember).Context(name)
		} else {
			v = fv
		}
	}

	return data.UnwrapConstant(v), nil
}

// getNativePackageMember is called when the value inside a *any is a native Go
// type that is not a *data.Struct or *data.Type (it hits the default case of the
// inner type switch inside getMemberValue's *any branch).
//
// It attempts the same reflection-based method lookup as getNativePackageMemberValue:
// if the Go type has a method called name, look it up in the Ego package registry.
//
// If the method is found in the Go type but not in the Ego registry (or if no
// method with that name exists on the type), the function returns ErrInvalidType
// with a diagnostic string showing both the Ego type and the underlying Go type.
//
// The interfaceValue parameter is the original interface value stored in the *any,
// used to produce the Ego type string for the error message.
func getNativePackageMember(c *Context, actual any, name string, interfaceValue any) (any, error) {
	// Capture the underlying Go type name for use in the error message below.
	realName := reflect.TypeOf(actual).String()

	// Check if the Go type exposes a method called name.
	gt := reflect.TypeOf(actual)
	if _, found := gt.MethodByName(name); found {
		// Attempt to resolve the method through the Ego package registry.
		// The reflect string (e.g., "*sync.Mutex") is split to get the package
		// name and the type name, then we walk through the Ego symbol chain.
		text := gt.String()

		if parts := strings.Split(text, "."); len(parts) == 2 {
			pkg := strings.TrimPrefix(parts[0], "*")
			typeName := parts[1]

			if pkgData, found := c.get(pkg); found {
				if pkg, ok := pkgData.(*data.Package); ok {
					if typeInterface, ok := pkg.Get(typeName); ok {
						if typeData, ok := typeInterface.(*data.Type); ok {
							fd := typeData.FunctionByName(name)
							if fd != nil {
								return *fd, nil
							}
						}
					}
				}
			}
		}
	}

	// The method could not be resolved through the Ego registry.  Build a
	// descriptive error that includes both the Ego type name and the Go type name
	// so the user (or developer) can understand what was being accessed.
	text := data.TypeOf(interfaceValue).String() + " (" + realName + ")"

	return nil, c.runtimeError(errors.ErrInvalidType).Context(text)
}

// getStructMemberValue retrieves the named member from a struct value.
// The member can be either a direct field stored in the struct, or a method
// defined on the struct's type (its "receiver function").
//
// Lookup order:
//  1. Direct field: mv.Get(name) checks the struct's fields map.
//  2. Type method:  data.TypeOf(mv).Function(name) looks for a Function
//     registered on the struct's associated type.  A Function entry is only
//     considered "found" if it has a non-nil Declaration or non-nil Value;
//     zero-value Function entries (stubs without any implementation) are
//     treated as not-found.
//
// Package visibility:
//
//	If the struct was defined in a named package (PackageName() != "") and the
//	current execution context is in a different package (c.pkg != pkg), then
//	only exported names (starting with an uppercase letter) are accessible.
//	Lowercase (unexported) names return ErrSymbolNotExported.
//
// NOTE (MEMBERS-3): the two error returns below are bare — not wrapped with
// c.runtimeError() — so they lack module/line location info.
// See docs/bytecode_issues.md MEMBERS-3.
func getStructMemberValue(c *Context, mv *data.Struct, name string) (any, error) {
	// Step 1: direct field lookup.
	v, found := mv.Get(name)

	if !found {
		// Step 2: look for a receiver method on the struct's type.
		v = data.TypeOf(mv).Function(name)
		found = (v != nil)

		// A data.Function with both Declaration == nil and Value == nil is a
		// stub entry (placeholder with no implementation).  Treat it as absent
		// so callers get ErrUnknownMember rather than a useless nil function.
		if decl, ok := v.(data.Function); ok {
			found = (decl.Declaration != nil) || decl.Value != nil
		}
	}

	if !found {
		// MEMBERS-3 fix: decorate with c.runtimeError so the error carries
		// the current module name and source-line position.
		return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
	}

	// Enforce package visibility: lowercase names in structs from a different
	// package are private and may not be accessed from outside.
	if pkg := mv.PackageName(); pkg != "" && pkg != c.pkg {
		if !egostrings.HasCapitalizedName(name) {
			// MEMBERS-3 fix: decorate with c.runtimeError for consistent
			// source-location annotation.
			return nil, c.runtimeError(errors.ErrSymbolNotExported).Context(name)
		}
	}

	// Strip any Immutable (read-only constant) wrapper before returning so
	// callers receive the plain underlying value.
	return data.UnwrapConstant(v), nil
}

// getScalarMemberValue resolves a receiver method (name) on a named scalar
// type value, e.g. func (b buzz) String() string {...}. Named scalar types
// have no fields, only methods, so this is a simplified version of
// getStructMemberValue that skips straight to the receiver-function lookup.
func getScalarMemberValue(c *Context, mv *data.Scalar, name string) (any, error) {
	v := data.TypeOf(mv).Function(name)
	found := v != nil

	// A data.Function with both Declaration == nil and Value == nil is a
	// stub entry (placeholder with no implementation).  Treat it as absent
	// so callers get ErrUnknownMember rather than a useless nil function.
	if decl, ok := v.(data.Function); ok {
		found = (decl.Declaration != nil) || decl.Value != nil
	}

	if !found {
		return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
	}

	return data.UnwrapConstant(v), nil
}
