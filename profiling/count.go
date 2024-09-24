package profiling

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Count adds a profile data point to the profile information. If profiling is not
// active, this function does nothing. The data point is represented by a module name
// and line number.
func Count(module string, line int) {
	if profilingActive {
		if line < 1 || module == "" {
			return
		}

		// Ignore lines generated during import processing.
		if strings.HasPrefix(module, "import ") {
			return
		}

		performanceMux.Lock()
		defer performanceMux.Unlock()

		key := fmt.Sprintf("%s:%d", module, line)
		if counter, found := PerformanceData[key]; found {
			counter.Add(1)
		} else {
			counter = &atomic.Uint32{}
			counter.Store(1)
			PerformanceData[key] = counter
		}
	}
}
