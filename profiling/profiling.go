package profiling

import (
	"sync"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

const (
	StartAction int = iota
	StopAction
	ReportAction
)

var (
	PerformanceData map[string]*atomic.Uint32 = make(map[string]*atomic.Uint32)
	profilingActive bool
	performanceMux  sync.Mutex
)

func Profile(action int) error {
	performanceMux.Lock()
	defer performanceMux.Unlock()

	switch action {
	case StartAction:
		if profilingActive {
			ui.Log(ui.InternalLogger, "runtime.profile.active")
		}

		profilingActive = true
		PerformanceData = make(map[string]*atomic.Uint32)

		ui.Log(ui.InternalLogger, "runtime.profile.started")

		return nil

	case StopAction:
		if !profilingActive {
			ui.Log(ui.InternalLogger, "runtime.profile.inactive")
		}

		ui.Log(ui.InternalLogger, "runtime.profile.stopped")

		profilingActive = false

	case ReportAction:
		return PrintProfileReport()

	default:
		return errors.ErrInvalidProfileAction.Context(action)
	}

	return nil
}
