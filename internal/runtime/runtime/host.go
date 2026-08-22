package runtime

import (
	"os"
	goRuntime "runtime"
	"strconv"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// hostSnapshot holds the host machine facts exposed as the runtime package's
// OS_KERNEL_VERSION, OS_PLATFORM, OS_PLATFORM_VERSION, OS_CPUCORES, and
// OS_MEMORY constants. These are captured once, because none of them can
// change over the lifetime of a running process -- unlike available memory
// (see memoryAvailable() below), which is a function rather than a constant
// precisely because it does change from moment to moment.
//
// This mirrors what the server's GET /admin/serverinfo endpoint reports
// (internal/server/admin/serverinfo.go) for the same reason: Go's standard
// library has no portable API for total/available memory or OS version
// information -- only CPU count and architecture are covered by runtime
// itself -- so gopsutil supplies the platform-specific lookups both places
// need.
type hostSnapshot struct {
	KernelVersion   string
	Platform        string
	PlatformVersion string
	CPUCores        int
	Memory          int
}

// osInfo is a package-level variable initialized by a direct call to
// captureHostInfo() in its own declaration, not from an init() function.
// This matters: RuntimePackage (in types.go) is also a package-level
// variable, and its initializer expression reads osInfo's fields directly.
// Go orders package-level variable initialization by analyzing dependencies
// between initializer expressions, so it correctly initializes osInfo
// before RuntimePackage -- but that analysis does not see into init()
// function bodies, so setting these fields from an init() there would run
// too late, after RuntimePackage had already captured their zero values.
var osInfo = captureHostInfo()

// captureHostInfo gathers the static host facts once. Each gopsutil call can
// fail independently (for example, in a sandboxed environment that blocks
// the underlying OS query); a failure just leaves that group of fields at
// its zero value rather than panicking or preventing the runtime package
// itself from loading.
//
// Fast path: if an ancestor Ego process on this same host already captured
// these facts and published them via hostInfoFromEnv's environment
// variables (see publishHostInfoToEnv below), read them from there instead
// of querying the OS again. This is always true for a child-service
// process spawned by internal/server/services/child.go, since child
// processes inherit their parent's environment (see the doc comment on
// defs.EgoHostKernelVersionEnv for exactly how). Skipping the OS query
// matters because host.Info() alone measured at ~25-30ms in local testing
// -- a cost every child-service process would otherwise pay again on every
// single request, even though kernel version, platform, platform version,
// and total memory cannot change for the life of the host machine, so a
// value an ancestor process captured minutes or days ago is exactly as
// correct as one captured fresh right now.
//
// CPUCores is deliberately excluded from the published/read environment
// values and always computed locally via goRuntime.NumCPU(): it is free to
// compute (no OS query involved) and, unlike the other fields, could
// legitimately differ between an ancestor and this process if scheduling
// constraints such as GOMAXPROCS or a cgroup CPU limit differ between them.
func captureHostInfo() hostSnapshot {
	snapshot := hostSnapshot{
		CPUCores: goRuntime.NumCPU(),
	}

	if fromEnv, ok := hostInfoFromEnv(); ok {
		fromEnv.CPUCores = snapshot.CPUCores

		return fromEnv
	}

	if info, err := host.Info(); err == nil {
		snapshot.KernelVersion = info.KernelVersion
		snapshot.Platform = info.Platform
		snapshot.PlatformVersion = info.PlatformVersion
	}

	if info, err := mem.VirtualMemory(); err == nil {
		// Ego only runs on 64-bit platforms, where int is 64 bits, so this
		// conversion from gopsutil's uint64 never truncates a real-world
		// memory size.
		snapshot.Memory = int(info.Total)
	}

	// Publish what was just queried so that any child process spawned by
	// this one later -- directly, or transitively through a chain of
	// children -- can take the fast path above instead of repeating the
	// same OS query. A no-op cost-wise for a process that never spawns any
	// children.
	publishHostInfoToEnv(snapshot)

	return snapshot
}

// hostInfoFromEnv attempts to read a previously-captured host snapshot from
// the environment variables publishHostInfoToEnv writes. All four of
// EgoHostKernelVersionEnv, EgoHostPlatformEnv, EgoHostPlatformVersionEnv,
// and EgoHostMemoryEnv must be present, and the memory value must parse as
// an integer, for this to report success (ok == true); otherwise it
// returns the zero value and false, telling captureHostInfo to fall back
// to querying the OS directly. CPUCores is intentionally left at its zero
// value here -- see the comment on captureHostInfo for why the caller
// always overwrites it with a freshly-computed value rather than trusting
// one read from the environment.
func hostInfoFromEnv() (hostSnapshot, bool) {
	kernelVersion, ok := os.LookupEnv(defs.EgoHostKernelVersionEnv)
	if !ok {
		return hostSnapshot{}, false
	}

	platform, ok := os.LookupEnv(defs.EgoHostPlatformEnv)
	if !ok {
		return hostSnapshot{}, false
	}

	platformVersion, ok := os.LookupEnv(defs.EgoHostPlatformVersionEnv)
	if !ok {
		return hostSnapshot{}, false
	}

	memoryText, ok := os.LookupEnv(defs.EgoHostMemoryEnv)
	if !ok {
		return hostSnapshot{}, false
	}

	memoryBytes, err := strconv.Atoi(memoryText)
	if err != nil {
		return hostSnapshot{}, false
	}

	return hostSnapshot{
		KernelVersion:   kernelVersion,
		Platform:        platform,
		PlatformVersion: platformVersion,
		Memory:          memoryBytes,
	}, true
}

// publishHostInfoToEnv records a freshly-queried host snapshot in this
// process's own environment table (via os.Setenv), so that any process
// this one spawns later inherits it and can use hostInfoFromEnv's fast
// path instead of querying the OS itself. This works without any change
// to how child processes are spawned because Go's os/exec falls back to
// os.Environ() -- which reflects os.Setenv calls made any time earlier in
// this process's lifetime, including here, during this package's
// var-initializer, before main() even starts -- whenever a *exec.Cmd's own
// Env field is left nil, and the one place that builds Env explicitly
// instead (runChildViaPipe in internal/server/services/child.go) seeds it
// from os.Environ() too. Errors from os.Setenv are deliberately ignored:
// on the extremely unlikely failure, the affected child process(es) simply
// fall back to querying the OS themselves, exactly as if this function did
// not exist.
func publishHostInfoToEnv(snapshot hostSnapshot) {
	_ = os.Setenv(defs.EgoHostKernelVersionEnv, snapshot.KernelVersion)
	_ = os.Setenv(defs.EgoHostPlatformEnv, snapshot.Platform)
	_ = os.Setenv(defs.EgoHostPlatformVersionEnv, snapshot.PlatformVersion)
	_ = os.Setenv(defs.EgoHostMemoryEnv, strconv.Itoa(snapshot.Memory))
}

// memoryAvailable implements runtime.MemoryAvailable(), which returns the
// number of bytes of physical memory currently available for new
// allocations on the host machine. Unlike OS_MEMORY (the host's fixed
// total, captured once above), this changes constantly as other processes
// on the host allocate and free memory, so it is queried fresh on every
// call rather than captured once like the OS_* constants.
func memoryAvailable(s *symbols.SymbolTable, args data.List) (any, error) {
	info, err := mem.VirtualMemory()
	if err != nil {
		return 0, errors.New(err)
	}

	return int(info.Available), nil
}
