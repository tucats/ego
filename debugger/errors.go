package debugger

import (
	"errors"
)

const (
	InvalidBreakClauseError = "invalid break clause"
)

var SignalDebugger = errors.New("signal")
var StepOver = errors.New("step-over")
var Stop = errors.New("stop")

func InvokeDebugger(e error) bool {
	if e == nil {
		return false
	}
	text := e.Error()
	if text == SignalDebugger.Error() {
		return true
	}
	if text == StepOver.Error() {
		return true
	}

	return false
}
