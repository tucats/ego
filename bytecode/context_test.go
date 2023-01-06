package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestNewContext(t *testing.T) {
	s := symbols.NewSymbolTable("context test")
	b := New("context test")

	s.SetAlways("foo", 1)

	b.Emit(AtLine, 100)
	b.Emit(Load, "foo")
	b.Emit(Push, 2)
	b.Emit(Add)
	b.Emit(Return, 1)

	// Create a new context to test and validate the initial state
	c := NewContext(s, b)

	if c.symbols != s {
		t.Error("Symbol table not set")
	}

	if c.bc != b {
		t.Error("bytecode not set")
	}

	if len(c.stack) != initialStackSize {
		t.Error("stack not sized correctly")
	}

	if c.stackPointer != 0 {
		t.Error("stack not empty before run")
	}

	e := c.constantSet("xyzzy", "frobozz")
	if e != nil {
		t.Errorf("Unexpected constant set error: %v", e)
	}

	if !c.symbolIsConstant("xyzzy") {
		t.Error("symbol not seen as constant")
	}

	if c.symbolIsConstant("zork") {
		t.Error("unknown symbol was constant")
	}

	r, found := c.symbolGet("xyzzy")
	if !found {
		t.Error("Failed to find previously created constant")
	}

	if !reflect.DeepEqual(r, "frobozz") {
		t.Errorf("Retrieval of constant has wrong value: %v", r)
	}
	// Change some of the attributes of the context and validate
	if !c.fullSymbolScope {
		t.Error("failed to initialize full symbol scope")
	}

	c.SetFullSymbolScope(false)

	if c.fullSymbolScope {
		t.Error("failed to clear full symbol scope")
	}

	c.SetDebug(true)

	if !c.debugging {
		t.Error("Failed to set debugging flag")
	}

	c.SetDebug(false)

	if c.debugging {
		t.Error("Failed to clear debugging flag")
	}

	c.EnableConsoleOutput(false)

	if c.output == nil {
		t.Error("Failed to set non-empty console output")
	}

	c.output.WriteString("foobar")

	o := c.GetOutput()
	if o != "foobar" {
		t.Errorf("Incorrect captured console text: %v", o)
	}

	c.EnableConsoleOutput(true)

	if c.output != nil {
		t.Error("Failed to clear console output state")
	}

	// Now run the short segment of bytecode, and see what the ending
	// context state looks like.
	e = c.Run()
	if !errors.Nil(e) && e.Error() != errors.ErrStop.Error() {
		t.Errorf("Failed to run bytecode: %v", e)
	}

	_, e = c.Pop()

	if errors.Nil(e) {
		t.Errorf("Expected stack underflow not found")
	}

	r = c.Result()
	if !reflect.DeepEqual(r, 3) {
		t.Errorf("Wrong final result: %v", r)
	}

	if c.stackPointer != 0 {
		t.Error("stack not empty after run")
	}

	if c.GetLine() != 100 {
		t.Errorf("Incorrect line number: %v", c.GetLine())
	}
}
