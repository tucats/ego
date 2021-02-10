package debugger

import "github.com/tucats/ego/errors"

func InvokeDebugger(e *errors.EgoError) bool {
	if e == nil {
		return false
	}

	if e.Is(errors.SignalDebugger) {
		return true
	}

	if e.Is(errors.StepOver) {
		return true
	}

	return false
}
