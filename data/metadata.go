package data

import "github.com/tucats/ego/defs"

// MetadataPrefix is the string prefix that distinguishes internal metadata
// keys from user-visible field names in Ego maps and structs.  Any key that
// starts with this prefix is considered invisible to Ego code and is stripped
// out by Sanitize() before the data is serialized or returned to the user.
//
// Using defs.InvisiblePrefix (which is "__") means that user code cannot
// accidentally shadow or expose these internal keys — valid Ego identifiers
// cannot start with two underscores.
const MetadataPrefix = defs.InvisiblePrefix

// The Metadata Key constants below are the fully-prefixed key names used
// when storing bookkeeping information inside a native Go map that will be
// passed to NewStructFromMap, NewMapFromMap, or similar constructors.
//
// For example, to create a struct that carries explicit type information you
// would include the TypeMDKey entry in the map:
//
//	m := map[string]any{
//	    data.TypeMDKey: myType,
//	    "Name":         "Alice",
//	}
//	s := data.NewStructFromMap(m)
const (
	BaseTypeMDKey     = MetadataPrefix + BaseTypeMDName
	ElementTypesMDKey = MetadataPrefix + ElementTypesName
	MembersMDKey      = MetadataPrefix + MembersMDName
	ReadonlyMDKey     = MetadataPrefix + ReadonlyMDName
	SizeMDKey         = MetadataPrefix + SizeMDName
	StaticMDKey       = MetadataPrefix + StaticMDName
	SymbolsMDKey      = MetadataPrefix + SymbolsMDName
	TypeMDKey         = MetadataPrefix + TypeMDName
	EmbedMDKey        = MetadataPrefix + EmbedMDName
)

// The field-name constants below define the names used inside a
// reflect.Reflection{} struct (the object returned by the Ego "reflect"
// package).  They are also reused as the suffix part of the Metadata Key
// constants above.
//
// Not every field is populated for every kind of reflected value; the
// fields that are relevant depend on whether the reflected item is a
// function, type, variable, package, etc.
const (
	BaseTypeMDName    = "BaseType"      // the underlying base type of a user-defined type
	BuiltinsMDName    = "Builtins"      // true if the package contains built-in Go functions
	ContextMDName     = "Context"       // contextual information (e.g. error context)
	DeclarationMDName = "Declaration"   // the function signature as a string
	ElementTypesName  = "Elements"      // element type(s) for arrays or maps
	ErrorMDName       = "Error"         // error value, if any
	ImportsMDName     = "Imports"       // list of packages imported by this package
	IsTypeMDName      = "IsType"        // true if the reflected item is a type
	LineMDName        = "Line"          // source line number
	MembersMDName     = "Members"       // field or member names
	ModuleMDName      = "Module"        // module name
	NameMDName        = "Name"          // identifier name
	FunctionsMDName   = "Functions"     // list of functions/methods
	MethodMDName      = "Methods"       // alias used in some reflection paths
	NativeMDName      = "Native"        // true if the item is a native Go type
	PackageMDName     = "Package"       // package name that owns this item
	ReadonlyMDName    = "Readonly"      // true if the value is read-only (immutable)
	SizeMDName        = "Size"          // size in bytes or element count
	StaticMDName      = "Static"        // true if a struct has a fixed set of fields
	SymbolsMDName     = "Symbols"       // symbol-table contents
	TextMDName        = "Text"          // source text or display text
	TypeMDName        = "Type"          // type descriptor
	EmbedMDName       = "Embed"         // embedded type information
)
