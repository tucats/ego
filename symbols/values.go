package symbols

import "github.com/tucats/ego/app-cli/ui"

func (s *SymbolTable) initializeValues() {
	bin := make([]interface{}, SymbolAllocationSize)
	s.Values = make([]*[]interface{}, 1)
	s.Values[0] = &bin
	s.ValueSize = 0
}

func (s *SymbolTable) SetValue(index int, v interface{}) {
	bin := index / SymbolAllocationSize
	for bin >= len(s.Values) {
		newBin := make([]interface{}, SymbolAllocationSize)
		s.Values = append(s.Values, &newBin)

		ui.Debug(ui.SymbolLogger, "+++ in table %s, create new value bin", s.Name)
	}

	slot := index % SymbolAllocationSize
	(*s.Values[bin])[slot] = v
}

func (s *SymbolTable) GetValue(index int) interface{} {
	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.Values) {
		return nil
	}

	return (*s.Values[bin])[slot]
}

func (s *SymbolTable) AddressOfValue(index int) *interface{} {
	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.Values) {
		return nil
	}

	return &(*s.Values[bin])[slot]
}
