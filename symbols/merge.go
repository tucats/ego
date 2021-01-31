package symbols

import "github.com/tucats/ego/app-cli/ui"

// Merge merges the contents of a table into the current table.
func (s *SymbolTable) Merge(st *SymbolTable) {
	ui.Debug(ui.SymbolLogger, "+++ Merging symbols from %s", st.Name)
	for k, v := range st.Symbols {
		// Is it a struct? If so we may need to merge to it...
		switch vv := v.(type) {
		case map[string]interface{}:
			// Does the old struct already exist in the compiler table?
			old, found := s.Get(k)
			if found {
				// Is the existing value also a struct?
				switch oldmap := old.(type) {
				case map[string]interface{}:
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
			ui.Debug(ui.SymbolLogger, "    copying entry %s with %v", k, v)
			_ = s.SetAlways(k, vv)
		}
	}

	// Do it again with the constants
	ui.Debug(ui.SymbolLogger, "+++ Merging constants from  %s", st.Name)
	for k, v := range st.Constants {
		// Is it a struct? If so we may need to merge to it...
		switch vv := v.(type) {
		case map[string]interface{}:
			// Does the old struct already exist in the compiler table?
			old, found := s.Get(k)
			if found {
				// Is the existing value also a struct?
				switch oldmap := old.(type) {
				case map[string]interface{}:
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
