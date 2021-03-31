package symbols

import (
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/util"
)

var mergeMutex sync.Mutex

// Merge merges the contents of a table into the current table.
func (s *SymbolTable) Merge(st *SymbolTable) {
	ui.Debug(ui.SymbolLogger, "+++ Merging symbols from %s to %s", st.Name, s.Name)

	if len(st.Values) == 0 {
		return
	}

	// This must be serialized on the two tables to avoid collisions between
	// threads.
	mergeMutex.Lock()

	defer mergeMutex.Unlock()

	for k, vx := range st.Symbols {
		// Is it a struct? If so we may need to merge to it...
		v := st.GetValue(vx)
		switch vv := v.(type) {
		case map[string]interface{}: // @tomcole should be package  **double check this***
			// Does the old struct already exist in the compiler table?
			old, found := s.Get(k)
			if found {
				// Is the existing value also a struct?
				switch oldmap := old.(type) {
				case map[string]interface{}: // @tomcole should be package
					// Copy the values into the existing map
					for newkeyword, newvalue := range vv {
						oldmap[newkeyword] = newvalue
						ui.Debug(ui.SymbolLogger, "    adding %v as \"%s\"", newvalue, newkeyword)
					}
					// Rewrite the map back to the bytecode.
					_ = s.SetAlways(k, oldmap)

				default:
					ui.Debug(ui.SymbolLogger, "    overwriting duplicate key \"%s\" with %v", k, old)
					_ = s.SetAlways(k, v)
				}
			} else {
				ui.Debug(ui.SymbolLogger, "    creating new map \"%s\" with %v", k, v)
				_ = s.SetAlways(k, v)
			}

		default:
			ui.Debug(ui.SymbolLogger, "    copying entry %s with %s", k, util.Format(vv))
			_ = s.SetAlways(k, vv)
		}
	}

	// Do it again with the constants
	ui.Debug(ui.SymbolLogger, "+++ Merging constants from  %s", st.Name)

	for k, v := range st.Constants {
		// Is it a struct? If so we may need to merge to it...
		switch vv := v.(type) {
		case map[string]interface{}: // @tomcole should be package
			// Does the old struct already exist in the compiler table?
			old, found := s.Get(k)
			if found {
				// Is the existing value also a struct?
				switch oldmap := old.(type) {
				case map[string]interface{}: // @tomcole should be package
					// Copy the values into the existing map
					for newkeyword, newvalue := range vv {
						oldmap[newkeyword] = newvalue
						ui.Debug(ui.SymbolLogger, "    adding %v to old map at %s", newvalue, newkeyword)
					}
					// Rewrite the map back to the bytecode.
					_ = s.SetConstant(k, oldmap)

				default:
					ui.Debug(ui.SymbolLogger, "    overwriting duplicate key %s with %v", k, old)
					_ = s.SetConstant(k, v)
				}
			} else {
				ui.Debug(ui.SymbolLogger, "    creating new map %s with %v", k, v)
				_ = s.SetConstant(k, v)
			}

		default:
			ui.Debug(ui.SymbolLogger, "    copying entry %s with %v", k, v)
			_ = s.SetConstant(k, vv)
		}
	}
}
