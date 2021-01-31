package io

import (
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func SetConfig(s *symbols.SymbolTable, name string, value bool) {
	v, found := s.Get("_config")
	if !found {
		m := map[string]interface{}{name: value}
		_ = s.SetAlways("_config", m)
	}
	if m, ok := v.(map[string]interface{}); ok {
		m[name] = value
	}
}

func GetConfig(s *symbols.SymbolTable, name string) bool {
	f := false
	v, found := s.Get("_config")
	if found {
		if m, ok := v.(map[string]interface{}); ok {
			f, found := m[name]
			if found {
				return util.GetBool(f)
			}
		}
	}

	return f
}
