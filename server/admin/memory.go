package admin

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// GetMemoryHandler is the server endpoint handler for retrieving the memory status from the server.
func GetMemoryHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	result := defs.MemoryResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Total:      int(m.TotalAlloc),
		Current:    int(m.HeapInuse),
		System:     int(m.Sys),
		Stack:      int(m.StackInuse),
		Objects:    int(m.HeapObjects),
		GCCount:    int(m.NumGC),
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.MemoryMediaType)

	b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
