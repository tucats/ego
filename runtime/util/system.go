package util

import (
	"runtime"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// getMode implements the util.getMode() function which reports the runtime mode.
func getMode(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	m, ok := symbols.Get(defs.ModeVariable)
	if !ok {
		m = "run"
	}

	return m, nil
}

func getMemoryStats(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		m runtime.MemStats
	)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	runtime.ReadMemStats(&m)

	return data.NewStructOfTypeFromMap(UtilMemoryType, map[string]interface{}{
		"Time":    time.Now().Format("Mon Jan 2 2006 15:04:05 MST"),
		"Current": bToMb(m.Alloc),
		"Total":   bToMb(m.TotalAlloc),
		"System":  bToMb(m.Sys),
		"GC":      int(m.NumGC),
	}), nil
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
