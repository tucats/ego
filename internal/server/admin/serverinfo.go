package admin

import (
	"net/http"
	"runtime"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// GetServerInfoHandler is the HTTP handler for GET /admin/serverinfo. It reports
// on the host machine the server is running on -- CPU, memory, operating
// system, and architecture -- as opposed to GetMemoryHandler/GetResourcesHandler,
// which report on the Go process itself.
//
// CPU count and architecture come from the runtime package, which has always
// had portable support for them. Total/available memory and OS version
// information have no equivalent in the Go standard library, so those come
// from gopsutil, a third-party library that wraps the OS-specific mechanism
// for each (/proc/meminfo and /etc/os-release on Linux, sysctl on macOS,
// Win32 APIs on Windows, and so on) behind one portable API.
//
// Both gopsutil calls can fail (for example, if a sandboxed or containerized
// environment blocks the underlying OS query) but that failure is not fatal
// to the request: this is a best-effort, informational endpoint for the
// dashboard's Server Info sheet, so a failure just leaves its fields zero and
// logs a warning rather than failing the whole response.
func GetServerInfoHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	response := defs.ServerInfoResponse{
		ServerInfo:   util.MakeServerInfo(session.ID),
		CPUCores:     runtime.NumCPU(),
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
		Status:       http.StatusOK,
	}

	if hostInfo, err := host.Info(); err != nil {
		ui.Log(ui.ServerLogger, "admin.serverinfo.error", ui.A{
			"item":  "OS",
			"error": err})
	} else {
		response.Platform = hostInfo.Platform
		response.PlatformVersion = hostInfo.PlatformVersion
		response.KernelVersion = hostInfo.KernelVersion
	}

	if memInfo, err := mem.VirtualMemory(); err != nil {
		ui.Log(ui.ServerLogger, "admin.serverinfo.error", ui.A{
			"item":  "memory",
			"error": err})
	} else {
		response.TotalMemory = memInfo.Total
		response.AvailableMemory = memInfo.Available
	}

	// Set the Content-Type header so clients know the response body contains
	// Ego server info rather than plain JSON.
	w.Header().Add(defs.ContentTypeHeader, defs.ServerInfoMediaType)

	// util.WriteJSON serializes response to JSON, writes it to w, and returns
	// the raw bytes so we can log them below.  session.ResponseLength is
	// updated so the server can report how many bytes were sent.
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
