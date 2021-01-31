package datatypes

import "errors"

type EgoMap struct {
	data      map[interface{}]interface{}
	keyType   int
	valueType int
}

func NewMap(keyType int, valueType int) *EgoMap {

	m := &EgoMap{
		data:      map[interface{}]interface{}{},
		keyType:   keyType,
		valueType: valueType,
	}

	return m
}

func (m *EgoMap) Get(key interface{}) (interface{}, bool, error) {
	if IsType(key, m.keyType) {
		v, found := m.data[key]

		return v, found, nil
	}

	return nil, false, errors.New(WrongMapKeyType)
}

func (m *EgoMap) Set(key interface{}, value interface{}) (bool, error) {
	if !IsType(key, m.keyType) || !IsType(value, m.valueType) {
		return false, errors.New(WrongMapKeyType)
	}
	_, found := m.data[key]
	m.data[key] = value

	return found, nil
}

func (m *EgoMap) Keys() []interface{} {
	r := []interface{}{}
	for k := range m.data {
		r = append(r, k)
	}

	return r
}
