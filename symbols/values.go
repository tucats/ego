package symbols

func (s *SymbolTable) initializeValues() {
	s.Values = make([][]interface{}, 1)
	s.Values[0] = make([]interface{}, SymbolAllocationSize)
	s.ValueSize = 0
}

func (s *SymbolTable) SetValue(index int, v interface{}) {
	bin := index / SymbolAllocationSize
	for bin >= len(s.Values) {
		s.Values = append(s.Values, make([]interface{}, SymbolAllocationSize))
	}

	slot := index % SymbolAllocationSize
	s.Values[bin][slot] = v
}

func (s *SymbolTable) GetValue(index int) interface{} {
	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.Values) {
		return nil
	}

	return s.Values[bin][slot]
}

func (s *SymbolTable) AddressOfValue(index int) *interface{} {
	bin := index / SymbolAllocationSize
	slot := index % SymbolAllocationSize

	if bin >= len(s.Values) {
		return nil
	}

	return &s.Values[bin][slot]
}
