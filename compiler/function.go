package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Descriptor of each parameter in the parameter list.
type parameter struct {
	name string
	kind int
}

// Function compiles a function definition. The literal flag indicates if
// this is a function literal, which is pushed on the stack, or a non-literal
// which is added to the symbol table dictionary.
func (c *Compiler) Function(literal bool) error {
	// List of type coercions that will be needed for any RETURN statement.
	coercions := []*bytecode.ByteCode{}
	parameters := []parameter{}
	this := ""
	fname := ""
	class := ""
	byValue := true

	// If it's not a literal, there will be a function name, which must be a valid
	// symbol name. It might also be an object-oriented (a->b()) call.
	if !literal {
		fname = c.t.Next()
		// Is this receiver notation?
		if fname == "(" {
			this = c.t.Next()
			if c.t.IsNext("*") {
				byValue = false
			}
			class = c.t.Next()
			if !tokenizer.IsSymbol(this) {
				return c.NewError(InvalidSymbolError, this)
			}
			if !tokenizer.IsSymbol(class) {
				return c.NewError(InvalidSymbolError, class)
			}
			if !c.t.IsNext(")") {
				return c.NewError(MissingParenthesisError)
			}
			fname = c.t.Next()
		}

		if !tokenizer.IsSymbol(fname) {
			return c.NewError(InvalidFunctionName, fname)
		}
		fname = c.Normalize(fname)
	}

	// Process the function parameter specification
	varargs := false
	if c.t.IsNext("(") {
		for !c.t.IsNext(")") {
			if c.t.AtEnd() {
				break
			}
			name := c.t.Next()
			p := parameter{kind: datatypes.UndefinedType}
			if tokenizer.IsSymbol(name) {
				p.name = name
			} else {
				return c.NewError(InvalidFunctionArgument)
			}
			if c.t.Peek(1) == "..." {
				c.t.Advance(1)
				varargs = true
			}

			// Is there a type name that follows it? We have to check for "[]" and "{}"
			// as two different tokens. Also note that you can use the word array or struct
			// instead if you wish.
			if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
				p.kind = datatypes.ArrayType
				c.t.Advance(2)
			} else if c.t.Peek(1) == "{}" {
				p.kind = datatypes.StructType
				c.t.Advance(1)
			} else if util.InList(c.t.Peek(1), "chan", "interface{}", "int", "string", "bool", "double", "float", "array", "struct") {
				switch c.t.Next() {
				case "int":
					p.kind = datatypes.IntType

				case "string":
					p.kind = datatypes.StringType

				case "bool":
					p.kind = datatypes.BoolType

				case "float", "double":
					p.kind = datatypes.FloatType

				case "struct":
					p.kind = datatypes.StructType

				case "array":
					p.kind = datatypes.ArrayType

				case "chan":
					p.kind = datatypes.ChanType
				}
			}
			if varargs {
				p.kind = datatypes.VarArgs
			}
			parameters = append(parameters, p)
			_ = c.t.IsNext(",")
		}
	}
	b := bytecode.New(fname)
	// If we know our source file, mark it in the bytecode now.
	if c.SourceFile != "" {
		b.Emit(bytecode.FromFile, c.SourceFile)
	}

	// Generate the argument check
	p := []interface{}{
		len(parameters),
		len(parameters),
		fname,
	}
	if varargs {
		p[0] = len(parameters)
		p[1] = -1
	}

	b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])
	b.Emit(bytecode.ArgCheck, p)

	// If there was a "this" variable defined, process it now.
	if this != "" {
		b.Emit(bytecode.This, this)

		// If it was by value, make a copy of that so the function can't
		// modify the actual value.
		if byValue {
			b.Emit(bytecode.Load, "new")
			b.Emit(bytecode.Load, this)
			b.Emit(bytecode.Call, 1)
			b.Emit(bytecode.Store, this)
		}
	}

	// Generate the parameter assignments. These are extracted from the automatic
	// array named __args which is generated as part of the bytecode function call.
	for n, p := range parameters {

		// is this the end of the fixed list? If so, emit the instruction that scoops
		// up the remaining arguments and stores them as an array value.
		//
		// Otherwise, generate code to extract the argument value by index number.
		if p.kind == datatypes.VarArgs {
			b.Emit(bytecode.GetVarArgs, n)
		} else {
			b.Emit(bytecode.Load, "__args")
			b.Emit(bytecode.Push, n)
			b.Emit(bytecode.LoadIndex)
		}

		// If this argumnet is not interface{} or a variable argument item,
		// generaet code to validate/coerce the value to a given type.
		if p.kind != datatypes.UndefinedType && p.kind != datatypes.VarArgs {
			b.Emit(bytecode.RequiredType, p.kind)
		}
		// Generate code to store the value on top of the stack into the local
		// symbol for the parameter name.
		b.Emit(bytecode.SymbolCreate, p.name)
		b.Emit(bytecode.Store, p.name)
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext("(")
	returnValueCount := 0
	wasVoid := false

	// Loop over the (possibly singular) return type specification
	for {
		coercion := bytecode.New(fmt.Sprintf("%s return item %d", fname, returnValueCount))
		if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
			coercion.Emit(bytecode.Coerce, datatypes.ArrayType)
			c.t.Advance(2)
		} else {
			if c.t.Peek(1) == "{}" {
				coercion.Emit(bytecode.Coerce, datatypes.StructType)
				c.t.Advance(1)
			} else {
				switch c.t.Peek(1) {
				// Start of block means no more types here.
				case "{":
					wasVoid = true

				case "error":
					c.t.Advance(1)
					coercion.Emit(bytecode.Coerce, datatypes.ErrorType)

				case "chan":
					coercion.Emit(bytecode.Coerce, datatypes.ChanType)
					c.t.Advance(1)

				case "int":
					coercion.Emit(bytecode.Coerce, datatypes.IntType)
					c.t.Advance(1)

				case "float", "double":
					coercion.Emit(bytecode.Coerce, datatypes.FloatType)
					c.t.Advance(1)

				case "string":
					coercion.Emit(bytecode.Coerce, datatypes.StringType)
					c.t.Advance(1)

				case "bool":
					coercion.Emit(bytecode.Coerce, datatypes.BoolType)
					c.t.Advance(1)

				case "struct":
					coercion.Emit(bytecode.Coerce, datatypes.StructType)
					c.t.Advance(1)

				case "array":
					coercion.Emit(bytecode.Coerce, datatypes.ArrayType)
					c.t.Advance(1)

				case "interface{}":
					coercion.Emit(bytecode.Coerce, datatypes.UndefinedType)
					c.t.Advance(1)

				case "void":
					// Do nothing, there is no result.
					wasVoid = true
					c.t.Advance(1)

				default:
					return c.NewError(MissingFunctionTypeError)
				}
			}
		}
		if !wasVoid {
			coercions = append(coercions, coercion)
		}
		if c.t.Peek(1) != "," {
			break
		}
		// If we got here, but never had a () around this list, it's an error
		if !hasReturnList {
			return c.NewError(InvalidReturnTypeList)
		}
		c.t.Advance(1)
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(")") {
		return c.NewError(MissingParenthesisError)
	}

	// Now compile a statement or block into the function body. We'll use the
	// current token stream in progress, and the current bytecode. But otherwise we
	// use a new compiler context, so any nested operations do not affect the definition
	// of the function body we're compiling.
	cx := New()
	cx.t = c.t
	cx.b = b
	cx.coerce = coercions
	err := cx.Statement()
	if err != nil {
		return err
	}
	// Generate the deferal invocations, if any, in reverse order
	// that they were defined.
	for i := len(cx.deferQueue) - 1; i >= 0; i = i - 1 {
		cx.b.Emit(bytecode.LocalCall, cx.deferQueue[i])
	}

	// Add trailing return to ensure we close out the scope correctly
	cx.b.Emit(bytecode.Return)

	// If there was a receiver, make sure this function is added to the type structure
	if class != "" {
		c.b.Emit(bytecode.Push, b)
		if c.PackageName != "" {
			c.b.Emit(bytecode.Load, c.PackageName)
			c.b.Emit(bytecode.Member, class)
		} else {
			c.b.Emit(bytecode.Load, class)
		}
		c.b.Emit(bytecode.Push, fname)
		c.b.Emit(bytecode.StoreIndex, true)

		return nil
	}

	// If it was a literal, push the body of the function (really, a bytecode expression
	// of the function code) on the stack. Otherwise, let's store it in the symbol table
	// or package dictionary as appropriate.
	if literal {
		c.b.Emit(bytecode.Push, b)
	} else {
		// Store address of the function, either in the current
		// compiler's symbol table or active package.
		if c.PackageName == "" {
			_ = c.s.SetAlways(fname, b)
		} else {
			_ = c.AddPackageFunction(c.PackageName, fname, b)
		}
	}

	return nil
}
