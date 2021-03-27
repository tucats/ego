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
	"github.com/tucats/ego/util"
)

// testDirective compiles the @test directive.
func (c *Compiler) testDirective() *errors.EgoError {
	_ = c.modeCheck("test", true)

	testDescription := c.t.Next()
	if testDescription[:1] == "\"" {
		testDescription = testDescription[1 : len(testDescription)-1]
	}

	test := map[string]interface{}{}
	test["assert"] = TestAssert
	test["fail"] = TestFail
	test["isType"] = TestIsType
	test["Nil"] = TestNil
	test["NotNil"] = TestNotNil
	test["True"] = TestTrue
	test["False"] = TestFalse
	test["Equal"] = TestEqual
	test["NotEqual"] = TestNotEqual
	test["description"] = testDescription

	padSize := 50 - len(testDescription)
	if padSize < 0 {
		padSize = 0
	}

	pad := strings.Repeat(" ", padSize)

	// Generate code to create a Test object. Make a native struct
	// if they are in play...
	//if bytecode.NativeStructures {
	//	c.b.Emit(bytecode.Push, datatypes.NewStructFromMap(test))
	// } else {
	c.b.Emit(bytecode.Push, test)
	//}

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

// TestAssert implements the T.assert() function.
func TestAssert(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ArgumentCountError).In("assert")
	}

	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if structMap, ok := m.(map[string]interface{}); ok {
			if nameString, ok := structMap["description"]; ok {
				name = util.GetString(nameString)
			}
		}
	}

	// The argument could be an array with the boolean value and the
	// messaging string, or it might just be the boolean.
	if array, ok := args[0].([]interface{}); ok && len(array) == 2 {
		b := util.GetBool(array[0])
		if !b {
			msg := util.GetString(array[1])

			fmt.Println()

			return nil, errors.New(errors.AssertError).In(name).Context(msg)
		} else {
			return true, nil
		}
	}

	// Just the boolean; the string is optionally in the second
	// argument.
	b := util.GetBool(args[0])
	if !b {
		msg := errors.New(errors.TestingAssertError)

		if len(args) > 1 {
			msg = msg.Context(args[1])
		}

		fmt.Println()

		return nil, msg
	}

	return true, nil
}

// TestIsType implements the T.assert() function.
func TestIsType(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ArgumentCountError).In("IsType()")
	}

	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if structMap, ok := m.(map[string]interface{}); ok {
			if nameString, ok := structMap["name"]; ok {
				name = util.GetString(nameString)
			}
		}
	}

	// Use the Type() function to get a string representation of the type
	got, _ := functions.Type(s, args[0:1])
	expected := util.GetString(args[1])

	b := (expected == got)
	if !b {
		msg := fmt.Sprintf("T.isType(\"%s\" != \"%s\") failure", got, expected)
		if len(args) > 2 {
			msg = util.GetString(args[2])
		}

		return nil, errors.NewMessage(msg).Context(name)
	}

	return true, nil
}

// TestFail implements the T.fail() function which generates a fatal
// error.
func TestFail(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	msg := "T.fail()"

	if len(args) == 1 {
		msg = util.GetString(args[0])
	}

	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if structMap, ok := m.(map[string]interface{}); ok {
			if nameString, ok := structMap["description"]; ok {
				name = util.GetString(nameString)
			}
		}
	}

	return nil, errors.NewMessage(msg).Context(name)
}

// TestNil implements the T.Nil() function.
func TestNil(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ArgumentCountError).In("Nil()")
	}

	isNil := args[0] == nil
	if e, ok := args[0].(*errors.EgoError); ok {
		isNil = errors.Nil(e)
	}

	if len(args) == 2 {
		return []interface{}{isNil, util.GetString(args[1])}, nil
	}

	return isNil, nil
}

// TestNotNil implements the T.NotNil() function.
func TestNotNil(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ArgumentCountError).In("NotNil")
	}

	isNil := args[0] == nil
	if e, ok := args[0].(*errors.EgoError); ok {
		isNil = errors.Nil(e)
	}

	if len(args) == 2 {
		return []interface{}{!isNil, util.GetString(args[1])}, nil
	}

	return !isNil, nil
}

// TestTrue implements the T.True() function.
func TestTrue(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ArgumentCountError).In("True()")
	}

	if len(args) == 2 {
		return []interface{}{util.GetBool(args[0]), util.GetString(args[1])}, nil
	}

	return util.GetBool(args[0]), nil
}

// TestFalse implements the T.False() function.
func TestFalse(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ArgumentCountError).In("False()")
	}

	if len(args) == 2 {
		return []interface{}{!util.GetBool(args[0]), util.GetString(args[1])}, nil
	}

	return !util.GetBool(args[0]), nil
}

// TestEqual implements the T.Equal() function.
func TestEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ArgumentCountError).In("Equal()")
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
		return []interface{}{b, util.GetString(args[2])}, nil
	}

	return b, nil
}

// TestNotEqual implements the T.NotEqual() function.
func TestNotEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 2 || len(args) > 3 {
		return nil, errors.New(errors.ArgumentCountError).In("NoEqual()")
	}

	b, err := TestEqual(s, args)
	if errors.Nil(err) {
		return !util.GetBool(b), nil
	}

	return nil, err
}

// Assert implements the @assert directive.
func (c *Compiler) Assert() *errors.EgoError {
	_ = c.modeCheck("test", true)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "assert")

	argCount := 1

	code, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(code)
	c.b.Emit(bytecode.Call, argCount)

	return nil
}

// Fail implements the @fail directive.
func (c *Compiler) Fail() *errors.EgoError {
	_ = c.modeCheck("test", true)

	next := c.t.Peek(1)
	if next != "@" && next != ";" && next != tokenizer.EndOfTokens {
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
