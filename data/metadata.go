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
	EmbedMDKey        = MetadataPrefix + EmbedMDName
)

// Names of reflection information. These are used as field names in the
// reflect.Reflection{} structure created using the Ego reflect package.
// The struct has each of the field names, though not all are used for
// every reflection type.
const (
	BasetypeMDName    = "Basetype"
	BuiltinsMDName    = "Builtins"
	ContextMDName     = "Context"
	DeclarationMDName = "Declaration"
	ElementTypesName  = "Elements"
	ErrorMDName       = "Error"
	ImportsMDName     = "Imports"
	IsTypeMDName      = "Istype"
	LineMDName        = "Line"
	MembersMDName     = "Members"
	ModuleMDName      = "Module"
	NameMDName        = "Name"
	FunctionsMDName   = "Functions"
	MethodMDName      = "Methods"
	NativeMDName      = "Native"
	PackageMDName     = "Package"
	ReadonlyMDName    = "Readonly"
	SizeMDName        = "Size"
	StaticMDName      = "Static"
	SymbolsMDName     = "Symbols"
	TextMDName        = "Text"
	TypeMDName        = "Type"
	EmbedMDName       = "Embed"
)
