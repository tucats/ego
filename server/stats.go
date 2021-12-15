package server

import (
	xruntime "runtime"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

const (
	AdminRequestCounter = iota
	ServiceRequestCounter
	CodeRequestCounter
	HeartbeatRequestCounter
	AssetRequestCounter
	TableRequestCounter

	logRequestCounterDuration = 60
)

var adminRequestCount int32
var serviceRequestCount int32
var codeRequestCount int32
var heartbeatRequestCount int32
var assetRequestCount int32
var tableRequestCount int32

func CountRequest(kind int) {
	switch kind {
	case TableRequestCounter:
		atomic.AddInt32(&tableRequestCount, 1)

	case AssetRequestCounter:
		atomic.AddInt32(&assetRequestCount, 1)

	case AdminRequestCounter:
		atomic.AddInt32(&adminRequestCount, 1)

	case ServiceRequestCounter:
		atomic.AddInt32(&serviceRequestCount, 1)

	case CodeRequestCounter:
		atomic.AddInt32(&codeRequestCount, 1)

	case HeartbeatRequestCounter:
		atomic.AddInt32(&heartbeatRequestCount, 1)
	}
}

// LogRequestCounts is a go-routine launched when a server is started. It generates logging output
// every 60 seconds _if_ there have been requests to any of the endpoint groups: admin, code,
// heartbeat, or service. If there have been no request in the last 60 seconds, no log record is
// generated. Once the log is evaluated and printed if needed, the routine sleeps for another 60
// seconds and repeats the operation.
func LogRequestCounts() {
	duration := logRequestCounterDuration

	for {
		time.Sleep(logRequestCounterDuration * time.Second)

		admin := atomic.SwapInt32(&adminRequestCount, 0)
		service := atomic.SwapInt32(&serviceRequestCount, 0)
		code := atomic.SwapInt32(&codeRequestCount, 0)
		heartbeats := atomic.SwapInt32(&heartbeatRequestCount, 0)
		assets := atomic.SwapInt32(&assetRequestCount, 0)
		tables := atomic.SwapInt32(&tableRequestCount, 0)

		// If no activity in the last minute, no work to do.
		if admin+service+code+heartbeats+assets == 0 {
			continue
		}

		ui.Debug(ui.ServerLogger, "Requests in last %d seconds: admin(%d)  service(%d)  asset(%d)  code(%d)  heartbeat(%d)  tables(%d)",
			duration, admin, service, assets, code, heartbeats, tables)
	}
}

// LogMemoryStatitics is a go-routine launched when a server is started. It generates a logging
// entry every ten minutes indicating the current memory allocation, the total memory ever
// allocated, the system memory, and the number of times the garbage-collector has run.
func LogMemoryStatistics() {
	var m xruntime.MemStats

	for {
		// For info on each, see: https://golang.org/pkg/runtime/#MemStats
		xruntime.ReadMemStats(&m)
		ui.Debug(ui.ServerLogger, "Memory: Allocated(%8.3fmb) Total(%8.3fmb) System(%8.3fmb) GC(%d) ",
			bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)

		// Generate this report in the log every ten minutes.
		time.Sleep(10 * time.Minute)
	}
}

// bToMb is a helper function that converts a total number of bytes to a fractional
// number of megabytes. This is used for formatting the memory statistics log entries.
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
