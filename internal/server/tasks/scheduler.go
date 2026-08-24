package tasks

import (
	"sync"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util"
)

const (
	// schedulerTickInterval is how often the scheduler looks for due tasks.
	// Tasks don't need to start at the exact moment they become due, so a
	// coarse tick keeps this cheap.
	schedulerTickInterval = 15 * time.Second

	// defaultMaxConcurrent is used when ego.server.tasks.max.concurrent is
	// unset or not a positive number.
	defaultMaxConcurrent = 3
)

// dispatchFunc performs one task's endpoint call and reports the resulting
// HTTP status and whether the run is considered successful (actual status
// matched the task's expected status). It is a package variable, not a
// direct call, so the scheduler's due-task and concurrency logic can be
// unit tested without a running router/auth service. dispatch.go assigns
// the real implementation.
var dispatchFunc = func(task *Task) (status int, success bool) {
	ui.Log(tasksLogger, "tasks.dispatch.unavailable", ui.A{"id": task.ID})

	return 0, false
}

var schedulerOnce sync.Once

// StartScheduler starts the background goroutine that periodically looks
// for due tasks and runs them, up to ego.server.tasks.max.concurrent at a
// time. Safe to call more than once; only the first call has any effect.
func StartScheduler() {
	schedulerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(schedulerTickInterval)
			defer ticker.Stop()

			for range ticker.C {
				util.SafeCall("task scheduler tick", tick)
			}
		}()
	})
}

// tick looks for due tasks and starts as many as the concurrency limit
// allows. A task that is due but can't be started this tick, because the
// pool is full, is simply reconsidered on the next tick.
func tick() {
	startDueTasks(time.Now(), resolveMaxConcurrent())
}

// resolveMaxConcurrent reads ego.server.tasks.max.concurrent, falling back
// to defaultMaxConcurrent when it's unset or not a positive number. Split
// out from tick so scheduling logic (startDueTasks) can be unit tested
// with a fixed limit, without touching the global settings map from a
// background goroutine while a test is concurrently mutating it.
func resolveMaxConcurrent() int {
	maxConcurrent := settings.GetInt(defs.TasksMaxConcurrentSetting)
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}

	return maxConcurrent
}

// startDueTasks looks for due tasks and starts as many as maxConcurrent
// (minus however many are already running) allows.
func startDueTasks(now time.Time, maxConcurrent int) {
	available := maxConcurrent - runningCount()
	if available <= 0 {
		return
	}

	for _, task := range Tasks() {
		if available <= 0 {
			return
		}

		if !isDue(task, now) {
			continue
		}

		if !tryClaim(task.ID) {
			continue
		}

		available--

		go runOne(task)
	}
}

// isDue reports whether task should run now. It must be active and not
// already running; beyond that, whether it's due depends on whether it has
// ever run before:
//
//   - Never run: due once it clears its After eligibility delay (measured
//     from when it was loaded into the registry).
//   - Already run at least once: a task with no Interval is one-shot and
//     is never due again. A task with an Interval is due again once that
//     much time has passed since its last run finished, unless Count
//     caps it and it has already used up its run budget -- validateTask
//     guarantees Count is 0 or 1 whenever Interval is empty, so the
//     one-shot check above and the Count check below never disagree.
func isDue(task *Task, now time.Time) bool {
	registryLock.RLock()
	defer registryLock.RUnlock()

	// Active is read inside the same critical section as the state lookup
	// below, not before it, because DELETE /admin/tasks/{id} (setActive)
	// flips it under this same lock -- reading it outside the lock would
	// race with that write.
	if !task.Active {
		return false
	}

	state, found := states[task.ID]
	if found && state.Running {
		return false
	}

	neverRun := !found || state.LastRun.IsZero()

	if !neverRun {
		switch {
		case task.Interval == "":
			// One-shot: it already had its one run.
			return false
		case task.Count > 0 && state.RunCount >= task.Count:
			// Recurring, but it's used up its lifetime run budget.
			return false
		default:
			interval, err := util.ParseDuration(task.Interval)
			if err != nil {
				return false
			}

			if now.Sub(state.LastRun) < interval {
				return false
			}
		}
	}

	// The After delay only gates the first-ever run -- a startup stagger,
	// measured from when the task was loaded into the registry -- not
	// reapplied to later recurrences.
	if neverRun && found && task.After != "" && !state.LoadedAt.IsZero() {
		if delay, err := util.ParseDuration(task.After); err == nil && now.Before(state.LoadedAt.Add(delay)) {
			return false
		}
	}

	return true
}

// runningCount returns the number of tasks currently marked as running.
func runningCount() int {
	registryLock.RLock()
	defer registryLock.RUnlock()

	count := 0

	for _, state := range states {
		if state.Running {
			count++
		}
	}

	return count
}

// tryClaim marks a task as running if it isn't already, returning true if
// the claim succeeded. The scheduler uses this to avoid starting a second
// concurrent run of the same task.
func tryClaim(id string) bool {
	registryLock.Lock()
	defer registryLock.Unlock()

	state, found := states[id]
	if !found {
		// Defensive fallback: register/upsert should always have created
		// a state entry already. LoadedAt is still set here for
		// consistency, in case this path is ever actually exercised.
		state = &State{LoadedAt: time.Now()}
		states[id] = state
	}

	if state.Running {
		return false
	}

	state.Running = true

	return true
}

// runOne runs a single task's dispatch and records the outcome. It always
// runs as its own goroutine (started by tick), so the call is isolated
// with SafeCall rather than being left to crash the whole process on a
// panic. recordRun happens unconditionally, whether or not dispatchFunc
// panicked: it clears the Running flag either way, so a panicking
// dispatch doesn't wedge the task in "running" forever, and the time it
// records reflects when the task finished, per the "interval restarts
// from completion" contract.
func runOne(task *Task) {
	var status int

	var success bool

	completed := util.SafeCall("run task "+task.ID, func() {
		status, success = dispatchFunc(task)
	})

	if !completed {
		status, success = 0, false
	}

	recordRun(task.ID, status, success, time.Now())
}
