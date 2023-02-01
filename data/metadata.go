package data

import "github.com/tucats/ego/defs"

// Common metadata keys. These are used as key names for storing metadata
// in a native Go map that will be used to construct an Ego struct or map
// type. They allow the compiler (or whomever is generating the instance)
// to populate the map and then ask for an Ego map based on all the data in
// the map.
const (
	MetadataPrefix    = defs.InvisiblePrefix
	BasetypeMDKey     = MetadataPrefix + BasetypeMDName
	ElementTypesMDKey = MetadataPrefix + ElementTypesName
	MembersMDKey      = MetadataPrefix + MembersMDName
	ReadonlyMDKey     = MetadataPrefix + ReadonlyMDName
	SizeMDKey         = MetadataPrefix + SizeMDName
	StaticMDKey       = MetadataPrefix + StaticMDName
	SymbolsMDKey      = MetadataPrefix + SymbolsMDName
	TypeMDKey         = MetadataPrefix + TypeMDName
)

// Names of reflection information. These are used as field names in the
// reflect.Reflection{} structure created using the Ego reflect package.
// The struct has each of the field names, though not all are used for
// every reflection type.
const (
	BasetypeMDName    = "basetype"
	BuiltinsMDName    = "builtins"
	ContextMDName     = "context"
	DeclarationMDName = "declaration"
	ElementTypesName  = "elements"
	ErrorMDName       = "error"
	ImportsMDName     = "imports"
	IsTypeMDName      = "istype"
	MembersMDName     = "members"
	NameMDName        = "name"
	FunctionsMDName   = "functions"
	MethodMDName      = "methods"
	NativeMDName      = "native"
	PackageMDName     = "package"
	ReadonlyMDName    = "readonly"
	SizeMDName        = "size"
	StaticMDName      = "static"
	SymbolsMDName     = "symbols"
	TextMDName        = "text"
	TypeMDName        = "type"
)
