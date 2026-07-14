package bytecode

// types.go implements bytecode instructions that deal with Ego's runtime type
// system: inspecting a value's type, asserting a value to a named type,
// enforcing type-strictness mode, dereferencing pointers, and taking addresses.
//
// # Ego's type system in brief
//
// Every value in the Ego runtime carries an implicit type.  The data package
// defines a *data.Type descriptor for each Ego type (int, string, bool, …) and
// provides helpers like data.TypeOf(v) to determine a value's type at runtime.
//
// # Interface wrapping
//
// When Ego code uses the interface{} / any keyword, the runtime wraps the
// concrete value in a data.Interface struct that keeps the value and its type
// together.  Several instructions here must "unwrap" these structs before
// inspecting or coercing the contained value.
//
// # Type-strictness levels
//
// The context carries a typeStrictness field (and the matching symbol-table
// variable defs.TypeCheckingVariable) that controls how rigidly argument types
// are checked:
//
//	defs.StrictTypeEnforcement  (0) — exact type match required
//	defs.RelaxedTypeEnforcement (1) — compatible types; some coercions allowed
//	defs.NoTypeEnforcement      (2) — widest possible coercions allowed
//
// Several instructions behave differently depending on this setting.

import (
	"reflect"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// typeOfByteCode is the instruction handler for the TypeOf opcode.
//
// It pops one value from the stack and pushes back the *data.Type that
// describes that value's Ego type.  This is the runtime implementation
// of Ego's reflect.TypeOf() equivalent.
//
// Stack contract:
//   - Pops:  one value of any type.
//   - Pushes: one *data.Type.
//
// The operand is ignored.
func typeOfByteCode(c *Context, i any) error {
	value, err := c.Pop()
	if err != nil {
		return err
	}

	t := data.TypeOf(value)
	_ = c.push(t)

	return nil
}

// unwrapByteCode is the instruction handler for the UnWrap opcode.
//
// It pops a value from the stack and, optionally, attempts to assert it to a
// named target type.  The operand controls the behavior:
//
//   - nil operand — "plain unwrap": if the value is wrapped in a data.Interface,
//     strip the wrapper.  Push the concrete type (*data.Type) and then the
//     concrete value (two pushes, type on top of stack).
//
//   - "type" operand — same as nil; used when the Ego compiler emits an explicit
//     `.(type)` assertion in a type-switch.
//
//   - any other string — named type assertion `value.(TargetType)`.  The target
//     type is resolved by searching data.TypeDeclarations and then the symbol
//     table.  On success, pushes the (possibly coerced) value and then a bool
//     indicating success (bool on top of stack).  On strict-mode failure,
//     pushes (nil, false).
//
// Stack contract for named type assertion:
//   - Pops:  one value.
//   - Pushes: (value, bool) — bool on top, so the comma-ok idiom works as
//     expected: `v, ok := x.(T)` pops ok first, then v.
func unwrapByteCode(c *Context, i any) error {
	var (
		t        *data.Type
		newType  *data.Type
		newValue any
	)

	value, err := c.Pop()
	if err != nil {
		return err
	}

	// If the value is wrapped in a data.Interface (the Ego runtime's typed
	// interface container), strip the wrapper to expose the concrete value
	// and its type.
	if _, ok := value.(data.Interface); ok {
		value, t = data.UnWrap(value)
	}

	// If there is no argument, this unwrap doesn't require any tests, but
	// just reports the actual value and type on the stack.
	if i == nil {
		if t == nil {
			t = data.TypeOf(value)
		}

		_ = c.push(t)
		_ = c.push(value)

		return nil
	}

	actualType := data.TypeOf(value)

	// If the compiler already resolved the assertion's target type at
	// compile time, the operand is the *data.Type itself rather than a name
	// string -- use it directly and skip the by-name resolution below
	// entirely. This is always the case for a compound type specification
	// (a pointer, slice, map, struct, interface literal, or function type
	// such as "func() int"), which has no single name to look up here; see
	// compileUnwrap in the compiler package (BUG-60).
	if resolvedType, ok := i.(*data.Type); ok {
		newType = resolvedType
	} else {
		targetType := data.String(i)

		// Special case: the operand "type" is the `.(type)` token used inside
		// type-switch statements.  It means "just unwrap" with no additional
		// conformance check.
		if targetType == tokenizer.TypeToken.Spelling() {
			if t == nil {
				t = data.TypeOf(value)
			}

			_ = c.push(t)
			_ = c.push(value)

			return nil
		}

		// Resolve the target type name.  Look first in the built-in type registry
		// (data.TypeDeclarations), then fall back to the current symbol table in
		// case the programmer defined a custom type alias.

		// "any" is a programmer-friendly alias for "interface{}".  The canonical
		// name stored in data.InterfaceType.Name() is "interface{}", so the lookup
		// loop below would never match "any" without this normalization.  The other
		// source-code spellings ("interface {}", "interface{}") are handled by the
		// TypeDeclarations entries directly because they all map to the same
		// *data.Type whose Name() is "interface{}".
		if targetType == "any" {
			targetType = data.InterfaceTypeName
		}

		for _, td := range data.TypeDeclarations {
			if td.Kind.Name() == targetType {
				newType = td.Kind

				break
			}
		}

		if newType == nil {
			if td, found := c.symbols.Get(targetType); found {
				if tdx, ok := td.(*data.Type); ok {
					newType = tdx
				}
			}
		}

		if newType == nil {
			return errors.ErrInvalidType.Context(targetType)
		}
	}

	// Apply the type assertion.
	//
	// A type assertion asks "does this interface value actually hold a value of
	// type T?"  It is NOT a coercion.  The previous non-strict implementation
	// called data.Coerce unconditionally, which made every assertion succeed by
	// silently converting the value — defeating the entire purpose of the
	// comma-ok idiom.  For example:
	//
	//   var v any = 42
	//   s, ok := v.(string)   // used to give s="42", ok=true  (WRONG)
	//
	// The correct result is s=nil, ok=false because v holds an int, not a string.
	//
	// Coercion is already available through explicit cast functions (int(),
	// string(), float64(), …).  Using coercion inside a type assertion makes it
	// impossible to use the two-value form as a runtime type guard, which is its
	// primary purpose.
	//
	// The same exact-match behavior applies in ALL type-strictness modes
	// (strict, relaxed, and dynamic):
	//
	//   ─ If the target type is an interface (any / interface{}), every value
	//     passes — any concrete type satisfies the empty interface.
	//
	//   ─ Otherwise the actual stored type must match the target type exactly.
	//     On mismatch, push (nil, false) and return; the compiler arranges for
	//     an IfError instruction to follow the assertion in the single-value form
	//     (v := x.(T)), which converts the false into a catchable runtime error
	//     equivalent to Go's type-assertion panic.
	if newType.Kind() == data.InterfaceKind {
		// Asserting to interface{}/any always succeeds regardless of the
		// concrete type — every value satisfies the empty interface.
		newValue = value
	} else if !actualType.IsType(newType) {
		// The value's actual type does not match the asserted target type.
		// Push the failure sentinels: nil value and false ok-flag.
		// The bool is pushed last so it sits on top of the stack; the
		// compiler-generated IfError instruction (single-value form) or the
		// store instruction for the ok variable (comma-ok form) pops it first.
		_ = c.push(nil)
		_ = c.push(false)

		return nil
	} else {
		// Type matches: pass the value through unchanged.
		newValue = value
	}

	// Push result and success indicator.  The bool is pushed last so it is
	// on top — the Ego comma-ok idiom pops the bool first.
	_ = c.push(newValue)
	_ = c.push(newValue != nil)

	return nil
}

// staticTypingByteCode is the instruction handler for the StaticTyping opcode.
//
// It pops an integer from the stack and stores it as the context's
// type-strictness setting.  The valid range is:
//
//	0 = defs.StrictTypeEnforcement   — exact type match required
//	1 = defs.RelaxedTypeEnforcement  — compatible types; some coercions allowed
//	2 = defs.NoTypeEnforcement       — widest possible coercions
//
// The setting is also written to the symbol table under
// defs.TypeCheckingVariable so that compiled Ego code can read it.
//
// This opcode is emitted when the programmer writes a compile-time type-mode
// directive in Ego source.
func staticTypingByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		value, err := data.Int(v)
		if err != nil {
			return c.runtimeError(err)
		}

		if value < defs.StrictTypeEnforcement || value > defs.NoTypeEnforcement {
			return c.runtimeError(errors.ErrInvalidValue).Context(value)
		}

		c.typeStrictness = value
		c.symbols.SetAlways(defs.TypeCheckingVariable, value)
	}

	return err
}

// sandboxByteCode is the instruction handler for the Sandbox opcode.
//
// It pops two values from the stack: an optional sandbox-root path (or nil
// if the directive had no "path=" clause) and a boolean. If the path is
// present, it is written to the ephemeral settings overlay (the same one
// @optimizer uses) *before* the flag is applied, so that this happens at
// exactly the point the @sandbox directive appears in the instruction
// stream rather than when it was compiled -- code earlier in the same test
// file is unaffected by a path set later on. The boolean is then applied
// via Context.Sandboxed(), enabling or disabling sandboxed I/O (and exec)
// restrictions for all subsequent native/runtime calls made in this
// context.
//
// This opcode is only ever emitted by the @sandbox test directive (see
// sandboxDirective in internal/language/compiler/directives.go), which is
// itself gated to only compile when the compiler is running in test mode.
// Ordinary compiled Ego source has no way to reach this opcode, so
// untrusted code (for example, code submitted to the dashboard's sandboxed
// "run code" handler) cannot use it to lift its own sandbox restrictions or
// repoint the sandbox root.
func sandboxByteCode(c *Context, i any) error {
	flagValue, err := c.Pop()
	if err != nil {
		return err
	}

	flag, err := data.Bool(flagValue)
	if err != nil {
		return c.runtimeError(err)
	}

	pathValue, err := c.Pop()
	if err != nil {
		return err
	}

	// A literal nil (pushed by sandboxDirective when there was no "path="
	// clause at all) means "leave the existing sandbox root untouched".
	// Any other value -- including an explicit path="" -- came from a real
	// expression and is converted with data.String rather than a raw type
	// assertion, since an expression's result may arrive boxed (e.g. as a
	// *data.Interface) rather than as a bare Go string.
	if pathValue != nil {
		settings.SetDefault(defs.SandboxPathSetting, data.String(pathValue))
	}

	c.Sandboxed(flag)

	return nil
}

// requiredTypeByteCode is the instruction handler for the RequiredType opcode.
//
// It pops a value from the stack, verifies that it conforms to the type
// described by the operand, and pushes the (possibly coerced) value back.
//
// This is used for variadic parameters (compileFunctionParameters) and thrown
// values (throw.go), where there is no way to know whether the value being
// checked came from a compile-time constant, so it always calls
// requiredTypeCheckValue with valueIsConst=false. arg.go's non-variadic
// parameter path calls requiredTypeByteCodeWithConst instead, which does know
// (BUG-67).
//
// The operand `i` can be a *data.Type, a reflect.Type, a string type name, or
// an integer that represents an expected Go primitive type.
func requiredTypeByteCode(c *Context, i any) error {
	return requiredTypeByteCodeImpl(c, i, false)
}

// requiredTypeByteCodeWithConst behaves exactly like requiredTypeByteCode,
// except the caller (arg.go, for a non-variadic named parameter) reports
// whether the value being checked came from a compile-time constant literal
// at the call site. strictConformanceCheck uses this to let a numeric
// constant adapt to a narrower declared parameter type in strict mode
// (BUG-67), the same leniency getComparisonTerms already grants for ==.
func requiredTypeByteCodeWithConst(c *Context, i any, valueIsConst bool) error {
	return requiredTypeByteCodeImpl(c, i, valueIsConst)
}

// requiredTypeByteCodeImpl pops a value from the stack, verifies that it
// conforms to the type described by operand i, and pushes the (possibly
// coerced) value back.
//
// The check is delegated to one of two helper functions based on the active
// type-strictness level:
//
//   - Non-strict: relaxedConformanceCheck — tries to coerce the value.
//   - Strict:     strictConformanceCheck  — requires an exact type match,
//     except for a numeric constant adapting to a numeric target (BUG-67).
//
// On error, the helper returns a non-nil error which is propagated immediately
// (the value is NOT pushed in that case).
func requiredTypeByteCodeImpl(c *Context, i any, valueIsConst bool) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		// Ugly case of native types tested using horrible reflection string munging.
		if t, ok := i.(*data.Type); ok {
			a := t.String()

			switch realV := v.(type) {
			case *any:
				pV := *realV
				switch innerV := pV.(type) {
				default:
					b := reflect.TypeOf(innerV).String()
					if a == b {
						return c.push(v)
					}
				}
			}
		}

		// Dispatch to the appropriate conformance checker based on the
		// type-strictness level set in the context.
		if c.typeStrictness != defs.StrictTypeEnforcement {
			// Nope, try regular stuff.
			v, err = relaxedConformanceCheck(c, i, v)
			if err != nil {
				return err
			}
		} else {
			v, err = strictConformanceCheck(c, i, v, valueIsConst)
			if err != nil {
				return err
			}
		}

		_ = c.push(v)
	}

	return err
}

// strictConformanceCheck verifies that value v exactly matches the type
// described by operand i under strict-mode type checking.
//
// The operand i may be a *data.Type (the most common case) or an
// interface{}-typed value that carries type information.  If i is an interface
// type, a full interface-conformity check is performed (the value's type must
// implement all of the interface's declared methods).
//
// valueIsConst reports whether v came from a compile-time constant literal at
// the call site (currently only meaningful for non-variadic function
// arguments, see requiredTypeByteCodeWithConst). When true and both v and t
// are numeric, a kind mismatch that would otherwise be rejected is instead
// allowed to fall through to the coercion below -- mirroring Go's
// untyped-constant conversion rule (e.g. f(4) where f's parameter is int32)
// and the same leniency getComparisonTerms already grants for == (BUG-67).
//
// Returns the (possibly identity-coerced) value and nil on success, or
// (nil, error) if the types do not match.
func strictConformanceCheck(c *Context, i any, v any, valueIsConst bool) (any, error) {
	var err error

	t := data.TypeOf(i)
	// If it's not interface type, check it out...
	if !t.IsInterface() {
		// A nil value is a valid zero value for the built-in "error" type
		// (Go's nil error), so let it through unchanged. Anything else
		// destined for an error-typed parameter/return falls through to the
		// normal actualType.IsType(t) conformance check below.
		if t.IsKind(data.ErrorKind) && v == nil {
			return v, nil
		}

		if _, ok := v.(*ByteCode); ok {
			if t.IsKind(data.FunctionKind) {
				// It's bytecode and a function definition, and we aren't
				// doing strict type checks. So consider this conformant.
				return v, nil
			}
		}
		// Figure out the type. If it's a user type, get the underlying type unless we're
		// testing against an interface (in which case we need the full type info to get the
		// list of functions).
		actualType := data.TypeOf(v)

		// *chan and chan will be considered valid matches
		if actualType.Kind() == data.PointerKind && actualType.BaseType().Kind() == data.ChanKind {
			actualType = actualType.BaseType()
		}

		if actualType.Kind() == data.TypeKind && !t.IsInterface() {
			actualType = actualType.BaseType()
		}

		if !actualType.IsType(t) {
			// Fix BUG-67 leniency: a numeric constant literal adapts to the
			// declared numeric parameter type instead of being rejected
			// outright -- but, per BUG-68, only when doing so loses no
			// information, exactly mirroring variable assignment's existing
			// rule (and real Go's static rejection of a lossy untyped
			// constant). This returns directly rather than falling through
			// to the coercion switch below, since CoerceLossless already
			// produces the canonically-coerced value on success.
			if !(valueIsConst && data.IsNumeric(v) && data.IsNumeric(t)) {
				return nil, c.runtimeError(errors.ErrArgumentType)
			}

			coerced, err := data.CoerceLossless(v, data.InstanceOfType(t))
			if err != nil {
				return nil, c.runtimeError(err)
			}

			return coerced, nil
		}

		// Perform a canonical coercion so the value's Go type precisely
		// matches the declared Ego type.  For example, if the declared type
		// is int and the value arrived as int64, coerce it to int.
		switch t.Kind() {
		case data.IntKind:
			v, err = data.Int(v)

		case data.Int32Kind:
			v, err = data.Int32(v)

		case data.Int64Kind:
			v, err = data.Int64(v)

		case data.BoolKind:
			v, err = data.Bool(v)

		case data.ByteKind:
			v, err = data.Byte(v)

		case data.Float32Kind:
			v, err = data.Float32(v)

		case data.Float64Kind:
			v, err = data.Float64(v)

		case data.StringKind:
			v = data.String(v)
			err = nil
		}
	} else {
		// It is an interface type, if it's a non-empty interface
		// verify the value against the interface entries.
		if t.HasFunctions() {
			vt := data.TypeOf(v)
			if e := t.ValidateInterfaceConformity(vt); e != nil {
				return nil, c.runtimeError(e)
			}
		}
	}

	return v, err
}

// relaxedConformanceCheck verifies that value v conforms to the type described
// by operand i under non-strict (relaxed or dynamic) type checking.
//
// The operand i is inspected by type-switching through several cases:
//
//   - *data.Type with FunctionKind — if v is a *ByteCode whose declaration
//     conforms to the function type, the value passes.
//   - *data.Type with InterfaceKind — wrap v in a data.Interface container.
//   - reflect.Type                 — compare Go reflection types directly.
//   - string                       — compare the string against reflect.TypeOf(v).String().
//     A nil v is treated as a mismatch (TYPES-2 fix: the original code called
//     reflect.TypeOf(nil).String() which panics).
//   - any integer/bool/float type  — type-switch on i directly (TYPES-3 fix).
//     The original code extracted i.(int) first, so the subsequent Kind switch
//     always produced IntKind, making int8/int16/int32/… cases unreachable.
//     Switching on i.(type) preserves the original Go type so each case is
//     now reachable.  Non-numeric operands (e.g. *data.Type) hit the default
//     branch and pass the value through unchanged.
//
// Returns the (possibly wrapped) value and nil on success, or (v, error) on
// type mismatch.
func relaxedConformanceCheck(c *Context, i any, v any) (any, error) {
	var err error

	if xf, ok := i.(*data.Type); ok {
		if xf.Kind() == data.FunctionKind {
			if fd := xf.GetFunctionDeclaration(""); fd != nil {
				if bc, ok := v.(*ByteCode); ok {
					if data.ConformingDeclarations(bc.Declaration(), fd) {
						return v, nil
					}
				}
			}
		}

		if xf.Kind() == data.InterfaceType.Kind() {
			v = data.Wrap(v)
		}
	}

	if t, ok := i.(reflect.Type); ok {
		if t != reflect.TypeOf(v) {
			err = c.runtimeError(errors.ErrArgumentType)
		}
	} else {
		if t, ok := i.(string); ok {
			// TYPES-2 fix: guard against nil before calling reflect.TypeOf(v).String().
			// reflect.TypeOf(nil) returns nil, and calling .String() on a nil
			// reflect.Type panics.  A nil value can never match a named type string.
			if v == nil || t != reflect.TypeOf(v).String() {
				err = c.runtimeError(errors.ErrArgumentType)
			}
		} else {
			// TYPES-3 fix: type-switch on i directly rather than extracting
			// i.(int) first.  The original extraction narrowed every integer
			// type to plain Go int, so data.TypeOf(t).Kind() always returned
			// IntKind and all other cases were dead.  Switching on i.(type)
			// preserves the actual Go type (int16, int32, float32, …) so each
			// case is now reachable.  Non-integer operand types (e.g. *data.Type
			// values handled by the block above) fall through to the default
			// branch, which accepts the value as-is — matching the original
			// no-op behavior for those operands.
			var kindOk bool

			switch i.(type) {
			case int:
				_, kindOk = v.(int)

			case int8:
				_, kindOk = v.(int8)

			case int16:
				_, kindOk = v.(int16)

			case int32:
				_, kindOk = v.(int32)

			case int64:
				_, kindOk = v.(int64)

			case uint16:
				_, kindOk = v.(uint16)

			case byte: // uint8
				_, kindOk = v.(byte)

			case bool:
				_, kindOk = v.(bool)

			case float32:
				_, kindOk = v.(float32)

			case float64:
				_, kindOk = v.(float64)

			default:
				kindOk = true
			}

			if !kindOk {
				err = c.runtimeError(errors.ErrArgumentType)
			}
		}
	}

	return v, err
}

// addressOfByteCode is the instruction handler for the AddressOf opcode.
//
// It looks up the named symbol in the symbol table, retrieves a pointer to
// the symbol's value storage slot (a *any), and pushes that pointer onto the
// stack.  This implements the Ego `&name` address-of operator.
//
// Operand: the symbol name as a string.
//
// Stack contract:
//   - Pushes: a *any pointer to the named symbol's storage slot.
//
// Returns ErrUnknownIdentifier if the name is not found in the visible scope.
func addressOfByteCode(c *Context, i any) error {
	name := data.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(addr)
}

// deRefByteCode is the instruction handler for the DeRef opcode.
//
// It looks up the named symbol, interprets its value as an Ego pointer (a
// *any stored inside the symbol's slot), and pushes the value that the
// pointer points to.  This implements the Ego `*name` dereference operator.
//
// Operand: the symbol name as a string.
//
// Stack contract:
//   - Pushes: the dereferenced value.
//
// Pointer representation in Ego:
//
//	GetAddress("p") returns addr (*any) — pointer to the symbol's value SLOT.
//	*addr = content                     — the value stored in the slot (must
//	                                      be *any, the Ego pointer value).
//	*content = c2                       — dereferences the Ego pointer.
//	   c2 itself may be another *any    — a doubly-indirect pointer (pointer to
//	                                      pointer); dereference once more.
//
// Nil handling (TYPES-1 fix): the inner pointer `c3` is checked for nil before
// dereferencing.  A nil inner pointer means the Ego pointer variable was
// declared but never assigned, and returns ErrNilPointerReference.
//
// Error conditions:
//   - ErrUnknownIdentifier   — symbol not in scope.
//   - ErrNilPointerReference — the pointer (or its target) is nil.
//   - ErrNotAPointer         — the symbol does not hold a pointer value.
//
func deRefByteCode(c *Context, i any) error {
	name := data.String(i)

	// Step 1: resolve the symbol to its storage address.
	// GetAddress returns a *any pointing to the symbol's value slot in the
	// symbol table's internal storage.
	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
	}

	if data.IsNil(addr) {
		return c.runtimeError(errors.ErrNilPointerReference)
	}

	// Step 2: the symbol's value slot must contain a *any — that is the Ego
	// pointer value stored in the variable.
	if content, ok := addr.(*any); ok {
		if data.IsNil(content) {
			return c.runtimeError(errors.ErrNilPointerReference)
		}

		// Step 3: dereference the outer pointer to get the Ego pointer value.
		c2 := *content
		// Step 4: the Ego pointer value itself is a *any; dereference it to
		// reach the pointed-to storage.
		if c3, ok := c2.(*any); ok {
			// TYPES-1 fix: guard against a nil inner pointer BEFORE dereferencing.
			// The original check tested `content` (the outer pointer, always
			// non-nil here) instead of `c3`; a nil `c3` caused `*c3` to panic.
			if c3 == nil {
				return c.runtimeError(errors.ErrNilPointerReference)
			}

			// Step 5: dereference the inner pointer.  c3 is guaranteed non-nil.
			xc3 := *c3
			// Step 6: if the target slot holds an Immutable wrapper (read-only
			// constant), expose the underlying value.
			if c4, ok := xc3.(data.Immutable); ok {
				return c.push(c4.Value)
			}

			return c.push(*c3)
		}

		return c.runtimeError(errors.ErrNotAPointer).Context(data.Format(c2))
	}

	return c.runtimeError(errors.ErrNotAPointer).Context(name)
}
