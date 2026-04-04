package admin

import (
	"net/http"
	"runtime"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// GetMemoryHandler is the HTTP handler for GET /admin/memory. It reports the
// server's current memory usage by reading live statistics from the Go runtime.
//
// The response includes several distinct memory measurements so the caller can
// understand both how much memory the server has allocated over its lifetime
// and how much it is actively using right now.
func GetMemoryHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// runtime.MemStats is a struct defined by the Go standard library that
	// contains dozens of memory counters maintained by the runtime.  We
	// declare a pointer to a zero-valued MemStats and then pass it to
	// runtime.ReadMemStats, which fills it with the current values.
	//
	// Note: runtime.ReadMemStats triggers a stop-the-world pause to collect
	// consistent numbers, so it is slightly expensive.  For an admin endpoint
	// polled occasionally by a dashboard this is perfectly acceptable.
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	// Build the response, copying the specific fields the caller cares about
	// from the raw MemStats struct into the API-facing defs.MemoryResponse type.
	//
	// Field meanings:
	//   TotalAlloc  — cumulative bytes allocated since the server started
	//                 (never decreases; useful for spotting allocation-heavy workloads)
	//   HeapInuse   — bytes in heap spans that are currently in use
	//                 (the closest approximation to "live heap size")
	//   Sys         — total bytes of memory obtained from the OS across all
	//                 arenas (heap, stack, metadata, etc.)
	//   StackInuse  — bytes used by goroutine stacks right now
	//   HeapObjects — number of live heap objects (a proxy for GC pressure)
	//   NumGC       — total number of completed garbage-collection cycles
	//
	// int() casts the uint64 values from MemStats to int because the
	// defs.MemoryResponse fields are typed as int.  Memory sizes this large
	// would overflow int32 on a 32-bit platform, but Ego servers only run on
	// 64-bit systems where int is 64 bits, so the cast is safe.
	response := defs.MemoryResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Total:      int(m.TotalAlloc),
		Current:    int(m.HeapInuse),
		System:     int(m.Sys),
		Stack:      int(m.StackInuse),
		Objects:    int(m.HeapObjects),
		GCCount:    int(m.NumGC),
		Status:     http.StatusOK,
	}

	// Set the Content-Type header so clients know the response body contains
	// Ego memory statistics rather than plain JSON.
	w.Header().Add(defs.ContentTypeHeader, defs.MemoryMediaType)

	// util.WriteJSON serializes response to JSON, writes it to w, and returns
	// the raw bytes so we can log them below.  session.ResponseLength is
	// updated so the server can report how many bytes were sent.
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
