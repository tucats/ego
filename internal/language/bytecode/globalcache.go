package bytecode

// This file implements the runtime half of PERFORMANCE.md Finding 17's fix
// (see docs/internals/GLOBALS.md Section 6): a per-instruction cache, on
// each compiled *ByteCode, of which persistent "global singleton" table
// (main's own top-level table, or an imported package's own table -- see
// symbols.SymbolTable.IsGlobalSingleton) a Load/Store/AddressOf/DeRef
// instruction resolved a name to. A cache hit lets the opcode handler call
// Get/Set/GetAddress directly on the remembered table, skipping the O(depth)
// walk through intervening call frames that FindNextScope would otherwise
// repeat on every call. This is the companion to Tier 1 (constant.go's
// compile-time const folding): Tier 1 eliminates the lookup entirely for
// same-compilation-unit consts; this cache instead amortizes it to O(1)
// after a one-time miss for everything Tier 1 can't fold -- var, cross-file/
// cross-package const, and (see types.go's unwrapByteCode) the one type-
// assertion fallback that still does a name-based lookup.
//
// Storage lives on *ByteCode (the long-lived compiled function, shared
// across every call/activation), indexed by instruction offset, following
// the exact pattern this codebase's statement profiler (profile.go) already
// established for the same reason: a bare unsafe.Pointer field, accessed
// only through sync/atomic's free functions, because go vet's copylocks
// check flags both a directly-embedded sync.Mutex and a generic
// atomic.Pointer[T] as unsafe to duplicate in a struct (ByteCode) that is
// copied by value elsewhere (Clone(), NeedsCoerce's value receiver in
// coerce.go, restoreByteCode's struct assignment in compiler/function.go).

import (
	"sync/atomic"
	"unsafe"

	"github.com/tucats/ego/internal/language/symbols"
)

// GlobalCacheEnabled gates the Tier 2 cache below, independent of Tier 1's
// const-folding (ego.compiler.constfold). Set once, before any concurrent
// execution begins, from the ego.runtime.globalcache setting (see
// internal/commands/run.go), mirroring how symbols.SerializeTableAccess is
// wired. Defaults to true.
var GlobalCacheEnabled = true

// globalRefStorage is the Finding 17 Tier 2 cache for one compiled ByteCode
// object: one *symbols.SymbolTable slot per instruction offset. It is
// allocated once (see ensureGlobalRefStorage) and its tables slice is never
// resized or reassigned afterward -- only the individual slot pointers are
// ever written, atomically.
type globalRefStorage struct {
	tables []unsafe.Pointer // each element holds a *symbols.SymbolTable, atomic
}

// ensureGlobalRefStorage lazily allocates this ByteCode's Tier 2 cache,
// sized to its instruction count. Lock-free, mirroring ensureProfileSlot's
// own CompareAndSwapPointer-guarded allocation (profile.go): the race to
// install the first storage for a given ByteCode is resolved by whichever
// goroutine's CompareAndSwapPointer succeeds; the loser's allocation is
// simply discarded.
func (b *ByteCode) ensureGlobalRefStorage() *globalRefStorage {
	if storage := (*globalRefStorage)(atomic.LoadPointer(&b.globalRefs)); storage != nil {
		return storage
	}

	newStorage := &globalRefStorage{tables: make([]unsafe.Pointer, len(b.instructions))}

	if atomic.CompareAndSwapPointer(&b.globalRefs, nil, unsafe.Pointer(newStorage)) {
		return newStorage
	}

	return (*globalRefStorage)(atomic.LoadPointer(&b.globalRefs))
}

// cachedGlobalTable returns the table previously cached for the instruction
// at offset pc, or nil if none has been cached yet. Returns nil (rather than
// panicking) if pc is out of range for this ByteCode's instructions -- this
// cache must never be the reason real execution fails, mirroring
// ensureProfileSlot's own defensive bounds check.
func (b *ByteCode) cachedGlobalTable(pc int) *symbols.SymbolTable {
	if pc < 0 || pc >= len(b.instructions) {
		return nil
	}

	storage := (*globalRefStorage)(atomic.LoadPointer(&b.globalRefs))
	if storage == nil {
		return nil
	}

	return (*symbols.SymbolTable)(atomic.LoadPointer(&storage.tables[pc]))
}

// cacheGlobalTable records table as the resolved destination for the
// instruction at offset pc, so future executions of that same instruction
// can skip straight to it instead of walking the scope chain. Callers must
// only pass a table for which table.IsGlobalSingleton() is true (see
// docs/internals/GLOBALS.md Section 6.3 for why that is what makes this
// safe to cache and reuse indefinitely). Safe to call concurrently from
// multiple goroutines racing to populate the same, correct answer --
// whichever write lands last simply overwrites an identical pointer value.
func (b *ByteCode) cacheGlobalTable(pc int, table *symbols.SymbolTable) {
	if pc < 0 || pc >= len(b.instructions) {
		return
	}

	storage := b.ensureGlobalRefStorage()

	atomic.StorePointer(&storage.tables[pc], unsafe.Pointer(table))
}
