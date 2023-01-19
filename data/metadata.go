package data

import "github.com/tucats/ego/defs"

// Common metadata keys.
const (
	MetadataPrefix = defs.InvisiblePrefix

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

// For a given package, set it's type value in the metadata. If the
// item is not a struct map then do no work.
func SetType(m *Package, t *Type) {
	SetMetadata(m, TypeMDKey, t)
}

// For a given structure, set a key/value in the metadata. The
// metadata member and it's map are created if necessary.
func SetMetadata(m *Package, key string, v interface{}) {
	m.Set(key, v)
}

// For a given struct, fetch a metadata value by key. The boolean flag
// indicates if the value was found or has to be created.
func GetMetadata(value *Package, key string) (interface{}, bool) {
	v, ok := value.Get(key)

	return v, ok
}
