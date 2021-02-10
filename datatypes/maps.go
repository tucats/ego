package datatypes

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/errors"
)

type EgoMap struct {
	data      map[interface{}]interface{}
	keyType   int
	valueType int
	immutable int
}

func NewMap(keyType int, valueType int) *EgoMap {
	m := &EgoMap{
		data:      map[interface{}]interface{}{},
		keyType:   keyType,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

func (m *EgoMap) KeyType() int {
	return m.keyType
}

func (m *EgoMap) ValueType() int {
	return m.valueType
}

func (m *EgoMap) ImmutableKeys(b bool) {
	if b {
		m.immutable++
	} else {
		m.immutable--
	}
}

func (m *EgoMap) Get(key interface{}) (interface{}, bool, error) {
	if IsType(key, m.keyType) {
		v, found := m.data[key]

		return v, found, nil
	}

	return nil, false, errors.New(errors.WrongMapKeyType).WithContext(key)
}

func (m *EgoMap) Set(key interface{}, value interface{}) (bool, error) {
	if m.immutable > 0 {
		return false, errors.New(errors.ImmutableMapError)
	}

	if !IsType(key, m.keyType) {
		return false, errors.New(errors.WrongMapKeyType).WithContext(key)
	}

	if !IsType(value, m.valueType) {
		return false, errors.New(errors.WrongMapValueType).WithContext(value)
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

func (m *EgoMap) Delete(key interface{}) (bool, error) {
	if m.immutable > 0 {
		return false, errors.New(errors.ImmutableMapError)
	}

	if !IsType(key, m.keyType) {
		return false, errors.New(errors.WrongMapKeyType).WithContext(key)
	}

	_, found, err := m.Get(key)
	if err == nil {
		delete(m.data, key)
	}

	return found, err
}

func (m *EgoMap) TypeString() string {
	return fmt.Sprintf("map[%s]%s", TypeString(m.keyType), TypeString(m.valueType))
}

func (m *EgoMap) String() string {
	var b strings.Builder

	b.WriteString("{")

	for i, k := range m.Keys() {
		v, _, _ := m.Get(k)

		if i > 0 {
			b.WriteString(", ")
		}

		if s, ok := v.(string); ok {
			b.WriteString(fmt.Sprintf("%v: \"%s\"", k, s))
		} else {
			b.WriteString(fmt.Sprintf("%v: %s", k, Format(v)))
		}
	}

	b.WriteString("}")

	return b.String()
}
