package data

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// Struct describes an Ego structure. This includes the associated type
// definition if it is a struct that is part of a type, and the name of
// that type. It also has metadata indicating if the structure's definition
// is static or if fields can be added to it, and if the structure is
// considered read-only.
//
// Access to the struct is thread-safe, and will enforce package namespace
// visibility if the struct is defined in one package but referneced from
// outside that pacikage.
type Struct struct {
	typeDef            *Type
	typeName           string
	static             bool
	readonly           bool
	strictTypeChecks   bool
	fromBuiltinPackage bool
	mutex              sync.RWMutex
	fields             map[string]interface{}
	fieldOrder         []string
}

// Create a new Struct of the given type. This can be a type wrapper
// for the struct type, or data.StructType to indicate an anonymous
// struct value.
func NewStruct(t *Type) *Struct {
	// If this is a user type, get the base type.
	typeName := ""
	baseType := t

	for baseType.IsTypeDefinition() {
		typeName = t.name
		if baseType.pkg != "" {
			typeName = baseType.pkg + "." + typeName
		}

		// If there are receiver functions in the type definition,
		// make them part of this structure as well.
		baseType = baseType.BaseType()
		if t.functions != nil {
			baseType.functions = t.functions
		}

		if t.fields != nil {
			baseType.fields = t.fields
		}
	}

	// It must be a structure
	if baseType.kind != StructKind {
		return nil
	}

	// If there are fields defined, this is static.
	static := true
	if len(baseType.fields) == 0 {
		static = false
	}

	// Create the fields structure, and fill it in using the field
	// names and the zero-type of each kind
	fields := map[string]interface{}{}

	for k, v := range baseType.fields {
		fields[k] = InstanceOfType(v)
	}

	// Create the structure and pass it back.
	result := Struct{
		typeDef:    t,
		static:     static,
		fields:     fields,
		typeName:   typeName,
		fieldOrder: baseType.fieldOrder,
	}

	return &result
}

// NewStructFromMap will create an anonymous Struct (with no type),
// and populate the fields and their types and values based on the
// provided map. This is often used within Ego to build a map with
// the information needed in the structure, and then assign the
// associated values in the map to the fields of the structure.
// Note that the only fields that will be defined are the ones
// indicated by the map.
func NewStructFromMap(m map[string]interface{}) *Struct {
	t := StructureType()

	if value, ok := m[TypeMDKey]; ok {
		t = TypeOf(value)
	} else {
		for k, v := range m {
			t.DefineField(k, TypeOf(v))
		}
	}

	static := (len(m) > 0)
	if value, ok := m[StaticMDKey]; ok {
		static = Bool(value)
	}

	readonly := false
	if value, ok := m[ReadonlyMDKey]; ok {
		readonly = Bool(value)
	}

	fields := map[string]interface{}{}

	// Copy all the map items except any metadata items.
	for k, v := range m {
		if !strings.HasPrefix(k, MetadataPrefix) {
			switch actual := v.(type) {
			case []interface{}:
				v = NewArrayFromInterfaces(InterfaceType, actual...)

			case map[string]interface{}:
				v = NewStructFromMap(actual)

			case map[interface{}]interface{}:
				v = NewMapFromMap(actual)
			}

			fields[k] = v
		}
	}

	result := Struct{
		static:     static,
		typeDef:    t,
		readonly:   readonly,
		fields:     fields,
		fieldOrder: t.fieldOrder,
	}

	return &result
}

// NewStructOfTypeFromMap creates a new structure of a given type, and
// populates the fields using a map. The map does not have to have a value
// for each field in the new structure (the fields are determined by the
// type information), but the map cannot contain values that do not map
// to a structure field name.
func NewStructOfTypeFromMap(t *Type, m map[string]interface{}) *Struct {
	if value, ok := m[TypeMDKey]; ok {
		t = TypeOf(value)
	} else if t == nil {
		t = StructureType()
		for k, v := range m {
			t.DefineField(k, TypeOf(v))
		}
	}

	static := (len(m) > 0)
	if value, ok := m[StaticMDKey]; ok {
		static = Bool(value)
	}

	readonly := false
	if value, ok := m[ReadonlyMDKey]; ok {
		readonly = Bool(value)
	}

	fields := map[string]interface{}{}

	// Populate the map with all the required fields and
	// a nil value.
	typeFields := t.fields
	if typeFields == nil && t.BaseType() != nil {
		typeFields = t.BaseType().fields
	}

	for k, kt := range typeFields {
		if kt.Kind() != StructKind && kt.Kind() != TypeKind && kt.Kind() != ArrayKind && kt.kind != MapKind {
			fields[k] = InstanceOfType(kt)
		} else {
			fields[k] = nil
		}
	}

	// Copy all the map items except any metadata items. Make
	// sure the map value matches the field definition.
	for k, v := range m {
		if !strings.HasPrefix(k, MetadataPrefix) {
			f := t.fields[k]
			if f != nil {
				v = Coerce(v, InstanceOfType(f))
			}

			fields[k] = v
		}
	}

	result := Struct{
		static:             static,
		typeName:           t.name,
		typeDef:            t,
		readonly:           readonly,
		fields:             fields,
		fromBuiltinPackage: t.pkg != "",
		fieldOrder:         t.fieldOrder,
	}

	return &result
}

// FromBuiltinPackage sets the flag in the structure metadata that
// indicates that it should be considered a member of a package.
// This setting is used to enforce package visibility rules. The
// function returns the same pointer that was passed to it, so
// this can be chained with other operations on the structure.
func (s *Struct) FromBuiltinPackage() *Struct {
	s.fromBuiltinPackage = true

	return s
}

// Type returns the Type description of the structure.
func (s *Struct) Type() *Type {
	return s.typeDef
}

// SetStrictTypeChecks enables or disables strict type checking
// for field assignments. This is typically set when
// a structure is created.
func (s *Struct) SetStrictTypeChecks(b bool) *Struct {
	s.strictTypeChecks = b

	return s
}

// SetReadonly marks this structure as readonly.
func (s *Struct) SetReadonly(b bool) *Struct {
	s.readonly = b

	return s
}

// / SetFieldOrder sets the order of the fields in the structure.
func (s *Struct) SetFieldOrder(fields []string) *Struct {
	s.fieldOrder = fields

	return s
}

// SetStatic indicates if this structure is static. That is,
// once defined, fields cannot be added to the structure.
func (s *Struct) SetStatic(b bool) *Struct {
	s.static = b

	return s
}

// This is used only by the unit testing to explicitly set the type
// of a structure. It changes no data, only updates the type value.
func (s *Struct) AsType(t *Type) *Struct {
	s.typeDef = t
	s.typeName = t.name
	s.fieldOrder = t.fieldOrder

	return s
}

// GetAlways retrieves a value from the named field. No error checking is done
// to verify that the field exists; if it does not then a nil value is returned.
// This is a short-cut used in runtime code to access well-known fields from
// pre-defined object types, such as a db.Client().
func (s *Struct) GetAlways(name string) interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, ok := s.fields[name]
	// If it's not a field, it might be locatable via the typedef's
	// declared receiver functions.
	if !ok {
		value = s.typeDef.functions[name]
	}

	return value
}

// PackageName returns the package in which the structure was
// defined. This is used to compare against the current package
// code being executed to determine if private structure members
// are accessible.
func (s *Struct) PackageName() string {
	if s.typeDef.pkg != "" {
		return s.typeDef.pkg
	}

	parts := strings.Split(s.typeName, ".")
	if len(parts) > 1 {
		return parts[0]
	}

	return ""
}

// Get retrieves a field from the structure. Note that if the
// structure is a synthetic package type, we only allow access
// to exported names.
func (s *Struct) Get(name string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.fromBuiltinPackage && !hasCapitalizedName(name) {
		return nil, false
	}

	value, ok := s.fields[name]
	// If it's not a field, it might be locatable via the typedef's
	// declared receiver functions.
	if !ok {
		value, ok = s.typeDef.functions[name]
	}

	return value, ok
}

// ToMap generates a map[string]interface{} from the structure, so
// it's fields can be accessed natively, such as using a structure
// to hold the parameter data for a template evaluation when not in
// strict type checking mode.
func (s *Struct) ToMap() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := map[string]interface{}{}

	for k, v := range s.fields {
		result[k] = DeepCopy(v)
	}

	return result
}

// Store a value in the structure under the given name. This ignores type safety,
// static, or readonly attributes, so be VERY sure the value is the right type!
func (s *Struct) SetAlways(name string, value interface{}) *Struct {
	if s == nil {
		return s
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.fields[name] = value

	return s
}

// Set stores a value in the structure. The structure must not be
// readonly, and the field must not be a readonly field. If strict
// type checking is enabled, the type is validated.
func (s *Struct) Set(name string, value interface{}) error {
	if s.readonly {
		return errors.ErrReadOnly
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Is it a readonly symbol name and it already exists? If so, fail...
	if name[0:1] == defs.ReadonlyVariablePrefix {
		_, ok := s.fields[name]
		if ok {
			return errors.ErrReadOnly
		}
	}

	if s.static {
		_, ok := s.fields[name]
		if !ok {
			return errors.ErrInvalidField.Context(name)
		}
	}

	if s.typeDef.fields != nil {
		if t, ok := s.typeDef.fields[name]; ok {
			// Does it have to match already?
			if s.strictTypeChecks && !IsType(value, t) {
				return errors.ErrInvalidType.Context(TypeOf(value).String())
			}
			// Make sure it is compatible with the field type.
			value = t.Coerce(value)
		}
	}

	s.fields[name] = value

	return nil
}

// Make a copy of the current structure object. The resulting structure
// will be an exact duplicate, but allocated in new storage.
func (s *Struct) Copy() *Struct {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := NewStructFromMap(s.fields)
	result.typeDef = s.typeDef
	result.typeName = s.typeName
	result.readonly = s.readonly
	result.static = s.static
	result.strictTypeChecks = s.strictTypeChecks
	result.typeDef = s.typeDef
	result.typeName = s.typeName
	result.fromBuiltinPackage = s.fromBuiltinPackage
	result.fieldOrder = s.fieldOrder

	return result
}

// FieldNames returns an array of strings containing the names of the
// structure fields that can be accessed. The private flag is used to
// indicate if private structures should be visible. When this is false,
// the list of names is filtered to remove any private (lower-case) names.
func (s *Struct) FieldNames(private bool) []string {
	if len(s.fieldOrder) > 0 && !private {
		return s.fieldOrder
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0)

	for k := range s.fields {
		if !private && s.fromBuiltinPackage && !hasCapitalizedName(k) {
			continue
		}

		if !strings.HasPrefix(k, MetadataPrefix) {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	return keys
}

// FieldNamesArray constructs an Ego array of strings containing
// the names of the fields in the structure that can be accessed.
// The private flag is used to
// indicate if private structures should be visible. When this is false,
// the list of names is filtered to remove any private (lower-case) names.
func (s *Struct) FieldNamesArray(private bool) *Array {
	if len(s.fieldOrder) > 0 {
		fields := []interface{}{}
		for _, v := range s.fieldOrder {
			fields = append(fields, v)
		}

		return NewArrayFromInterfaces(StringType, fields...)
	}

	keys := s.FieldNames(private)
	keyValues := make([]interface{}, len(keys))

	for i, v := range keys {
		keyValues[i] = v
	}

	return NewArrayFromInterfaces(StringType, keyValues...)
}

// TypeString generates a string representation fo this current
// Type. If the structure is actually a typed structure, then the
// result is th  name of the type. Otherwise it is the full
// type string that enumerates the names of the fields and their
// types.
func (s *Struct) TypeString() string {
	if s.typeName != "" {
		return s.typeName
	}

	if s.typeDef.IsTypeDefinition() {
		name := s.typeDef.name
		if s.typeDef.pkg != "" {
			name = s.typeDef.pkg + "." + s.typeDef.name
		}

		return name
	}

	return s.typeDef.String()
}

// String creates a human-readable representation of a structure, showing
// the field names and their values.
func (s *Struct) String() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.fields) == 0 {
		return "{}"
	}

	keys := make([]string, 0)
	b := strings.Builder{}

	if s.typeName != "" {
		b.WriteString(s.typeName)
	}

	b.WriteString("{ ")

	if len(s.fieldOrder) > 0 {
		keys = append(keys, s.fieldOrder...)
	} else {
		for k := range s.fields {
			if !strings.HasPrefix(k, MetadataPrefix) {
				keys = append(keys, k)
			}
		}

		sort.Strings(keys)
	}

	for i, k := range keys {
		if s.fromBuiltinPackage && !hasCapitalizedName(k) {
			continue
		}

		if i > 0 {
			b.WriteString(", ")
		}

		v := s.fields[k]

		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(Format(v))
	}

	b.WriteString(" }")

	return b.String()
}

// StringWithType creates a human-readable representation of a structure,
// including the type definition, the field names and their values.
func (s *Struct) StringWithType() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.fields) == 0 {
		return "{}"
	}

	keys := make([]string, 0)
	b := strings.Builder{}

	b.WriteString(s.typeDef.TypeString())
	b.WriteString("{ ")

	if len(s.fieldOrder) > 0 {
		keys = append(keys, s.fieldOrder...)
	} else {
		for k := range s.fields {
			if !strings.HasPrefix(k, MetadataPrefix) {
				keys = append(keys, k)
			}
		}

		sort.Strings(keys)
	}

	for i, k := range keys {
		if s.fromBuiltinPackage && !hasCapitalizedName(k) {
			continue
		}

		if i > 0 {
			b.WriteString(", ")
		}

		v := s.fields[k]

		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(Format(v))
	}

	b.WriteString(" }")

	return b.String()
}

// MarshalJSON is a helper function that assists the json package
// operations that must generate the JSON sequence that represents
// the Ego structure (as opposed to the native Struct object itself).
func (s *Struct) MarshalJSON() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	b := strings.Builder{}
	b.WriteString("{")

	// Need to use the sorted list of names so results are deterministic,
	// as opposed to ranging over the fields directly.
	keys := s.FieldNames(false)
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}

		v := s.GetAlways(k)

		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, errors.New(err)
		}

		b.WriteString(fmt.Sprintf(`"%s":%s`, k, string(jsonBytes)))
	}

	b.WriteString("}")

	return []byte(b.String()), nil
}

// hasCapitalizedName returns true if the first rune/character of the
// string is considered a capital letter in Unicode.
func hasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

func (s *Struct) DeepEqual(v interface{}) bool {
	if v == nil {
		return false
	}

	s2, ok := v.(*Struct)
	if !ok {
		return false
	}

	if !s.typeDef.IsType(s2.typeDef) {
		return false
	}

	if len(s.fields) != len(s2.fields) {
		return false
	}

	for k, v := range s.fields {
		v2, ok := s2.fields[k]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(v, v2) {
			return false
		}
	}

	return true
}
