package util

import (
	"runtime"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// Mode implements the util.Mode() function which reports the runtime mode.
func Mode(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	m, ok := symbols.Get(defs.ModeVariable)
	if !ok {
		m = "run"
	}

	return m, nil
}

func Memory(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var m runtime.MemStats

	result := map[string]interface{}{}

	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats

	result["Time"] = time.Now().Format("Mon Jan 2 2006 15:04:05 MST")
	result["Current"] = bToMb(m.Alloc)
	result["Total"] = bToMb(m.TotalAlloc)
	result["System"] = bToMb(m.Sys)
	result["GC"] = int(m.NumGC)
	result[data.TypeMDKey] = memoryTypeDef

	return data.NewStructFromMap(result), nil
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
