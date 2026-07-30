package router

// Regression tests for the GORTNS-4 fix: the server's long-lived background
// tasks now take a stop channel so they can be shut down instead of spinning
// until the process exits.
//
// Each test asserts that the task actually RETURNS, using a channel to observe
// completion. A test that merely called the task and checked nothing would pass
// against the unfixed code, because the failure mode is "never returns" -- so
// the assertion has to be about termination, with a timeout to catch a hang.

import (
	"testing"
	"time"
)

// runsUntilStopped launches task on its own goroutine, closes the stop channel,
// and reports whether the task returned within the grace period.
//
// The "done" channel is how the test observes another goroutine finishing: the
// launched function closes it on the way out, and the select below waits for
// either that close or a timeout, whichever happens first.
func runsUntilStopped(t *testing.T, task func(stop <-chan struct{})) bool {
	t.Helper()

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		task(stop)
	}()

	// Give the task a moment to reach its wait, so the test exercises the
	// "interrupt an in-progress wait" path rather than racing to signal before
	// the task even starts.
	time.Sleep(50 * time.Millisecond)

	close(stop)

	select {
	case <-done:
		return true

	case <-time.After(5 * time.Second):
		return false
	}
}

// TestLogMemoryStatisticsStops covers the memory-statistics task. Its interval
// comes from configuration and defaults to five minutes, so before the fix this
// goroutine was unstoppable for minutes at a time.
func TestLogMemoryStatisticsStops_GORTNS4(t *testing.T) {
	if !runsUntilStopped(t, LogMemoryStatistics) {
		t.Error("LogMemoryStatistics did not return after its stop channel was closed")
	}
}

// TestLogRequestCountsStops covers the request-counter task, whose interval is
// 60 seconds.
func TestLogRequestCountsStops_GORTNS4(t *testing.T) {
	if !runsUntilStopped(t, LogRequestCounts) {
		t.Error("LogRequestCounts did not return after its stop channel was closed")
	}
}

// TestSleepOrStopReportsFullInterval confirms the helper returns true when the
// interval elapses with no stop signal, which is what keeps a task looping
// during normal operation. Getting this backwards would make the tasks exit
// after their first tick.
func TestSleepOrStopReportsFullInterval_GORTNS4(t *testing.T) {
	stop := make(chan struct{})

	if !sleepOrStop(10*time.Millisecond, stop) {
		t.Error("sleepOrStop returned false after a full interval with no stop signal")
	}
}

// TestSleepOrStopReportsStop confirms the cancel path, and that it returns
// promptly rather than waiting out the interval. A one-hour interval makes the
// distinction unambiguous: if the implementation used time.Sleep, this test
// would time out rather than fail.
func TestSleepOrStopReportsStop_GORTNS4(t *testing.T) {
	stop := make(chan struct{})
	close(stop)

	start := time.Now()

	if sleepOrStop(time.Hour, stop) {
		t.Error("sleepOrStop returned true even though the stop channel was closed")
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("sleepOrStop took %v to notice the stop signal; it should return promptly", elapsed)
	}
}

// TestSleepOrStopWithNilChannel documents the "never cancel" case. A receive on
// a nil channel blocks forever, so the select can only ever choose the timer --
// which is what makes passing nil a legal way to say "run until the process
// ends".
func TestSleepOrStopWithNilChannel_GORTNS4(t *testing.T) {
	if !sleepOrStop(10*time.Millisecond, nil) {
		t.Error("sleepOrStop with a nil stop channel returned false; it should always complete its interval")
	}
}
