package debugger

import "github.com/tucats/ego/errors"

func InvokeDebugger(e error) bool {
	if e == nil {
		return false
	}

	if errors.Equals(e, errors.ErrSignalDebugger) {
		return true
	}

	if errors.Equals(e, errors.ErrStepOver) {
		return true
	}

	return false
}
