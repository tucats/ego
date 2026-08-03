package bytecode

// This file implements Ego's built-in statement profiler: the @profile
// compiler directive (profileByteCode, in flow.go) and the "ego run
// --profile" CLI flag both funnel into ProfileAction() below.
//
// Profiling data used to live in a single global map[string]*atomic.Uint32,
// keyed by a "module:line" string built with fmt.Sprintf on every single
// AtLine hit -- an allocation and a map lookup for every statement executed,
// serialized behind one mutex shared by every goroutine. This version instead
// attaches a []profileSlot directly to each compiled *ByteCode object,
// indexed by instruction offset (the same offset the interpreter already
// tracks as c.programCounter), so recording a hit is a direct slice index
// with no allocation, no hashing, and no lock on the hot path -- only atomic
// adds. See ensureProfileSlot below, and the Context.profileSlot/
// Context.profileStart fields (context.go) and atLineByteCode (flow.go)
// for the elapsed-time bookkeeping this enables.

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/tucats/ego/internal/cli/tables"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
)

// Profiling action codes for the @profile directive and the --profile CLI flag.
const (
	StartAction int = iota
	StopAction
	ReportAction
)

// profilingActive is read on every AtLine hit, so it must be cheap when
// profiling is off. An atomic.Bool load is a single instruction and requires
// no lock, unlike the mutex-guarded bool the old implementation used.
var profilingActive atomic.Bool

// profiledCode is the registry of every *ByteCode object that has recorded at
// least one profiling hit during the current session -- the only thing
// PrintProfileReport needs to walk, now that the actual data lives on each
// ByteCode object rather than in one central map. profileRegistryMux guards
// it, but it is only ever taken once per unique compiled function (the first
// time that function records a hit), not once per statement, so it is not a
// hot-path cost.
var (
	profiledCode      []*ByteCode
	profileRegistryMux sync.Mutex
)

// profileSlot holds the accumulated profiling data for a single AtLine
// instruction (i.e. one source statement) within a compiled ByteCode object.
// count and nanos are updated with atomic adds, since the same compiled
// function can execute concurrently on multiple goroutines (a cached service
// program handling concurrent requests, for example) -- no per-slot lock is
// needed. line is effectively write-once: every goroutine that races to set
// it writes the same value (a given instruction's source line never
// changes), so using an atomic store/load instead of a lock keeps this race-
// detector-clean without needing synchronization for what is otherwise an
// idempotent write.
type profileSlot struct {
	line  atomic.Int32
	count atomic.Uint32
	nanos atomic.Int64
}

// profileStorage is the actual profiling storage for one compiled ByteCode
// object: one slot per instruction, indexed by instruction offset. It is
// allocated once (see ensureProfileSlot) and its slots slice is never
// resized or reassigned afterward, only the slots' own atomic fields are
// updated -- so slots is safe to read without a lock once storage itself has
// been obtained via ByteCode.profile.Load().
type profileStorage struct {
	slots []profileSlot
}

// ensureProfileSlot returns the profile slot for the AtLine instruction at
// offset pc within this ByteCode's instruction array, lazily allocating (and
// registering, on first use) the slot storage. line is the resolved source
// line number for this instruction; it is recorded the first time this slot
// is touched. Returns nil if pc is out of range for this ByteCode's
// instructions -- normal execution always calls this with
// c.programCounter-1 immediately after the interpreter's run loop has
// already advanced programCounter past a real instruction (see run.go), so
// pc is always valid there, but atLineByteCode can also run with a
// synthetic, never-advanced Context (unit tests calling it directly, for
// instance), where pc-1 would otherwise index the slots slice with -1.
// Profiling must never be the reason real execution panics, so this is a
// bounds check, not an assertion.
//
// Allocation is lock-free: b.profile is accessed via atomic.LoadPointer/
// CompareAndSwapPointer (see its comment in bytecode.go for why it is a bare
// unsafe.Pointer rather than a sync.Mutex or atomic.Pointer[T]), and the race
// to install the first profileStorage for a given ByteCode is resolved with
// CompareAndSwapPointer, not a mutex. profileRegistryMux is only ever taken
// by whichever goroutine actually wins that race, to append to the
// report-time walk list -- once per unique ByteCode, not once per statement.
func (b *ByteCode) ensureProfileSlot(pc int, line int) *profileSlot {
	if pc < 0 || pc >= len(b.instructions) {
		return nil
	}

	storage := (*profileStorage)(atomic.LoadPointer(&b.profile))

	if storage == nil {
		newStorage := &profileStorage{slots: make([]profileSlot, len(b.instructions))}

		if atomic.CompareAndSwapPointer(&b.profile, nil, unsafe.Pointer(newStorage)) {
			storage = newStorage

			profileRegistryMux.Lock()
			profiledCode = append(profiledCode, b)
			profileRegistryMux.Unlock()
		} else {
			// Another goroutine won the race and installed storage first;
			// newStorage is simply discarded.
			storage = (*profileStorage)(atomic.LoadPointer(&b.profile))
		}
	}

	slot := &storage.slots[pc]

	if slot.line.Load() == 0 {
		slot.line.Store(int32(line))
	}

	return slot
}

// FlushProfileTimer credits whatever statement's timer is currently pending
// (if any) with its elapsed time so far, and clears it. atLineByteCode
// normally does this itself as part of moving on to the next statement (see
// flow.go), crediting the delta as it starts the new one's timer -- but nothing
// else naturally follows the LAST statement a function executes before it
// returns, or the last statement the whole program executes before it halts.
// Without this, that pending time would either be lost entirely, or -- worse
// -- silently bleed into whatever unrelated statement happened to run next
// (e.g. a caller's next line, well after the callee returned), inflating it
// with time that was never really spent there. Called from callFramePop
// (function return) and from the top-level Run() caller
// (internal/commands/run.go, program exit).
//
// This must be called under the same c.shared/c.mux guard that protects
// c.profileSlot/c.profileStart everywhere else they are touched, when the
// context is shared with another goroutine.
func (c *Context) FlushProfileTimer() {
	if !profilingActive.Load() || c.profileSlot == nil {
		return
	}

	c.profileSlot.nanos.Add(int64(time.Since(c.profileStart)))
	c.profileSlot = nil
}

// ProfileAction starts, stops, or reports on the built-in profiler. This is
// the single entry point used by both profileByteCode (the @profile compiler
// directive, in flow.go) and the "ego run --profile" CLI flag
// (internal/commands/run.go). Named ProfileAction rather than Profile to
// avoid colliding with the pre-existing Profile Opcode constant
// (opcodes.go), which is a completely unrelated identifier: the numeric
// bytecode operation that dispatches to profileByteCode.
func ProfileAction(action int) error {
	switch action {
	case StartAction:
		if profilingActive.Load() {
			ui.Log(ui.InternalLogger, "runtime.profile.active", nil)
		}

		resetProfileData()
		profilingActive.Store(true)

		ui.Log(ui.InternalLogger, "runtime.profile.started", nil)

		return nil

	case StopAction:
		if !profilingActive.Load() {
			ui.Log(ui.InternalLogger, "runtime.profile.inactive", nil)
		}

		ui.Log(ui.InternalLogger, "runtime.profile.stopped", nil)

		profilingActive.Store(false)

	case ReportAction:
		return PrintProfileReport()

	default:
		return errors.ErrInvalidProfileAction.Context(action)
	}

	return nil
}

// resetProfileData clears every registered ByteCode's profiling data and
// empties the registry, so a fresh "start" (or the report that follows one)
// begins with a clean slate, exactly like the old map-based implementation
// clearing PerformanceData.
func resetProfileData() {
	profileRegistryMux.Lock()
	code := profiledCode
	profiledCode = nil
	profileRegistryMux.Unlock()

	for _, bc := range code {
		atomic.StorePointer(&bc.profile, nil)
	}
}

// profileEntry is one reportable (module, line) row: a statement that was
// actually hit at least once during the profiling session.
type profileEntry struct {
	module string
	line   int32
	count  uint32
	nanos  int64
}

// PrintProfileReport prints a formatted report of the performance data
// collected during profiling: every location visited, how many times, and
// the total elapsed time attributed to it.
func PrintProfileReport() error {
	profileRegistryMux.Lock()
	code := append([]*ByteCode(nil), profiledCode...)
	profileRegistryMux.Unlock()

	// Keyed by "module:line" to merge rows that land on the same source
	// location but different *ByteCode objects -- the same function literal
	// cloned once per push (pushByteCode, stack.go) for a closure created
	// inside a loop gets its own independent profileStorage per clone (see
	// ByteCode.profile's comment in bytecode.go), so without this merge the
	// report would show one fragmented row per clone instead of one combined
	// row for the location they all share.
	merged := map[string]*profileEntry{}

	for _, bc := range code {
		storage := (*profileStorage)(atomic.LoadPointer(&bc.profile))
		if storage == nil {
			continue
		}

		for i := range storage.slots {
			slot := &storage.slots[i]

			count := slot.count.Load()
			if count == 0 {
				continue
			}

			line := slot.line.Load()
			key := fmt.Sprintf("%s:%d", bc.name, line)

			if existing, found := merged[key]; found {
				existing.count += count
				existing.nanos += slot.nanos.Load()
			} else {
				merged[key] = &profileEntry{
					module: bc.name,
					line:   line,
					count:  count,
					nanos:  slot.nanos.Load(),
				}
			}
		}
	}

	if len(merged) == 0 {
		return nil
	}

	entries := make([]profileEntry, 0, len(merged))
	for _, entry := range merged {
		entries = append(entries, *entry)
	}

	// Sorting numerically by (module, line) is straightforward now that line
	// is a real int rather than a component of a formatted string -- the old
	// implementation's zero-padded sort-key workaround (and the parsing bug
	// it was working around, INDEX-16) no longer applies.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].module != entries[j].module {
			return entries[i].module < entries[j].module
		}

		return entries[i].line < entries[j].line
	})

	t, err := tables.New([]string{i18n.L("Location"), i18n.L("Count"), i18n.L("Elapsed")})
	if err != nil {
		return err
	}

	if err := t.SetAlignment(1, tables.AlignmentRight); err != nil {
		return err
	}

	if err := t.SetAlignment(2, tables.AlignmentRight); err != nil {
		return err
	}

	// No pagination for this report.
	t.SetPagination(0, 0)

	for _, entry := range entries {
		location := fmt.Sprintf("%s:%d", entry.module, entry.line)
		elapsed := time.Duration(entry.nanos).String()

		if err := t.AddRowItems(location, entry.count, elapsed); err != nil {
			return err
		}
	}

	if err := t.Print(ui.TextFormat); err != nil {
		return err
	}

	// Empty out the performance data for the next report.
	resetProfileData()

	return nil
}
