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
