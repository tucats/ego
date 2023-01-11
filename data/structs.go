package data

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

type Struct struct {
	typeDef            *Type
	typeName           string
	static             bool
	readonly           bool
	strongTyping       bool
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

func (s *Struct) FromBuiltinPackage() *Struct {
	s.fromBuiltinPackage = true

	return s
}

func (s *Struct) GetType() *Type {
	return s.typeDef
}

func (s *Struct) SetTyping(b bool) *Struct {
	s.strongTyping = b

	return s
}

func (s *Struct) SetReadonly(b bool) *Struct {
	s.readonly = b

	return s
}

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
		ui.Log(ui.InfoLogger, "Fatal error - null struct pointer in SetAlways")

		return s
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.fields[name] = value

	return s
}

func (s *Struct) Set(name string, value interface{}) error {
	if s.readonly {
		return errors.ErrReadOnly
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Is it a readonly symbol name and it already exists? If so, fail...
	if name[0:1] == "_" {
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
			if s.strongTyping && !IsType(value, t) {
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
	result.readonly = s.readonly
	result.static = s.static
	result.strongTyping = s.strongTyping
	result.typeDef = s.typeDef
	result.typeName = s.typeName

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
		return s.typeDef.name
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
