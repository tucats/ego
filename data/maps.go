package data

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Map is a wrapper around a native Go map. The actual map supports interface items
// for both key and value. The wrapper contains additional information about the expected
// types for key and value, as well as a counting semaphore to determine if the map
// should be considered immutable (such as during a for...range loop).
type Map struct {
	data        map[interface{}]interface{}
	keyType     *Type
	elementType *Type
	immutable   int
	mutex       sync.RWMutex
}

// Generate a new map value. The caller must supply the data type codes for the expected
// key and value types (such as data.StringType or data.FloatType). You can also
// use data.InterfaceType for a type value, which means any type is accepted. The
// result is an initialized map that you can begin to store or read values from.
func NewMap(keyType, valueType *Type) *Map {
	return &Map{
		data:        map[interface{}]interface{}{},
		keyType:     keyType,
		elementType: valueType,
		immutable:   0,
	}
}

// ValueType returns the integer description of the declared key type for
// this map.
func (m *Map) KeyType() *Type {
	return m.keyType
}

// ElementType returns the integer description of the declared value type for
// this map.
func (m *Map) ElementType() *Type {
	return m.elementType
}

// SetReadonly marks the map as immutable. This is passed in as a boolean
// value (true means immutable). Internally, this is actually a counting
// semaphore, so the calls to SetReadonly to set/clear the state must
// be balanced to prevent having a map that is permanently locked or unlocked.
func (m *Map) SetReadonly(b bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

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
func (m *Map) Get(key interface{}) (interface{}, bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if IsType(key, m.keyType) {
		v, found := m.data[key]

		return v, found, nil
	}

	return nil, false, errors.ErrWrongMapKeyType.Context(key)
}

// Set sets a value in the map. The key value and type value must be compatible
// with the type declaration for the map. Bad type values result in an error.
// The function also returns a boolean indicating if the value replaced an
// existing item or not.
func (m *Map) Set(key interface{}, value interface{}) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.immutable > 0 {
		return false, errors.ErrImmutableMap
	}

	if !IsBaseType(key, m.keyType) {
		return false, errors.ErrWrongMapKeyType.Context(key)
	}

	if !IsBaseType(value, m.elementType) {
		return false, errors.ErrWrongMapValueType.Context(value)
	}

	_, found := m.data[key]
	m.data[key] = value

	return found, nil
}

// Keys returns the set of keys for the map as an array. If the values are strings,
// ints, or floats they are returned in ascending sorted order.
func (m *Map) Keys() []interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.keyType.IsType(StringType) {
		idx := 0
		array := make([]string, len(m.data))

		for k := range m.data {
			array[idx] = String(k)
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

		sort.Ints(array)

		for k := range m.data {
			array[idx] = Int(k)
			idx++
		}

		sort.Ints(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(Float64Type) {
		idx := 0
		array := make([]float64, len(m.data))

		sort.Float64s(array)

		for k := range m.data {
			array[idx] = Float64(k)
			idx++
		}

		sort.Float64s(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(Float32Type) {
		idx := 0
		array := make([]float64, len(m.data))

		for k := range m.data {
			array[idx] = Float64(k)
			idx++
		}

		sort.Float64s(array)

		result := make([]interface{}, len(array))

		for i, v := range array {
			result[i] = float32(v)
		}

		return result
	} else {
		r := []interface{}{}
		for _, k := range m.data {
			r = append(r, k)
		}

		return r
	}
}

// Delete will delete a given value from the map based on key. The return
// value indicates if the value was found (and therefore deleted) versus
// was not found.
func (m *Map) Delete(key interface{}) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.immutable > 0 {
		return false, errors.ErrImmutableMap
	}

	if !IsType(key, m.keyType) {
		return false, errors.ErrWrongMapKeyType.Context(key)
	}

	_, found, err := m.Get(key)
	if err == nil {
		delete(m.data, key)
	}

	return found, err
}

// TypeString produces a human-readable string describing the map type in Ego
// native terms.
func (m *Map) TypeString() string {
	return fmt.Sprintf("map[%s]%s", m.keyType.String(), m.elementType.String())
}

// String displays a simplified formatted string value of a map, using the Ego
// anonymous struct syntax. Key values are not quoted, but data values are if
// they are strings.
func (m *Map) String() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

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

// Type returns a type descriptor for the current map.
func (m *Map) Type() *Type {
	return &Type{
		name:      "map",
		kind:      MapKind,
		keyType:   m.keyType,
		valueType: m.elementType,
	}
}

// Given a map whose keys and values are simple types (string, int, float64, bool),
// create a new EgoMap with the appropriate types, populated with the values from
// the source map.
func NewMapFromMap(sourceMap interface{}) *Map {
	valueType := InterfaceType
	keyType := InterfaceType

	valueKind := reflect.TypeOf(sourceMap).Elem().Kind()
	keyKind := reflect.TypeOf(sourceMap).Key().Kind()

	switch valueKind {
	case reflect.Int, reflect.Int32, reflect.Int64:
		valueType = IntType

	case reflect.Float64:
		valueType = Float64Type

	case reflect.Float32:
		valueType = Float32Type

	case reflect.Bool:
		valueType = BoolType

	case reflect.String:
		valueType = StringType

	case reflect.Map:
		valueType = MapType(InterfaceType, InterfaceType)

	case reflect.Array:
		valueType = ArrayType(InterfaceType)
	}

	switch keyKind {
	case reflect.Int, reflect.Int32, reflect.Int64:
		keyType = IntType

	case reflect.Float32:
		keyType = Float32Type

	case reflect.Float64:
		keyType = Float64Type

	case reflect.Bool:
		keyType = BoolType

	case reflect.String:
		keyType = StringType
	}

	result := NewMap(keyType, valueType)
	val := reflect.ValueOf(sourceMap)

	for _, key := range val.MapKeys() {
		value := val.MapIndex(key).Interface()
		switch actual := value.(type) {
		case []interface{}:
			value = NewArrayFromInterfaces(InterfaceType, actual...)

		case map[string]interface{}:
			value = NewStructFromMap(actual)

		case map[interface{}]interface{}:
			value = NewMapFromMap(actual)
		}
		_, _ = result.Set(key.Interface(), value)
	}

	return result
}

// MarshalJSON is a helper function used by the JSON package to marshal
// an Ego map value. This is required to create and maintain the metadata
// for the Ego map in sync with the JSON stream.
func (m *Map) MarshalJSON() ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	b := strings.Builder{}
	b.WriteString("{")

	// Need to use the sorted list of names so results are deterministic,
	// as opposed to ranging over the fields directly.
	keys := m.Keys()
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}

		v, _, _ := m.Get(k)
		key := String(k)

		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, errors.NewError(err)
		}

		b.WriteString(fmt.Sprintf(`"%s":%s`, key, string(jsonBytes)))
	}

	b.WriteString("}")

	return []byte(b.String()), nil
}

// ToMap extracts the Ego map and converts it to a native map. This is needed
// to access an Ego map object from a native Go runtime. Currently this only
// supports making maps with string keys.
func (m *Map) ToMap() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	switch TypeOf(m.keyType).kind {
	case StringKind:
		result := map[string]interface{}{}

		for k, v := range m.data {
			result[String(k)] = DeepCopy(v)
		}

		return result
	}

	ui.Log(ui.InternalLogger, "Attempt to convert unsupported Ego map to native map, with key type %s", TypeOf(m.KeyType))

	return nil
}
