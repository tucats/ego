package datatypes

import "fmt"

// Common metadata keys.
const (
	MetadataKey = "__metadata"

	BasetypeMDKey     = "basetype"
	ElementTypesMDKey = "elements"
	MembersMDKey      = "members"
	ReplicaMDKey      = "replica"
	ReadonlyMDKey     = "readonly"
	SizeMDKey         = "size"
	StaticMDKey       = "static"
	SymbolsMDKey      = "symbols"
	TypeMDKey         = "type"
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

	if m, ok := object.(map[string]interface{}); ok {
		// Debugging check. We require that "type" be a Type value
		if key == TypeMDKey {
			if _, ok := v.(Type); !ok {
				fmt.Printf("DEBUG: Storing type other than Type: %v\n", v)
			}
		}

		metadataValue, ok := m[MetadataKey]
		if !ok {
			m[MetadataKey] = map[string]interface{}{key: v}

			return true
		}

		metadataMap, ok := metadataValue.(map[string]interface{})
		if !ok {
			return false
		}

		metadataMap[key] = v
		m[MetadataKey] = metadataMap
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

	if m, ok := value.(map[string]interface{}); ok {
		if md, ok := m[MetadataKey]; ok {
			if mdx, ok := md.(map[string]interface{}); ok {
				v, ok := mdx[key]

				// Debugging check.
				if ok && key == TypeMDKey {
					if _, ok := v.(Type); !ok {
						fmt.Printf("DEBUG: Retrieving type other than Type: %v\n", v)
					}
				}

				return v, ok
			}
		}
	}

	return nil, false
}
