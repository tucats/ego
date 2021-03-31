package datatypes

import (
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

type EgoStruct struct {
	typeDef      Type
	typeName     string
	static       bool
	readonly     bool
	strongTyping bool
	replica      int
	fields       map[string]interface{}
}

func NewStruct(t Type) *EgoStruct {
	// IF this is a user type, get the base type.
	typeName := ""
	baseType := t

	for baseType.IsTypeDefinition() {
		typeName = t.name

		// If there are receiver functions in the type definition,
		// make them part of this structure as well.
		baseType = *baseType.BaseType()
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
	if baseType.fields == nil || len(baseType.fields) == 0 {
		static = false
	}

	// Create the fields structure, and fill it in using the field
	// names and the zero-type of each kind
	fields := map[string]interface{}{}

	for k, v := range baseType.fields {
		fields[k] = InstanceOfType(v)
	}

	// Create the structure and pass it back.
	result := EgoStruct{
		typeDef:  baseType,
		static:   static,
		fields:   fields,
		typeName: typeName,
	}

	return &result
}

func NewStructFromMap(m map[string]interface{}) *EgoStruct {
	t := Structure()

	if value, ok := GetMetadata(m, TypeMDKey); ok {
		t = GetType(value)
	} else {
		for k, v := range m {
			_ = t.DefineField(k, TypeOf(v))
		}
	}

	static := (len(m) > 0)
	if value, ok := GetMetadata(m, StaticMDKey); ok {
		static = GetBool(value)
	}

	readonly := false
	if value, ok := GetMetadata(m, ReadonlyMDKey); ok {
		readonly = GetBool(value)
	}

	fields := map[string]interface{}{}

	// Copy all the map items except any metadata items.
	for k, v := range m {
		if !strings.HasPrefix(k, MetadataPrefix) {
			fields[k] = v
		}
	}

	result := EgoStruct{
		static:   static,
		typeDef:  t,
		readonly: readonly,
		fields:   fields,
	}

	return &result
}

func (s *EgoStruct) GetType() Type {
	return s.typeDef
}

func (s *EgoStruct) IsReplica() *EgoStruct {
	s.replica++

	return s
}

func (s *EgoStruct) SetTyping(b bool) *EgoStruct {
	s.strongTyping = b

	return s
}

func (s *EgoStruct) SetReadonly(b bool) *EgoStruct {
	s.readonly = b

	return s
}

func (s *EgoStruct) SetStatic(b bool) *EgoStruct {
	s.static = b

	return s
}

// This is used only by the unit testing to explicitly set the type
// of a structure. It changes no data, only updates the type value.
func (s *EgoStruct) AsType(t Type) *EgoStruct {
	s.typeDef = t
	s.typeName = t.name

	return s
}

// GetAlways retrieves a value from the named field. No error checking is done
// to verify that the field exists; if it does not then a nil value is returned.
// This is a short-cut used in runtime code to access well-known fields from
// pre-defined object types, such as a db.Client().
func (s *EgoStruct) GetAlways(name string) interface{} {
	value := s.fields[name]

	return value
}
func (s *EgoStruct) Get(name string) (interface{}, bool) {
	value, ok := s.fields[name]

	// If its not a field, it might be locatable via the typedef's
	// declared receiver functions.
	if !ok {
		value, ok = s.typeDef.functions[name]
	}

	return value, ok
}

func (s EgoStruct) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	for k, v := range s.fields {
		result[k] = DeepCopy(v)
	}

	return result
}

// Store a value in the structure under the given name. This ignores type safety,
// static, or readonly attributes, so be VERY sure the value is the right type!
func (s *EgoStruct) SetAlways(name string, value interface{}) *EgoStruct {
	s.fields[name] = value

	return s
}

func (s *EgoStruct) Set(name string, value interface{}) *errors.EgoError {
	if s.readonly {
		return errors.New(errors.ReadOnlyError)
	}

	// Is it a readonly symbol name and it already exists? If so, fail...
	if name[0:1] == "_" {
		_, ok := s.fields[name]
		if ok {
			return errors.New(errors.ReadOnlyError)
		}
	}

	if s.static {
		_, ok := s.fields[name]
		if !ok {
			return errors.New(errors.InvalidFieldError)
		}
	}

	if s.typeDef.fields != nil {
		if t, ok := s.typeDef.fields[name]; ok {
			// Does it have to match already?
			if s.strongTyping && !IsType(value, t) {
				return errors.New(errors.InvalidTypeError)
			}
			// Make sure it is compatible with the field type.
			value = t.Coerce(value)
		}
	}

	s.fields[name] = value

	return nil
}

// Make a copy of the current structure object.
func (s EgoStruct) Copy() *EgoStruct {
	result := NewStructFromMap(s.fields)
	result.readonly = s.readonly
	result.static = s.static
	result.replica = s.replica + 1
	result.strongTyping = s.strongTyping

	return result
}

func (s EgoStruct) FieldNames() []string {
	keys := make([]string, 0)

	for k := range s.fields {
		if !strings.HasPrefix(k, MetadataPrefix) {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	return keys
}

func (s EgoStruct) FieldNamesArray() *EgoArray {
	keys := s.FieldNames()
	keyValues := make([]interface{}, len(keys))

	for i, v := range keys {
		keyValues[i] = v
	}

	return NewFromArray(StringType, keyValues)
}

func (s EgoStruct) TypeString() string {
	if s.typeName != "" {
		return s.typeName
	}

	if s.typeDef.IsTypeDefinition() {
		return s.typeDef.name
	}

	return s.typeDef.String()
}

func (s EgoStruct) String() string {
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

func (s EgoStruct) Reflect() *EgoStruct {
	m := map[string]interface{}{}

	m["type"] = s.TypeString()
	if s.typeDef.IsTypeDefinition() {
		m["basetype"] = s.typeDef.BaseType().String()
	} else {
		m["basetype"] = s.typeDef.String()
	}

	m["native"] = true
	m["members"] = s.FieldNamesArray()
	m["replicas"] = s.replica
	m["readonly"] = s.readonly
	m["static"] = s.static

	return NewStructFromMap(m)
}
