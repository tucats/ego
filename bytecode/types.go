package bytecode

import (
	"reflect"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// typeOfByteCode pops the top stack item and replaces it with
// a value representinhg it's type.
func typeOfByteCode(c *Context, i interface{}) error {
	value, err := c.Pop()
	if err != nil {
		return err
	}

	t := data.TypeOf(value)
	_ = c.push(t)

	return nil
}

// unwrapByteCode unwraps the top of stack interface and
// attempts to cast it to the named type. If there is no
// named type, this just unwraps the value and pushes
// the type and value back to the stack.
func unwrapByteCode(c *Context, i interface{}) error {
	var (
		t        *data.Type
		newType  *data.Type
		newValue interface{}
	)

	value, err := c.Pop()
	if err != nil {
		return err
	}

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

	targetType := data.String(i)

	// Special case, if the type is "type" it really just means
	// unwrap it, and there's no action to be done here.
	if targetType == tokenizer.TypeToken.Spelling() {
		if t == nil {
			t = data.TypeOf(value)
		}

		_ = c.push(t)
		_ = c.push(value)

		return nil
	}

	actualType := data.TypeOf(value)

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

	// If we are not in stricted of type checking, just do the conversion
	// helpfully.  If we are in strict type checking, the types must match.
	if c.typeStrictness != defs.StrictTypeEnforcement {
		newValue = data.Coerce(value, newType.InstanceOf(newType.BaseType()))
	} else {
		if !actualType.IsType(newType) {
			_ = c.push(nil)
			_ = c.push(false)

			return nil
		}

		newValue = value
	}

	_ = c.push(newValue)
	_ = c.push(newValue != nil)

	return nil
}

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func staticTypingByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		value := data.Int(v)
		if value < defs.StrictTypeEnforcement || value > defs.NoTypeEnforcement {
			return c.error(errors.ErrInvalidValue).Context(value)
		}

		c.typeStrictness = value
		c.symbols.SetAlways(defs.TypeCheckingVariable, value)
	}

	return err
}

func requiredTypeByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		// Ugly case of native types tested using horrible reflection string munging.
		if t, ok := i.(*data.Type); ok {
			if true {
				a := t.String()

				switch realV := v.(type) {
				case *interface{}:
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
		}

		// If we're doing strict type checking...
		if c.typeStrictness != defs.StrictTypeEnforcement {
			// Nope, try regular stuff.
			if xf, ok := i.(*data.Type); ok {
				if xf.Kind() == data.FunctionKind {
					if fd := xf.GetFunctionDeclaration(""); fd != nil {
						if bc, ok := v.(*ByteCode); ok {
							if data.ConformingDeclarations(bc.Declaration(), fd) {
								_ = c.push(v)

								return nil
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
					err = c.error(errors.ErrArgumentType)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.error(errors.ErrArgumentType)
					}
				} else {
					if t, ok := i.(int); ok {
						switch data.TypeOf(t).Kind() {
						case data.IntKind:
							_, ok = v.(int)

						case data.Int32Kind:
							_, ok = v.(int32)

						case data.Int64Kind:
							_, ok = v.(int64)

						case data.ByteKind:
							_, ok = v.(byte)

						case data.BoolKind:
							_, ok = v.(bool)

						case data.StringKind:
							_, ok = v.(string)

						case data.Float32Kind:
							_, ok = v.(float32)

						case data.Float64Kind:
							_, ok = v.(float64)

						default:
							ok = true
						}

						if !ok {
							err = c.error(errors.ErrArgumentType)
						}
					}
				}
			}
		} else {
			t := data.TypeOf(i)
			// If it's not interface type, check it out...
			if !t.IsInterface() {
				if t.IsKind(data.ErrorKind) {
					v = errors.ErrPanic.Context(v)
				}

				if _, ok := v.(*ByteCode); ok {
					if t.IsKind(data.FunctionKind) {
						// It's bytecode and a function definition, and we aren't
						// doing strict type checks. So consider this conformant.
						_ = c.push(v)

						return nil
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
					return c.error(errors.ErrArgumentType)
				}

				switch t.Kind() {
				case data.IntKind:
					v = data.Int(v)

				case data.Int32Kind:
					v = data.Int32(v)

				case data.Int64Kind:
					v = data.Int64(v)

				case data.BoolKind:
					v = data.Bool(v)

				case data.ByteKind:
					v = data.Byte(v)

				case data.Float32Kind:
					v = data.Float32(v)

				case data.Float64Kind:
					v = data.Float64(v)

				case data.StringKind:
					v = data.String(v)
				}
			} else {
				// It is an interface type, if it's a non-empty interface
				// verify the value against the interface entries.
				if t.HasFunctions() {
					vt := data.TypeOf(v)
					if e := t.ValidateInterfaceConformity(vt); e != nil {
						return c.error(e)
					}
				}
			}
		}

		_ = c.push(v)
	}

	return err
}

func addressOfByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(addr)
}

func deRefByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	if data.IsNil(addr) {
		return c.error(errors.ErrNilPointerReference)
	}

	if content, ok := addr.(*interface{}); ok {
		if data.IsNil(content) {
			return c.error(errors.ErrNilPointerReference)
		}

		c2 := *content
		if c3, ok := c2.(*interface{}); ok {
			xc3 := *c3
			if c4, ok := xc3.(data.Immutable); ok {
				return c.push(c4.Value)
			}

			if data.IsNil(content) {
				return c.error(errors.ErrNilPointerReference)
			}

			return c.push(*c3)
		}

		return c.error(errors.ErrNotAPointer).Context(data.Format(c2))
	}

	return c.error(errors.ErrNotAPointer).Context(name)
}
