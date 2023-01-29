package data

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

type Struct struct {
	typeDef            *Type
	typeName           string
	static             bool
	readonly           bool
	strictTypeChecks   bool
	fromBuiltinPackage bool
	mutex              sync.RWMutex
	fields             map[string]interface{}
}

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
		if baseType.functions == nil {
			baseType.functions = t.functions
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
		typeDef:  baseType,
		static:   static,
		fields:   fields,
		typeName: typeName,
	}

	return &result
}

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
			fields[k] = v
		}
	}

	result := Struct{
		static:   static,
		typeDef:  t,
		readonly: readonly,
		fields:   fields,
	}

	return &result
}

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
		static:   static,
		typeDef:  t,
		readonly: readonly,
		fields:   fields,
	}

	return &result
}

func (s *Struct) FromBuiltinPackage() *Struct {
	s.fromBuiltinPackage = true

	return s
}

func (s *Struct) GetType() *Type {
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
		ui.WriteLog(ui.InfoLogger, "Fatal error - null struct pointer in SetAlways")

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
			return errors.ErrInvalidField
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

// Make a copy of the current structure object.
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

	return result
}

func (s *Struct) FieldNames() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0)

	for k := range s.fields {
		if s.fromBuiltinPackage && !hasCapitalizedName(k) {
			continue
		}

		if !strings.HasPrefix(k, MetadataPrefix) {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	return keys
}

func (s *Struct) FieldNamesArray() *Array {
	keys := s.FieldNames()
	keyValues := make([]interface{}, len(keys))

	for i, v := range keys {
		keyValues[i] = v
	}

	return NewArrayFromArray(StringType, keyValues)
}

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

	for k := range s.fields {
		if !strings.HasPrefix(k, MetadataPrefix) {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

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

func (s *Struct) Reflect() *Struct {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	m := map[string]interface{}{}

	m[TypeMDName] = s.TypeString()
	if s.typeDef.IsTypeDefinition() {
		m["basetype"] = s.typeDef.BaseType().String()
	} else {
		m["basetype"] = s.typeDef.String()
	}

	// If there are methods associated with this type, add them to the output structure.
	methods := s.typeDef.FunctionNames()
	if len(methods) > 0 {
		names := make([]interface{}, 0)

		for _, name := range methods {
			if name > "" {
				names = append(names, name)
			}
		}

		m["methods"] = NewArrayFromArray(StringType, names)
	}

	m["istype"] = false
	m["native"] = true
	m["members"] = s.FieldNamesArray()
	m["readonly"] = s.readonly
	m["static"] = s.static
	m["package"] = s.fromBuiltinPackage

	return NewStructFromMap(m)
}

func (s *Struct) MarshalJSON() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	b := strings.Builder{}
	b.WriteString("{")

	// Need to use the sorted list of names so results are deterministic,
	// as opposed to ranging over the fields directly.
	keys := s.FieldNames()
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}

		v := s.GetAlways(k)

		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, errors.NewError(err)
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
