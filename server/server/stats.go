package server

import (
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

type ServiceClass int

const (
	NotCounted ServiceClass = iota
	AdminRequestCounter
	ServiceRequestCounter
	CodeRequestCounter
	HeartbeatRequestCounter
	AssetRequestCounter
	TableRequestCounter

	logRequestCounterDuration = 60
)

// These are the actual counters for each class of value. They are int32 to
// be able to use the atomic increment function.
var (
	adminRequestCount     int32
	serviceRequestCount   int32
	codeRequestCount      int32
	heartbeatRequestCount int32
	assetRequestCount     int32
	tableRequestCount     int32
)

// CountRequest provides thread-safe counting of classes of REST API calls,
// which are periodically logged by Ego when running REST server mode. The
// parameter must be the request classification.
func CountRequest(kind ServiceClass) {
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

		// For each class, get the current counter value and
		// reset the counter to zero.
		admin := atomic.SwapInt32(&adminRequestCount, 0)
		service := atomic.SwapInt32(&serviceRequestCount, 0)
		code := atomic.SwapInt32(&codeRequestCount, 0)
		heartbeats := atomic.SwapInt32(&heartbeatRequestCount, 0)
		assets := atomic.SwapInt32(&assetRequestCount, 0)
		tables := atomic.SwapInt32(&tableRequestCount, 0)

		// If there was activity on one or more classes, log the counts.
		if admin+service+code+heartbeats+assets > 0 {
			ui.Log(ui.ServerLogger, "Requests in last %d seconds: admin(%d)  service(%d)  asset(%d)  code(%d)  heartbeat(%d)  tables(%d)",
				duration, admin, service, assets, code, heartbeats, tables)
		}
	}
}
