package data

import "github.com/tucats/ego/defs"

// Common metadata keys.
const (
	MetadataPrefix = defs.InvisiblePrefix

	BasetypeMDName    = "basetype"
	BasetypeMDKey     = MetadataPrefix + BasetypeMDName
	BuiltinsMDName    = "builtins"
	ContextMDName     = "context"
	DeclarationMDName = "declaration"
	ElementTypesName  = "elements"
	ElementTypesMDKey = MetadataPrefix + ElementTypesName
	ErrorMDName       = "error"
	ImportsMDName     = "imports"
	IsTypeMDName      = "istype"
	MembersMDName     = "members"
	MembersMDKey      = MetadataPrefix + MembersMDName
	NameMDName        = "name"
	FunctionsMDName   = "functions"
	MethodMDName      = "methods"
	NativeMDName      = "native"
	PackageMDName     = "package"
	ReadonlyMDName    = "readonly"
	ReadonlyMDKey     = MetadataPrefix + ReadonlyMDName
	SizeMDName        = "size"
	SizeMDKey         = MetadataPrefix + SizeMDName
	StaticMDName      = "static"
	StaticMDKey       = MetadataPrefix + StaticMDName
	SymbolsMDName     = "symbols"
	SymbolsMDKey      = MetadataPrefix + SymbolsMDName
	TextMDName        = "text"
	TypeMDName        = "type"
	TypeMDKey         = MetadataPrefix + TypeMDName
)
