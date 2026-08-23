package tasks

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// useFakeDispatcher substitutes dispatchFunc for the duration of the test
// and restores the original (dispatch-unavailable) stub afterward.
func useFakeDispatcher(t *testing.T, fake func(task *Task) (int, bool)) {
	t.Helper()

	original := dispatchFunc
	dispatchFunc = fake

	t.Cleanup(func() {
		dispatchFunc = original
	})
}

// useSandboxedTasksDir points the tasks directory at a temp dir and
// creates it, so a test that runs recordRun (which calls SaveState, which
// writes the sidecar state file) can never write into a real lib/tasks
// directory on the machine running the test.
func useSandboxedTasksDir(t *testing.T) {
	t.Helper()

	root := useTempLibDir(t)

	if err := os.MkdirAll(filepath.Join(root, "tasks"), requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}
}

func TestIsDueNeverRun(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "1h"}

	if !isDue(task, time.Now()) {
		t.Error("a task with no prior state should be due")
	}
}

func TestIsDueInactive(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: false, Repeat: "1h"}

	if isDue(task, time.Now()) {
		t.Error("an inactive task should never be due")
	}
}

func TestIsDueAlreadyRunning(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "1h"}

	registryLock.Lock()
	states[task.ID] = &State{Running: true, LastRun: time.Now().Add(-2 * time.Hour)}
	registryLock.Unlock()

	if isDue(task, time.Now()) {
		t.Error("a task already running should not be due again")
	}
}

func TestIsDueOnceAlreadyRan(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "once"}

	registryLock.Lock()
	states[task.ID] = &State{LastRun: time.Now().Add(-1 * time.Minute)}
	registryLock.Unlock()

	if isDue(task, time.Now()) {
		t.Error("a \"once\" task that already ran should not be due again")
	}
}

func TestIsDueRecurringNotYetElapsed(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "1h"}

	registryLock.Lock()
	states[task.ID] = &State{LastRun: time.Now().Add(-30 * time.Minute)}
	registryLock.Unlock()

	if isDue(task, time.Now()) {
		t.Error("a recurring task should not be due before its interval elapses")
	}
}

func TestIsDueRecurringElapsed(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "1h"}

	registryLock.Lock()
	states[task.ID] = &State{LastRun: time.Now().Add(-2 * time.Hour)}
	registryLock.Unlock()

	if !isDue(task, time.Now()) {
		t.Error("a recurring task should be due once its interval elapses, including overdue after a restart")
	}
}

func TestIsDueUnparseableRepeatIsNeverDueAgain(t *testing.T) {
	resetRegistry(t)

	task := &Task{ID: "t1", Active: true, Repeat: "garbage"}

	registryLock.Lock()
	states[task.ID] = &State{LastRun: time.Now().Add(-1 * time.Hour)}
	registryLock.Unlock()

	if isDue(task, time.Now()) {
		t.Error("a task with an unparseable repeat value should not be treated as due")
	}
}

func TestTryClaimPreventsDoubleRun(t *testing.T) {
	resetRegistry(t)

	if !tryClaim("t1") {
		t.Fatal("first claim should succeed")
	}

	if tryClaim("t1") {
		t.Error("second claim while running should fail")
	}

	registryLock.Lock()
	states["t1"].Running = false
	registryLock.Unlock()

	if !tryClaim("t1") {
		t.Error("claim should succeed again once Running is cleared")
	}
}

func TestRunningCount(t *testing.T) {
	resetRegistry(t)

	registryLock.Lock()
	states["a"] = &State{Running: true}
	states["b"] = &State{Running: false}
	states["c"] = &State{Running: true}
	registryLock.Unlock()

	if count := runningCount(); count != 2 {
		t.Errorf("runningCount() = %d, want 2", count)
	}
}

func TestRunOneRecordsSuccessAndClearsRunning(t *testing.T) {
	useSandboxedTasksDir(t)

	task := &Task{ID: "t1", Active: true}
	registryLock.Lock()
	states[task.ID] = &State{}
	registryLock.Unlock()

	tryClaim(task.ID)

	useFakeDispatcher(t, func(*Task) (int, bool) { return 200, true })

	runOne(task)

	state, found := Status(task.ID)
	if !found {
		t.Fatal("expected a recorded state")
	}

	if state.Running {
		t.Error("expected Running to be false after the run completes")
	}

	if state.LastStatus != 200 || !state.Success {
		t.Errorf("state = %+v, want LastStatus=200, Success=true", state)
	}
}

func TestRunOneRecoversFromPanicAndClearsRunning(t *testing.T) {
	useSandboxedTasksDir(t)

	task := &Task{ID: "t1", Active: true}
	registryLock.Lock()
	states[task.ID] = &State{}
	registryLock.Unlock()

	tryClaim(task.ID)

	useFakeDispatcher(t, func(*Task) (int, bool) { panic("boom") })

	runOne(task) // must not crash the test process

	state, found := Status(task.ID)
	if !found {
		t.Fatal("expected a recorded state even after a panic")
	}

	if state.Running {
		t.Error("expected Running to be cleared even after dispatchFunc panicked")
	}

	if state.Success {
		t.Error("expected Success to be false after a panicking dispatch")
	}
}

func TestStartDueTasksRespectsConcurrencyLimit(t *testing.T) {
	useSandboxedTasksDir(t)

	const taskCount = 5

	release := make(chan struct{})

	var started int32

	useFakeDispatcher(t, func(*Task) (int, bool) {
		atomic.AddInt32(&started, 1)
		<-release

		return 200, true
	})

	for i := 0; i < taskCount; i++ {
		id := string(rune('a' + i))
		task := &Task{ID: id, Active: true, Repeat: "once"}
		registry[id] = task
		states[id] = &State{}
	}

	startDueTasks(time.Now(), 2)

	// Give the goroutines startDueTasks started a moment to call dispatchFunc.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&started) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&started); got != 2 {
		t.Errorf("dispatchFunc started %d times on one tick, want exactly 2 (the concurrency limit)", got)
	}

	if running := runningCount(); running != 2 {
		t.Errorf("runningCount() = %d, want 2", running)
	}

	close(release)

	deadline = time.Now().Add(2 * time.Second)
	for runningCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if running := runningCount(); running != 0 {
		t.Fatalf("runningCount() = %d after release, want 0 (leaked goroutine would race the next test)", running)
	}
}
