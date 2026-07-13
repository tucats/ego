package bytecode

// entryPoint_test.go tests the EntryPoint/EntryPointExit opcode pair emitted
// by (*compiler.Compiler).entrypointDirective for the synthetic "@entrypoint"
// directive appended to every program run with "ego run" or "ego --entry-point".
//
// Regression coverage for a bug where the compiler always called the Ego
// equivalent of os.Exit(0) after the entry point function returned, silently
// discarding whatever value the function (e.g. "func main() int { return 42 }")
// actually returned -- so a program's exit code was always 0 regardless of
// what main() returned, contradicting docs/LANGUAGE.md's documented behavior
// that a non-zero return from main() has the same effect as calling os.Exit()
// with that value.
//
// The fix cannot collect the entry point's return value synchronously inside
// entryPointByteCode, because calling a *ByteCode function (callBytecodeFunction)
// only pushes a new call frame and returns -- the callee's body actually runs
// later, as the run loop continues past the EntryPoint instruction. So the
// compiler now pushes a StackMarker before EntryPoint, and a separate
// EntryPointExit instruction (emitted right after EntryPoint) collects
// whatever is above that marker once the callee has actually finished.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// buildEntryPointProgram constructs the bytecode.Context for a minimal
// program equivalent to what the compiler emits for:
//
//	func main() <returns>
//	@entrypoint
//
// where mainInstructions is the body of "main" (assumed to leave its
// declared number of return values, if any, on the stack via a Return
// instruction, exactly as compiled Ego code does).
func buildEntryPointProgram(mainInstructions func(bc *ByteCode)) *Context {
	root := symbols.NewRootSymbolTable("test root")
	local := symbols.NewChildSymbolTable("test local", root)

	mainBC := New("main")
	mainInstructions(mainBC)
	local.SetAlways("main", mainBC)

	wrapper := New("wrapper")
	wrapper.Emit(Push, NewStackMarker("entrypoint"))
	wrapper.Emit(Push, "main")
	wrapper.Emit(Dup)
	wrapper.Emit(StoreAlways, "__main")
	wrapper.Emit(EntryPoint)
	wrapper.Emit(EntryPointExit)

	return NewContext(local, wrapper)
}

func Test_EntryPointExit_UsesMainReturnValueAsExitCode(t *testing.T) {
	ctx := buildEntryPointProgram(func(bc *ByteCode) {
		bc.Emit(Push, 42)
		bc.Emit(Return, 1)
	})

	err := ctx.Run()
	if !errors.Equals(err, errors.ErrExit) {
		t.Fatalf("expected ErrExit, got %v (%T)", err, err)
	}

	egoErr, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	if got := egoErr.GetContext(); got != "42" {
		t.Errorf("expected exit code context %q, got %q", "42", got)
	}
}

func Test_EntryPointExit_ZeroReturnValueStillExits(t *testing.T) {
	ctx := buildEntryPointProgram(func(bc *ByteCode) {
		bc.Emit(Push, 0)
		bc.Emit(Return, 1)
	})

	err := ctx.Run()
	if !errors.Equals(err, errors.ErrExit) {
		t.Fatalf("expected ErrExit, got %v (%T)", err, err)
	}

	egoErr, _ := err.(*errors.Error)
	if got := egoErr.GetContext(); got != "0" {
		t.Errorf("expected exit code context %q, got %q", "0", got)
	}
}

func Test_EntryPointExit_NoReturnValueDoesNotExit(t *testing.T) {
	ctx := buildEntryPointProgram(func(bc *ByteCode) {
		bc.Emit(Return, 0)
	})

	err := ctx.Run()
	if err != nil {
		t.Fatalf("expected no error (natural completion), got %v", err)
	}
}
