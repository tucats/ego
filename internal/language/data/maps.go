package data

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// Map is a wrapper around a native Go map. The actual map supports interface items
// for both key and value. The wrapper contains additional information about the expected
// types for key and value, as well as a counting semaphore to determine if the map
// should be considered immutable (such as during a for...range loop).
type Map struct {
	data        map[any]any
	keyType     *Type
	elementType *Type
	immutable   int
	mutex       sync.RWMutex
}

// NewMap creates an initialized, usable map with the given key and value types.
// The internal data store is allocated immediately, so values can be read and
// written without any additional initialization. Use this when you know the map
// will be populated right away (e.g., a composite literal "map[K]V{...}" or an
// explicit make() call).
func NewMap(keyType, valueType *Type) *Map {
	return &Map{
		data:        map[any]any{},
		keyType:     keyType,
		elementType: valueType,
		immutable:   0,
	}
}

// NewNilMap creates a typed nil map — a *Map wrapper whose internal data store
// is nil, matching Go's zero value for "var m map[K]V". The key and value types
// are retained so that type introspection (TypeString, reflect.Type comparisons,
// etc.) works correctly even though no data can be stored.
//
// Reading from a nil-state map returns the zero value (nil) with no error,
// matching Go's nil-map read semantics. Writing to a nil-state map via Set()
// returns ErrNilMapWrite, matching Go's "assignment to entry in nil map" panic
// (converted to a catchable Ego runtime error here).
//
// Trusted native Go callers that use SetAlways() to initialize a struct field's
// map (e.g. runtime/http header initialization) will auto-vivify the internal
// data store on first write, so they are unaffected by nil-state semantics.
func NewNilMap(keyType, valueType *Type) *Map {
	return &Map{
		// data is deliberately left as nil (Go's zero value for map[any]any).
		// This is the nil-state sentinel used by variable declarations without
		// an explicit initializer. All Map methods are nil-state-aware.
		keyType:     keyType,
		elementType: valueType,
	}
}

// ValueType returns the integer description of the declared key type for
// this map.
func (m *Map) KeyType() *Type {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

	return m.keyType
}

// ElementType returns the integer description of the declared value type for
// this map.
func (m *Map) ElementType() *Type {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

	return m.elementType
}

// IsReadonly reports whether the map is currently read-only (immutable counter > 0).
func (m *Map) IsReadonly() bool {
	if m == nil {
		return false
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.immutable > 0
}

// SetReadonly marks the map as immutable. This is passed in as a boolean
// value (true means immutable). Internally, this is actually a counting
// semaphore, so the calls to SetReadonly to set/clear the state must
// be balanced to prevent having a map that is permanently locked or unlocked.
func (m *Map) SetReadonly(b bool) {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.modify", nil)

		return
	}

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
func (m *Map) Get(key any) (any, bool, error) {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil, false, errors.ErrNilPointerReference
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.keyType == InterfaceType || IsType(key, m.keyType) {
		v, found := m.data[key]

		return v, found, nil
	}

	return nil, false, errors.ErrWrongMapKeyType.Context(key)
}

// Set sets a value in the map. The key value and type value must be compatible
// with the type declaration for the map. Bad type values result in an error.
// The function also returns a boolean indicating if the value replaced an
// existing item or not.
//
// If the map is in nil-state (declared via "var m map[K]V" but never assigned
// an initialized map), Set returns ErrNilMapWrite — a catchable Ego runtime
// error that mirrors Go's "assignment to entry in nil map" panic.
func (m *Map) Set(key any, value any) (bool, error) {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.modify", nil)

		return false, errors.ErrNilPointerReference
	}

	// A nil-state map (data == nil) was never initialized. Writes are
	// forbidden, matching Go's nil-map assignment semantics. This check must
	// come before the mutex lock because a nil-state map carries no data to
	// protect — returning early avoids acquiring an unnecessary lock.
	if m.data == nil {
		return false, errors.ErrNilMapWrite
	}

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

// SetAlways sets a value in the map. The key and value types are assumed to
// be correct; no type validation or immutability check is performed. This is
// the trusted native-Go bypass used by runtime packages that initialize
// struct-owned map fields directly (e.g. HTTP header maps).
//
// If the map is in nil-state (data == nil), the internal data store is
// lazily allocated before the write. This auto-vivification is intentional:
// SetAlways is called only by trusted native Go code, not by Ego scripts, so
// it does not need to enforce the "assignment to entry in nil map" restriction
// that Set() enforces for user-visible writes. Without auto-vivification,
// runtime packages that call SetAlways on a struct field's map (which starts
// nil-state after NewStruct) would trigger a native Go panic.
func (m *Map) SetAlways(key any, value any) *Map {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.modify", nil)

		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Lazily allocate the data store on the first trusted native write so
	// that struct fields of map type, which start nil-state after NewStruct,
	// become usable without requiring an explicit Ego-level initialization.
	if m.data == nil {
		m.data = map[any]any{}
	}

	m.data[key] = value

	return m
}

// IsNil returns true if the map has never been allocated (for example, a
// "var m map[K]V" declaration with no subsequent make() or assignment). This
// is distinct from a map that has been allocated but has no entries -- Set()
// rejects writes to a nil-state map with ErrNilMapWrite, but permits writes
// to an allocated, empty one.
func (m *Map) IsNil() bool {
	if m == nil {
		return true
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.data == nil
}

// Len returns the number of keys in the map.
func (m *Map) Len() int {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return 0
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.data)
}

// Keys returns the set of keys for the map as an array. If the values are strings,
// ints, or floats they are returned in ascending sorted order.
func (m *Map) Keys() []any {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

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

		result := make([]any, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(IntType) {
		idx := 0
		array := make([]int, len(m.data))

		sort.Ints(array)

		for k := range m.data {
			array[idx], _ = Int(k)
			idx++
		}

		sort.Ints(array)

		result := make([]any, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(Float64Type) {
		idx := 0
		array := make([]float64, len(m.data))

		sort.Float64s(array)

		for k := range m.data {
			array[idx], _ = Float64(k)
			idx++
		}

		sort.Float64s(array)

		result := make([]any, len(array))

		for i, v := range array {
			result[i] = v
		}

		return result
	} else if m.keyType.IsType(Float32Type) {
		idx := 0
		array := make([]float64, len(m.data))

		for k := range m.data {
			array[idx], _ = Float64(k)
			idx++
		}

		sort.Float64s(array)

		result := make([]any, len(array))

		for i, v := range array {
			result[i] = float32(v)
		}

		return result
	} else {
		r := []any{}
		for k := range m.data {
			r = append(r, k)
		}

		return r
	}
}

// Delete will delete a given value from the map based on key. The return
// value indicates if the value was found (and therefore deleted) versus
// was not found.
func (m *Map) Delete(key any) (bool, error) {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.modify", nil)

		return false, errors.ErrNilPointerReference
	}

	if key == nil {
		return false, errors.ErrNilPointerReference
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.immutable > 0 {
		return false, errors.ErrImmutableMap
	}

	if !IsType(key, m.keyType) {
		return false, errors.ErrWrongMapKeyType.Context(key)
	}

	_, found := m.data[key]
	if found {
		delete(m.data, key)
	}

	return found, nil
}

// TypeString produces a human-readable string describing the map type in Ego
// native terms.
func (m *Map) TypeString() string {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return defs.NilTypeString
	}

	return fmt.Sprintf("map[%s]%s", m.keyType.String(), m.elementType.String())
}

// String displays a simplified formatted string value of a map, using the Ego
// anonymous struct syntax. Key values are not quoted, but data values are if
// they are strings.
func (m *Map) String() string {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return defs.NilTypeString
	}

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
			b.WriteString(fmt.Sprintf("%s: %s", Format(k), strconv.Quote(s)))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s", Format(k), Format(v)))
		}
	}

	b.WriteString("]")

	return b.String()
}

// StringWithType displays a formatted string value of a map, using the Ego
// anonymous struct syntax. Key values are not quoted, but data values are if
// they are strings.
func (m *Map) StringWithType() string {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return defs.NilTypeString
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var b strings.Builder

	b.WriteString(m.TypeString())
	b.WriteString("{")

	for i, k := range m.Keys() {
		v, _, _ := m.Get(k)

		if i > 0 {
			b.WriteString(", ")
		}

		if s, ok := v.(string); ok {
			b.WriteString(fmt.Sprintf("%s: %s", Format(k), strconv.Quote(s)))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s", Format(k), Format(v)))
		}
	}

	b.WriteString("}")

	return b.String()
}

// Type returns a type descriptor for the current map.
func (m *Map) Type() *Type {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

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
func NewMapFromMap(sourceMap any) *Map {
	if sourceMap == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

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
		case []any:
			value = NewArrayFromInterfaces(InterfaceType, actual...)

		case map[string]any:
			value = NewStructFromMap(actual)

		case map[any]any:
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
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil, errors.ErrNilPointerReference
	}

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
			return nil, errors.New(err)
		}

		b.WriteString(fmt.Sprintf(`"%s":%s`, key, string(jsonBytes)))
	}

	b.WriteString("}")

	return []byte(b.String()), nil
}

// ToMap extracts the Ego map and converts it to a native map. This is needed
// to access an Ego map object from a native Go runtime. Currently this only
// supports making maps with string keys.
func (m *Map) ToMap() map[string]any {
	if m == nil {
		ui.Log(ui.InternalLogger, "runtime.map.nil.read", nil)

		return nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	switch TypeOf(m.keyType).kind {
	case StringKind:
		result := map[string]any{}

		for k, v := range m.data {
			result[String(k)] = DeepCopy(v)
		}

		return result
	}

	if ui.IsActive(ui.InternalLogger) {
		ui.Log(ui.InternalLogger, "runtime.map.tomap", ui.A{
			"type": TypeOf(m.KeyType).String()})
	}

	return nil
}
