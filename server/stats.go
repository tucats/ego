package server

import (
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

const (
	AdminRequestCounter = iota
	ServiceRequestCounter
	CodeRequestCounter
	HeartbeatRequestCounter

	logRequestCounterDuration = 60
)

var adminRequestCount int32
var serviceRequestCount int32
var codeRequestCount int32
var heartbeatRequestCount int32

func CountRequest(kind int) {
	switch kind {
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

func LogRequestCounts() {
	duration := logRequestCounterDuration

	for {
		time.Sleep(logRequestCounterDuration * time.Second)

		admin := atomic.SwapInt32(&adminRequestCount, 0)
		service := atomic.SwapInt32(&serviceRequestCount, 0)
		code := atomic.SwapInt32(&codeRequestCount, 0)
		heartbeats := atomic.SwapInt32(&heartbeatRequestCount, 0)

		// If no activity in the last minute, no work to do.
		if admin+service+code+heartbeats == 0 {
			continue
		}

		ui.Debug(ui.ServerLogger, "Requests in last %d seconds: admin(%d)  service(%d)  code(%d)  heartbeat(%d)", duration, admin, service, code, heartbeats)
	}
}
