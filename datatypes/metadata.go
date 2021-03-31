package datatypes

// Common metadata keys.
const (
	MetadataPrefix = "__"

	BasetypeMDName    = "basetype"
	BasetypeMDKey     = MetadataPrefix + BasetypeMDName
	ElementTypesName  = "elements"
	ElementTypesMDKey = MetadataPrefix + ElementTypesName
	MembersMDName     = "members"
	MembersMDKey      = MetadataPrefix + MembersMDName
	ReplicaMDName     = "replica"
	ReplicaMDKey      = MetadataPrefix + ReplicaMDName
	ReadonlyMDName    = "readonly"
	ReadonlyMDKey     = MetadataPrefix + ReadonlyMDName
	SizeMDName        = "size"
	SizeMDKey         = MetadataPrefix + SizeMDName
	StaticMDName      = "static"
	StaticMDKey       = MetadataPrefix + StaticMDName
	SymbolsMDName     = "symbols"
	SymbolsMDKey      = MetadataPrefix + SymbolsMDName
	TypeMDName        = "type"
	TypeMDKey         = MetadataPrefix + TypeMDName
)

// For a given struct type, set it's type value in the metadata. If the
// item is not a struct map then do no work.
func SetType(m map[string]interface{}, t Type) {
	SetMetadata(m, TypeMDKey, t)
}

// For a given structure, set a key/value in the metadata. The
// metadata member and it's map are created if necessary.
func SetMetadata(object interface{}, key string, v interface{}) bool {
	if m, ok := object.(*EgoStruct); ok {
		switch key {
		case ReadonlyMDKey:
			m.readonly = GetBool(v)

		case StaticMDKey:
			m.static = GetBool(v)

		case ReplicaMDKey:
			m.replica = GetInt(v)

		default:
			return false
		}

		return true
	}

	// This is/should be a package.
	if m, ok := object.(map[string]interface{}); ok {
		if !ok {
			return false
		}

		m[key] = v
	}

	return true
}

// For a given struct, fetch a metadata value by key. The boolean flag
// indicates if the value was found or has to be created.
func GetMetadata(value interface{}, key string) (interface{}, bool) {
	if s, ok := value.(*EgoStruct); ok {
		switch key {
		case TypeMDKey:
			return s.typeDef, true

		case ReadonlyMDKey:
			return s.readonly, true

		case ReplicaMDKey:
			return s.replica, true

		case StaticMDKey:
			return s.static, true

		default:
			return nil, false
		}
	}

	// This is a package.
	if m, ok := value.(map[string]interface{}); ok {
		v, ok := m[key]

		return v, ok
	}

	return nil, false
}
