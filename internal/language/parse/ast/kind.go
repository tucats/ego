package ast

// Kind is a small integer tag identifying a node's type. It lets consumers
// dispatch on node type cheaply, without reflection. The built-in kinds are
// enumerated below; the range at and above KindUserBase is reserved for node
// types defined outside this package so their tags cannot collide with kinds
// added here later.
type Kind int

const (
	// KindInvalid is the zero value and marks an uninitialized or unknown node.
	KindInvalid Kind = iota

	// --- File / top level ---.
	KindFile

	// --- Declarations (SYNTAX.md section 5) ---.
	KindConstDecl
	KindConstSpec
	KindImportDecl
	KindImportSpec
	KindPackageDecl
	KindTypeDecl
	KindVarDecl
	KindVarSpec

	// --- Types (SYNTAX.md section 6) ---.
	KindPrimitiveType
	KindNamedType
	KindQualifiedType
	KindPointerType
	KindSliceType
	KindMapType
	KindFuncType
	KindInterfaceType
	KindStructType
	KindField

	// --- Functions (SYNTAX.md section 7) ---.
	KindFuncDecl
	KindFuncLit
	KindReceiver
	KindParam
	KindReturnItem

	// --- Statements (SYNTAX.md sections 3, 10, 11) ---.
	KindAssignStmt
	KindIncDecStmt
	KindSendStmt
	KindBlock
	KindBreakStmt
	KindContinueStmt
	KindDeferStmt
	KindGoStmt
	KindForStmt
	KindIfStmt
	KindReturnStmt
	KindSwitchStmt
	KindCaseClause
	KindTryStmt
	KindPanicStmt
	KindPrintStmt
	KindCallStmt
	KindThrowStmt
	KindExitStmt
	KindLabeledStmt
	KindExprStmt
	KindEmptyStmt
	KindDirectiveStmt

	// --- Expressions (SYNTAX.md sections 9, 14, 15) ---.
	KindBinaryExpr
	KindUnaryExpr
	KindStarExpr
	KindAddrExpr
	KindRecvExpr
	KindIdent
	KindBasicLit
	KindParenExpr
	KindSelectorExpr
	KindIndexExpr
	KindSliceExpr
	KindCallExpr
	KindTypeAssertExpr
	KindCompositeLit
	KindKeyValueExpr
	KindIfExpr
	KindOptionalExpr
	KindRangeLit

	// KindUserBase is the first Kind value reserved for node types defined
	// outside this package. Built-in kinds will never be assigned a value at or
	// above this constant, so external node types may allocate their own kinds
	// starting here without fear of collision with future additions.
	KindUserBase Kind = 10000
)

// kindNames maps built-in kinds to their names for String().
var kindNames = map[Kind]string{
	KindInvalid:        "Invalid",
	KindFile:           "File",
	KindConstDecl:      "ConstDecl",
	KindConstSpec:      "ConstSpec",
	KindImportDecl:     "ImportDecl",
	KindImportSpec:     "ImportSpec",
	KindPackageDecl:    "PackageDecl",
	KindTypeDecl:       "TypeDecl",
	KindVarDecl:        "VarDecl",
	KindVarSpec:        "VarSpec",
	KindPrimitiveType:  "PrimitiveType",
	KindNamedType:      "NamedType",
	KindQualifiedType:  "QualifiedType",
	KindPointerType:    "PointerType",
	KindSliceType:      "SliceType",
	KindMapType:        "MapType",
	KindFuncType:       "FuncType",
	KindInterfaceType:  "InterfaceType",
	KindStructType:     "StructType",
	KindField:          "Field",
	KindFuncDecl:       "FuncDecl",
	KindFuncLit:        "FuncLit",
	KindReceiver:       "Receiver",
	KindParam:          "Param",
	KindReturnItem:     "ReturnItem",
	KindAssignStmt:     "AssignStmt",
	KindIncDecStmt:     "IncDecStmt",
	KindSendStmt:       "SendStmt",
	KindBlock:          "Block",
	KindBreakStmt:      "BreakStmt",
	KindContinueStmt:   "ContinueStmt",
	KindDeferStmt:      "DeferStmt",
	KindGoStmt:         "GoStmt",
	KindForStmt:        "ForStmt",
	KindIfStmt:         "IfStmt",
	KindReturnStmt:     "ReturnStmt",
	KindSwitchStmt:     "SwitchStmt",
	KindCaseClause:     "CaseClause",
	KindTryStmt:        "TryStmt",
	KindPanicStmt:      "PanicStmt",
	KindPrintStmt:      "PrintStmt",
	KindCallStmt:       "CallStmt",
	KindThrowStmt:      "ThrowStmt",
	KindExitStmt:       "ExitStmt",
	KindLabeledStmt:    "LabeledStmt",
	KindExprStmt:       "ExprStmt",
	KindEmptyStmt:      "EmptyStmt",
	KindDirectiveStmt:  "DirectiveStmt",
	KindBinaryExpr:     "BinaryExpr",
	KindUnaryExpr:      "UnaryExpr",
	KindStarExpr:       "StarExpr",
	KindAddrExpr:       "AddrExpr",
	KindRecvExpr:       "RecvExpr",
	KindIdent:          "Ident",
	KindBasicLit:       "BasicLit",
	KindParenExpr:      "ParenExpr",
	KindSelectorExpr:   "SelectorExpr",
	KindIndexExpr:      "IndexExpr",
	KindSliceExpr:      "SliceExpr",
	KindCallExpr:       "CallExpr",
	KindTypeAssertExpr: "TypeAssertExpr",
	KindCompositeLit:   "CompositeLit",
	KindKeyValueExpr:   "KeyValueExpr",
	KindIfExpr:         "IfExpr",
	KindOptionalExpr:   "OptionalExpr",
	KindRangeLit:       "RangeLit",
}

// String returns the name of the kind. External kinds (>= KindUserBase) that
// are not registered render as "User(n)".
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}

	if k >= KindUserBase {
		return "User(" + itoa(int(k)) + ")"
	}

	return "Kind(" + itoa(int(k)) + ")"
}

// itoa is a tiny dependency-free integer formatter used by String().
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var buf [20]byte

	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}
