package util

import (
	"runtime"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// getMode implements the util.Mode() function, which returns the current execution
// mode of the Ego runtime (e.g., "run", "test", "server"). The mode is stored in
// the symbol table under the reserved key defs.ModeVariable. If no mode has been
// set, the default "run" is returned.
func getMode(symbols *symbols.SymbolTable, args data.List) (any, error) {
	m, ok := symbols.Get(defs.ModeVariable)
	if !ok {
		m = "run"
	}

	return m, nil
}

// getMemoryStats implements the util.Memory() function. It reads the Go runtime's
// memory allocator statistics and returns them as a UtilMemoryType struct. All
// byte counts are converted to megabytes for readability. The GC field counts the
// number of completed garbage-collection cycles since the process started.
func getMemoryStats(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		m runtime.MemStats
	)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	runtime.ReadMemStats(&m)

	return data.NewStructOfTypeFromMap(UtilMemoryType, map[string]any{
		"Time":    time.Now().Format("Mon Jan 2 2006 15:04:05 MST"),
		"Current": bToMb(m.Alloc),
		"Total":   bToMb(m.TotalAlloc),
		"System":  bToMb(m.Sys),
		"GC":      int(m.NumGC),
	}), nil
}

// bToMb converts a byte count to megabytes (1 MB = 1024 * 1024 bytes).
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
