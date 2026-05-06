package compiler

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var testType *data.Type

// initTestType lazily creates the singleton "Testing" struct type that backs the
// implicit "T" variable available in Ego test files. The type has a single
// "description" string field and a set of built-in assertion methods (assert,
// Fail, Nil, NotNil, True, False, Equal, NotEqual). After the first call the
// type is cached in testType and subsequent calls are no-ops.
func initTestType() {
	if testType == nil {
		t := data.TypeDefinition("Testing",
			data.StructureType(data.Field{
				Name: "description",
				Type: data.StringType,
			}))

		// Define the type receiver functions
		t.DefineFunction("assert", nil, TestAssert)
		t.DefineFunction("Fail", nil, TestFail)
		t.DefineFunction("Nil", nil, TestNil)
		t.DefineFunction("NotNil", nil, TestNotNil)
		t.DefineFunction("True", nil, TestTrue)
		t.DefineFunction("False", nil, TestFalse)
		t.DefineFunction("Equal", nil, TestEqual)
		t.DefineFunction("NotEqual", nil, TestNotEqual)

		testType = t
	}
}

// testDirective compiles the "@test description" directive. It may only appear
// when the compiler is running in test mode (the "ego test" command). Each
// @test directive:
//
//  1. Implicitly closes the preceding test (emitting a @pass if one was open).
//  2. Creates a new Testing struct instance with the given description string
//     and stores it in the "T" variable so assertion methods are available.
//  3. Emits code to print "TEST: <description>" to the console and start a
//     timer for the test.
func (c *Compiler) testDirective() error {
	var (
		err             error
		testDescription string
	)

	// If we're not in test mode, this is an invalid use of the @test
	// directive.
	if !c.flags.testMode {
		return c.compileError(errors.ErrWrongMode)
	}

	// Generate an implicit @pass for any test that came before. This
	// also includes a mode check to ensure that the directive is only used
	// in test mode.
	if err := c.TestPass(); err != nil {
		return err
	}

	testDescription = c.t.NextText()
	if testDescription[0] == '"' {
		testDescription, err = strconv.Unquote(testDescription)

		if err != nil {
			return c.compileError(err)
		}
	}

	// Sanity check; the can't be longer than 48 characters or the
	// formatting of the output gets weird. So truncate the string
	// to 48 chars max, including "..." if truncation happens.

	result := strings.Builder{}

	descLen := 0
	for range testDescription {
		descLen++
	}

	if descLen > 48 {
		for i, ch := range testDescription {
			if i >= 46 {
				result.WriteString("...")

				break
			}

			result.WriteRune(ch)
		}

		testDescription = result.String()
	}

	// Create an instance of the object, and assign the value to
	// the data field.
	initTestType()

	test := data.NewStruct(testType)
	test.SetAlways("description", testDescription)

	padSize := 50 - len(testDescription)
	if padSize < 0 {
		padSize = 0
	}

	pad := strings.Repeat(" ", padSize)

	c.b.Emit(bytecode.Push, test)

	c.b.Emit(bytecode.StoreAlways, "T")

	// Generate code to report that the test is starting.
	if err := c.ReferenceSymbol("T"); err != nil {
		return err
	}

	c.b.Emit(bytecode.Console, false)
	c.b.Emit(bytecode.Push, "TEST: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "description")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, pad)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 0)
	c.b.Emit(bytecode.PushTest)

	c.DefineGlobalSymbol("T")

	return c.ReferenceSymbol("T")
}

// Helper function to get the test name.
func getTestName(s *symbols.SymbolTable) string {
	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if testStruct, ok := m.(*data.Struct); ok {
			if nameString, ok := testStruct.Get("description"); ok {
				name = data.String(nameString)
			}
		}
	}

	return name
}

// TestAssert implements the T.assert() function.
func TestAssert(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In("assert")
	}

	// The argument could be an array with the boolean value and the
	// messaging string, or it might just be the boolean.
	if array, ok := args.Get(0).([]any); ok && len(array) == 2 {
		b, err := data.Bool(array[0])
		if err != nil {
			return nil, err
		}

		if !b {
			msg := data.String(array[1])

			fmt.Println()

			return nil, errors.ErrAssert.In(getTestName(s)).Context(msg)
		}

		return true, nil
	}

	// Just the boolean; the string is optionally in the second
	// argument.
	b, err := data.Bool(args.Get(0))
	if err != nil {
		return nil, err
	}

	if !b {
		msg := errors.ErrTestingAssert

		if args.Len() > 1 {
			msg = msg.Context(args.Get(1))
		} else {
			msg = msg.Context(getTestName(s))
		}

		fmt.Println()

		return nil, msg
	}

	return true, nil
}

// TestFail implements the T.fail() function which generates a fatal
// error.
func TestFail(s *symbols.SymbolTable, args data.List) (any, error) {
	msg := "T.fail()"

	if args.Len() == 1 {
		msg = data.String(args.Get(0))
	}

	return nil, errors.Message(msg).In(getTestName(s))
}

// TestNil implements the T.Nil() function.
func TestNil(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	isNil := args.Get(0) == nil
	if e, ok := args.Get(0).(error); ok {
		isNil = errors.Nil(e)
	}

	if args.Len() == 2 {
		return []any{isNil, data.String(args.Get(1))}, nil
	}

	return isNil, nil
}

// TestNotNil implements the T.NotNil() function.
func TestNotNil(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	isNil := args.Get(0) == nil
	if e, ok := args.Get(0).(error); ok {
		isNil = errors.Nil(e)
	}

	if args.Len() == 2 {
		return []any{!isNil, data.String(args.Get(1))}, nil
	}

	return !isNil, nil
}

// TestTrue implements the T.True() function.
func TestTrue(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	if args.Len() == 2 {
		b, err := data.Bool(args.Get(0))
		if err != nil {
			return nil, err
		}

		return []any{b, data.String(args.Get(1))}, nil
	}

	v, err := data.Bool(args.Get(0))

	return v, err
}

// TestFalse implements the T.False() function.
func TestFalse(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	if args.Len() == 2 {
		b, err := data.Bool(args.Get(0))
		if err != nil {
			return nil, err
		}

		return []any{!b, data.String(args.Get(1))}, nil
	}

	b, err := data.Bool(args.Get(0))

	return !b, err
}

// TestEqual implements the T.Equal() function.
func TestEqual(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 2 || args.Len() > 3 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	b := reflect.DeepEqual(args.Get(0), args.Get(1))

	if a1, ok := args.Get(0).([]any); ok {
		if a2, ok := args.Get(1).(*data.Array); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args.Get(1).([]any); ok {
		if a2, ok := args.Get(0).(*data.Array); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args.Get(0).(*data.Array); ok {
		if a2, ok := args.Get(1).(*data.Array); ok {
			b = a1.DeepEqual(a2)
		}
	}

	if args.Len() == 3 {
		return []any{b, data.String(args.Get(2))}, nil
	}

	return b, nil
}

// TestNotEqual implements the T.NotEqual() function.
func TestNotEqual(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 2 || args.Len() > 3 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	b, err := TestEqual(s, args)
	if err == nil {
		b, err := data.Bool(b)

		return !b, err
	}

	return nil, err
}

// Assert implements the @assert directive.
func (c *Compiler) Assert() error {
	_ = c.modeCheck("test")

	if c.t.IsNext(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.ReferenceSymbol("T"); err != nil {
		return err
	}

	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("assert"))
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "assert")

	argCount := 1

	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Call, argCount)
	c.b.Emit(bytecode.DropToMarker)

	return nil
}

// Fail implements the @fail directive. Note that unlike other test-related
// directives, @fail can be used even when not in "test" mode.
func (c *Compiler) Fail() error {
	if !c.t.EndOfStatement() {
		if err := c.emitExpression(); err != nil {
			return err
		}
	} else {
		c.b.Emit(bytecode.Push, "@fail error signal")
	}

	c.b.Emit(bytecode.TryFlush)
	c.b.Emit(bytecode.Signal, nil)

	return nil
}

// TestPass implements the @pass directive.
func (c *Compiler) TestPass() error {
	_ = c.modeCheck("test")
	c.b.Emit(bytecode.RunDefers)

	here := c.b.Mark()
	c.b.Emit(bytecode.PopTest, 0)

	// Increment the test counter variable
	c.b.Emit(bytecode.Load, "_testcount")
	c.b.Emit(bytecode.Push, 1)
	c.b.Emit(bytecode.Add)
	c.b.Emit(bytecode.StoreGlobal, "_testcount")

	// Print out the status result
	c.b.Emit(bytecode.Push, "(PASS)  ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 1)
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.Push, "<none>")
	c.b.Emit(bytecode.Equal)

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Say, true)
	done := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	c.b.SetAddressHere(branch)
	c.b.Emit(bytecode.Drop) // timer value string

	c.b.SetAddressHere(done)
	c.b.SetAddressHere(here)

	return nil
}

// Fail implements the @file directive.
func (c *Compiler) File() error {
	fileName := c.t.Next().Spelling()
	c.sourceFile = fileName

	c.b.Emit(bytecode.InFile, fileName)

	return nil
}
