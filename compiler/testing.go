package compiler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Test compiles the @test directive.
func (c *Compiler) Test() error {
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

	_ = c.s.SetAlways("T", test)

	// Generate code to update the description (this is required for the
	// cases of the ego test command running multiple tests as a single
	// stream)
	c.b.Emit(bytecode.Push, testDescription)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Push, "description")
	c.b.Emit(bytecode.StoreIndex)

	// Generate code to report that the test is starting.
	c.b.Emit(bytecode.Push, "TEST: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Push, "description")
	c.b.Emit(bytecode.Member)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, pad)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 0)

	//c.b.Emit(bytecode.Newline)

	return nil
}

// TestAssert implements the T.assert() function.
func TestAssert(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, functions.NewError("assert", functions.ArgumentCountError)
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

			return nil, fmt.Errorf("@assert, %s in %s", msg, name)
		} else {
			return true, nil
		}
	}

	// Just the boolean; the string is optionally in the second
	// argument.
	b := util.GetBool(args[0])
	if !b {
		msg := TestingAssertError

		if len(args) > 1 {
			msg = util.GetString(args[1])
		}

		fmt.Println()

		return nil, fmt.Errorf("%s in %s", msg, name)
	}

	return true, nil
}

// TestIsType implements the T.assert() function.
func TestIsType(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, functions.NewError("istype", functions.ArgumentCountError)
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

		return nil, fmt.Errorf("%s in %s", msg, name)
	}

	return true, nil
}

// TestFail implements the T.fail() function which generates a fatal
// error.
func TestFail(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	msg := "T.fail()"

	if len(args) == 1 {
		msg = util.GetString(args[0])
	}

	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		fmt.Printf("DEBUG: found testing package\n")

		if structMap, ok := m.(map[string]interface{}); ok {
			fmt.Printf("DEBUG: found map\n")

			if nameString, ok := structMap["description"]; ok {
				fmt.Printf("DEBUG: found name member\n")

				name = util.GetString(nameString)
			}
		}
	}

	return nil, fmt.Errorf("%s in %s", msg, name)
}

// TestNil implements the T.Nil() function.
func TestNil(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, functions.NewError("Nil", functions.ArgumentCountError)
	}

	if len(args) == 2 {
		return []interface{}{args[0] == nil, util.GetString(args[1])}, nil
	}

	return args[0] == nil, nil
}

// TestNotNil implements the T.NotNil() function.
func TestNotNil(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, functions.NewError("NotNil", functions.ArgumentCountError)
	}

	if len(args) == 2 {
		return []interface{}{args[0] != nil, util.GetString(args[1])}, nil
	}

	return args[0] != nil, nil
}

// TestTrue implements the T.True() function.
func TestTrue(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, functions.NewError("True", functions.ArgumentCountError)
	}

	if len(args) == 2 {
		return []interface{}{util.GetBool(args[0]), util.GetString(args[1])}, nil
	}

	return util.GetBool(args[0]), nil
}

// TestFalse implements the T.False() function.
func TestFalse(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, functions.NewError("False", functions.ArgumentCountError)
	}

	if len(args) == 2 {
		return []interface{}{!util.GetBool(args[0]), util.GetString(args[1])}, nil
	}

	return !util.GetBool(args[0]), nil
}

// TestEqual implements the T.Equal() function.
func TestEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, functions.NewError("Equal", functions.ArgumentCountError)
	}

	b := reflect.DeepEqual(args[0], args[1])

	if len(args) == 3 {
		return []interface{}{b, util.GetString(args[2])}, nil
	}

	return b, nil
}

// TestNotEqual implements the T.NotEqual() function.
func TestNotEqual(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, functions.NewError("NotEqual", functions.ArgumentCountError)
	}

	b := !reflect.DeepEqual(args[0], args[1])

	if len(args) == 3 {
		return []interface{}{b, util.GetString(args[2])}, nil
	}

	return b, nil
}

// Assert implements the @assert directive.
func (c *Compiler) Assert() error {
	_ = c.modeCheck("test", true)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Push, "assert")
	c.b.Emit(bytecode.Member)

	argCount := 1

	code, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(code)
	c.b.Emit(bytecode.Call, argCount)

	return nil
}

// Fail implements the @fail directive.
func (c *Compiler) Fail() error {
	_ = c.modeCheck("test", true)

	next := c.t.Peek(1)
	if next != "@" && next != ";" && next != tokenizer.EndOfTokens {
		code, err := c.Expression()
		if err != nil {
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
func (c *Compiler) TestPass() error {
	_ = c.modeCheck("test", true)
	c.b.Emit(bytecode.Push, "(PASS)  ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 1)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Newline)

	return nil
}
