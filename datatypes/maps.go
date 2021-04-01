package datatypes

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

// EgoMap is a wrapper around a native Go map. The actual map supports interface items
// for both key and value. The wrapper contains additional information about the expected
// types for key and value, as well as a counting semaphore to determine if the map
// should be considered immutable (such as during a for...range loop).
type EgoMap struct {
	data      map[interface{}]interface{}
	keyType   Type
	valueType Type
	immutable int
}

// Generate a new map value. The caller must supply the data type codes for the expected
// key and value types (such as datatypes.StringType or datatypes.FloatType). You can also
// use datatypes.InterfaceType for a type value, which means any type is accepted. The
// result is an initialized map that you can begin to store or read values from.
func NewMap(keyType Type, valueType Type) *EgoMap {
	m := &EgoMap{
		data:      map[interface{}]interface{}{},
		keyType:   keyType,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// ValueType returns the integer description of the declared key type for
// this map.
func (m *EgoMap) KeyType() Type {
	return m.keyType
}

// ValueType returns the integer description of the declared value type for
// this map.
func (m *EgoMap) ValueType() Type {
	return m.valueType
}

// ImmutableKeys marks the map as immutable. This is passed in as a boolean
// value (true means immutable). Internally, this is actually a counting
// semaphore, so the calls to ImmutableKeys to set/clear the state must
// be balanced to prevent having a map that is permanently locked or unlocked.
func (m *EgoMap) ImmutableKeys(b bool) {
	if b {
		m.immutable++
	} else {
		m.immutable--
	}
}

// Get reads a value from the map. The key value must be compatible with the
// type declaration of the map (no coercion occurs). This returns the actual
// value, or nil if not found. It also returns a flag indicating if the
// interface was found or not (i.e. should the result be considered value).
// Finally, it returns an error code if there is a type mismatch.
func (m *EgoMap) Get(key interface{}) (interface{}, bool, *errors.EgoError) {
	if IsType(key, m.keyType) {
		v, found := m.data[key]

		return v, found, nil
	}

	return nil, false, errors.New(errors.WrongMapKeyType).Context(key)
}

// Set sets a value in the map. The key value and type value must be compatible
// with the type declaration for the map. Bad type values result in an error.
// The function also returns a boolean indicating if the value replaced an
// existing item or not.
func (m *EgoMap) Set(key interface{}, value interface{}) (bool, *errors.EgoError) {
	if m.immutable > 0 {
		return false, errors.New(errors.ImmutableMapError)
	}

	if !IsBaseType(key, m.keyType) {
		return false, errors.New(errors.WrongMapKeyType).Context(key)
	}

	if !IsBaseType(value, m.valueType) {
		return false, errors.New(errors.WrongMapValueType).Context(value)
	}

	_, found := m.data[key]
	m.data[key] = value

	return found, nil
}

// Keys returns the set of keys for the map as an array. If the values are strings,
// ints, or floats they are returned in ascending sorted order.
func (m *EgoMap) Keys() []interface{} {
	if m.keyType.IsType(StringType) {
		idx := 0
		array := make([]string, len(m.data))

		for k := range m.data {
			array[idx] = GetString(k)
			idx++
		}

		sort.Strings(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(IntType) {
		idx := 0
		array := make([]int, len(m.data))

		for k := range m.data {
			array[idx] = GetInt(k)
			idx++
		}

		sort.Ints(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(FloatType) {
		idx := 0
		array := make([]float64, len(m.data))

		for k := range m.data {
			array[idx] = GetFloat(k)
			idx++
		}

		sort.Float64s(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else {
		r := []interface{}{}
		for k := range m.data {
			r = append(r, k)
		}

		return r
	}
}

// Delete will delete a given value from the map based on key. The return
// value indicates if the value was found (and therefore deleted) versus
// was not found.
func (m *EgoMap) Delete(key interface{}) (bool, *errors.EgoError) {
	if m.immutable > 0 {
		return false, errors.New(errors.ImmutableMapError)
	}

	if !IsType(key, m.keyType) {
		return false, errors.New(errors.WrongMapKeyType).Context(key)
	}

	_, found, err := m.Get(key)
	if errors.Nil(err) {
		delete(m.data, key)
	}

	return found, err
}

// TypeString produces a human-readable string describing the map type in Ego
// native terms.
func (m *EgoMap) TypeString() string {
	return fmt.Sprintf("map[%s]%s", m.keyType.String(), m.valueType.String())
}

// String displays a simplified formatted string value of a map, using the Ego
// anonymous struct syntax. Key values are not quoted, but data values are if
// they are strings.
func (m *EgoMap) String() string {
	var b strings.Builder

	b.WriteString("[")

	for i, k := range m.Keys() {
		v, _, _ := m.Get(k)

		if i > 0 {
			b.WriteString(", ")
		}

		if s, ok := v.(string); ok {
			b.WriteString(fmt.Sprintf("%s: \"%s\"", Format(k), s))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s", Format(k), Format(v)))
		}
	}

	b.WriteString("]")

	return b.String()
}

func (m EgoMap) Type() Type {
	return Type{
		name:      "map",
		kind:      MapKind,
		keyType:   &m.keyType,
		valueType: &m.valueType,
	}
}

// Given a map whose keys and values are simple types (string, int, float, bool),
// create a new EgoMap with the appropriate types, populated with the values from
// the source map.
func NewMapFromMap(sourceMap interface{}) *EgoMap {
	valueType := InterfaceType
	keyType := InterfaceType

	valueKind := reflect.TypeOf(sourceMap).Elem().Kind()
	keyKind := reflect.TypeOf(sourceMap).Key().Kind()

	switch valueKind {
	case reflect.Int, reflect.Int32, reflect.Int64:
		valueType = IntType

	case reflect.Float32, reflect.Float64:
		valueType = FloatType

	case reflect.Bool:
		valueType = BoolType

	case reflect.String:
		valueType = StringType
	}

	switch keyKind {
	case reflect.Int, reflect.Int32, reflect.Int64:
		keyType = IntType

	case reflect.Float32, reflect.Float64:
		keyType = FloatType

	case reflect.Bool:
		keyType = BoolType

	case reflect.String:
		keyType = StringType
	}

	result := NewMap(keyType, valueType)
	val := reflect.ValueOf(sourceMap)

	for _, key := range val.MapKeys() {
		value := val.MapIndex(key)
		_, _ = result.Set(key.Interface(), value.Interface())
	}

	return result
}
