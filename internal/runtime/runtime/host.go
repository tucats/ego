package runtime

import (
	goRuntime "runtime"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
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
func captureHostInfo() hostSnapshot {
	snapshot := hostSnapshot{
		CPUCores: goRuntime.NumCPU(),
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

	return snapshot
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
