package debugger

import "github.com/tucats/ego/errors"

func InvokeDebugger(e *errors.EgoError) bool {
	if e == nil {
		return false
	}

	if e.Is(errors.ErrSignalDebugger) {
		return true
	}

	if e.Is(errors.ErrStepOver) {
		return true
	}

	return false
}
