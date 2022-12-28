package compiler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var testType *datatypes.Type

// If it hasn't already been defined, define the type of the testing object.
func initTestType() {
	if testType == nil {
		t := datatypes.TypeDefinition("Testing",
			datatypes.Structure(datatypes.Field{
				Name: "description",
				Type: &datatypes.StringType,
			}))

		// Define the type receiver functions
		t.DefineFunction("assert", TestAssert)
		t.DefineFunction("fail", TestFail)
		t.DefineFunction("isType", TestIsType)
		t.DefineFunction("Nil", TestNil)
		t.DefineFunction("NotNil", TestNotNil)
		t.DefineFunction("True", TestTrue)
		t.DefineFunction("False", TestFalse)
		t.DefineFunction("Equal", TestEqual)
		t.DefineFunction("NotEqual", TestNotEqual)

		testType = t
	}
}

// testDirective compiles the @test directive.
func (c *Compiler) testDirective() *errors.EgoError {
	_ = c.modeCheck("test", true)

	testDescription := c.t.Next()
	if testDescription[:1] == "\"" {
		testDescription = testDescription[1 : len(testDescription)-1]
	}

	// Create an instance of the object, and assign the value to
	// the data field.
	initTestType()

	test := datatypes.NewStruct(testType)
	test.SetAlways("description", testDescription)

	padSize := 50 - len(testDescription)
	if padSize < 0 {
		padSize = 0
	}

	pad := strings.Repeat(" ", padSize)

	c.b.Emit(bytecode.Push, test)

	c.b.Emit(bytecode.StoreAlways, "T")

	// Generate code to report that the test is starting.
	c.b.Emit(bytecode.Push, "TEST: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "description")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, pad)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 0)

	return nil
}

// Helper function to get the test name.
func getTestName(s *symbols.SymbolTable) string {
	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if testStruct, ok := m.(*datatypes.EgoStruct); ok {
			if nameString, ok := testStruct.Get("description"); ok {
				name = datatypes.GetString(nameString)
			}
		}
	}

	return name
}

// TestAssert implements the T.assert() function.
func TestAssert(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount).In("assert")
	}

	// The argument could be an array with the boolean value and the
	// messaging string, or it might just be the boolean.
	if array, ok := args[0].([]interface{}); ok && len(array) == 2 {
		b := datatypes.GetBool(array[0])
		if !b {
			msg := datatypes.GetString(array[1])

			fmt.Println()

			return nil, errors.New(errors.ErrAssert).In(getTestName(s)).Context(msg)
		}

		return true, nil
	}

	// Just the boolean; the string is optionally in the second
	// argument.
	b := datatypes.GetBool(args[0])
	if !b {
		msg := errors.New(errors.ErrTestingAssert)

		if len(args) > 1 {
			msg = msg.Context(args[1])
		} else {
			msg = msg.Context(getTestName(s))
		}

		fmt.Println()

		return nil, msg
	}

	return true, nil
}

// TestIsType implements the T.type() function.
func TestIsType(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ErrArgumentCount).In("IsType()")
	}

	// Use the Type() function to get a string representation of the type
	got, _ := functions.Type(s, args[0:1])
	expected := datatypes.GetString(args[1])

	b := (expected == got)
	if !b {
		msg := fmt.Sprintf("T.isType(\"%s\" != \"%s\") failure", got, expected)
		if len(args) > 2 {
			msg = datatypes.GetString(args[2])
		}

		return nil, errors.NewMessage(msg).In(getTestName(s))
	}

	return true, nil
}

// TestFail implements the T.fail() function which generates a fatal
// error.
func TestFail(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	msg := "T.fail()"

	if len(args) == 1 {
		msg = datatypes.GetString(args[0])
	}

	return nil, errors.NewMessage(msg).In(getTestName(s))
}

// TestNil implements the T.Nil() function.
func TestNil(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	isNil := args[0] == nil
	if e, ok := args[0].(*errors.EgoError); ok {
		isNil = errors.Nil(e)
	}

	if len(args) == 2 {
		return []interface{}{isNil, datatypes.GetString(args[1])}, nil
	}

	return isNil, nil
}

// TestNotNil implements the T.NotNil() function.
func TestNotNil(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	isNil := args[0] == nil
	if e, ok := args[0].(*errors.EgoError); ok {
		isNil = errors.Nil(e)
	}

	if len(args) == 2 {
		return []interface{}{!isNil, datatypes.GetString(args[1])}, nil
	}

	return !isNil, nil
}

// TestTrue implements the T.True() function.
func TestTrue(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	if len(args) == 2 {
		return []interface{}{datatypes.GetBool(args[0]), datatypes.GetString(args[1])}, nil
	}

	return datatypes.GetBool(args[0]), nil
}

// TestFalse implements the T.False() function.
func TestFalse(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	if len(args) == 2 {
		return []interface{}{!datatypes.GetBool(args[0]), datatypes.GetString(args[1])}, nil
	}

	return !datatypes.GetBool(args[0]), nil
}

// TestEqual implements the T.Equal() function.
func TestEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	b := reflect.DeepEqual(args[0], args[1])

	if a1, ok := args[0].([]interface{}); ok {
		if a2, ok := args[1].(*datatypes.EgoArray); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args[1].([]interface{}); ok {
		if a2, ok := args[0].(*datatypes.EgoArray); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args[0].(*datatypes.EgoArray); ok {
		if a2, ok := args[1].(*datatypes.EgoArray); ok {
			b = a1.DeepEqual(a2)
		}
	}

	if len(args) == 3 {
		return []interface{}{b, datatypes.GetString(args[2])}, nil
	}

	if !b {
		fmt.Printf("DEBUG: args[0] = %v\n       args[1] = %v\n", args[0], args[1])
	}

	return b, nil
}

// TestNotEqual implements the T.NotEqual() function.
func TestNotEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ErrArgumentCount).In(getTestName(s))
	}

	b, err := TestEqual(s, args)
	if errors.Nil(err) {
		return !datatypes.GetBool(b), nil
	}

	return nil, err
}

// Assert implements the @assert directive.
func (c *Compiler) Assert() *errors.EgoError {
	_ = c.modeCheck("test", true)
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("assert"))
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "assert")

	argCount := 1

	code, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(code)
	c.b.Emit(bytecode.Call, argCount)
	c.b.Emit(bytecode.DropToMarker)

	return nil
}

// Fail implements the @fail directive.
func (c *Compiler) Fail() *errors.EgoError {
	_ = c.modeCheck("test", true)

	next := c.t.Peek(1)
	if next != "@" && next != tokenizer.SemicolonToken && next != tokenizer.EndOfTokens {
		code, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(code)
	} else {
		c.b.Emit(bytecode.Push, "@fail error signal")
	}

	c.b.Emit(bytecode.Panic, true)

	return nil
}

// TestPass implements the @pass directive.
func (c *Compiler) TestPass() *errors.EgoError {
	_ = c.modeCheck("test", true)
	c.b.Emit(bytecode.Push, "(PASS)  ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 1)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Say, true)

	return nil
}
