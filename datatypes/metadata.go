package datatypes

// Common metadata keys
const (
	MetadataKey = "__metadata"

	BasetypeMDKey     = "basetype"
	ElementTypesMDKey = "elements"
	MembersMDKey      = "members"
	ParentMDKey       = "parent"
	ReplicaMDKey      = "replica"
	ReadonlyMDKey     = "readonly"
	SizeMDKey         = "size"
	StaticMDKey       = "static"
	TypeMDKey         = "type"
)

// For a given structure, set a key/value in the metadata. The
// metadata member and it's map are created if necessary.
func SetMetadata(value interface{}, key string, v interface{}) bool {

	m, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	mdx, ok := m[MetadataKey]
	if !ok {
		m[MetadataKey] = map[string]interface{}{key: v}

		return true
	}
	mdxx, ok := mdx.(map[string]interface{})
	if !ok {
		return false
	}
	mdxx[key] = v

	return true
}

// For a given struct, fetch a metadata value by key. The boolean flag
// indicates if the value was found or has to be created.
func GetMetadata(value interface{}, key string) (interface{}, bool) {

	if m, ok := value.(map[string]interface{}); ok {
		if md, ok := m[MetadataKey]; ok {
			if mdx, ok := md.(map[string]interface{}); ok {
				v, ok := mdx[key]

				return v, ok
			}
		}
	}

	return nil, false
}
