package ast

// Kind is a small integer tag identifying a node's type. It lets consumers
// dispatch on node type cheaply, without reflection. The built-in kinds are
// enumerated below; the range at and above KindUserBase is reserved for node
// types defined outside this package so their tags cannot collide with kinds
// added here later.
type Kind int

// This const block uses Go's iota, which starts at 0 on the first line of a
// const(...) block and increases by one on every subsequent line inside the
// same block (blank lines and comment-only lines don't consume a step). So
// KindInvalid is 0, KindSelectStmt is 1, KindInsertStmt is 2, and so on down
// the list, purely from their position — nobody writes the numbers by hand.
// That also means inserting a new Kind in the middle of this list renumbers
// everything after it; since nothing here is persisted to disk or sent over
// the wire, that's safe. Appending new kinds at the end (rather than
// alphabetizing them into the middle) is still good practice so unrelated
// diffs don't touch unrelated lines.
const (
	// KindInvalid is the zero value and marks an uninitialized or unknown node.
	KindInvalid Kind = iota

	// --- Statements ---.
	KindSelectStmt
	KindInsertStmt
	KindUpdateStmt
	KindDeleteStmt
	KindCreateTableStmt
	KindDropTableStmt
	KindAlterTableStmt
	KindCreateIndexStmt
	KindDropIndexStmt
	KindCreateViewStmt
	KindDropViewStmt
	KindBeginStmt
	KindCommitStmt
	KindRollbackStmt
	KindSavepointStmt
	KindReleaseStmt

	// --- SELECT structure ---.
	KindSelectCore
	KindCompoundSelect
	KindWithClause
	KindCTE
	KindResultColumn
	KindTableRef
	KindSubqueryRef
	KindJoinClause
	KindOrderByTerm
	KindLimitClause

	// --- DML clauses ---.
	KindSetClause
	KindOnConflictClause
	KindReturningClause
	KindInsertValues
	KindInsertSelect
	KindInsertDefaultValues

	// --- DDL ---.
	KindColumnDef
	KindTypeName
	KindTablePrimaryKey
	KindTableUnique
	KindTableForeignKey
	KindTableCheck
	KindColumnPrimaryKey
	KindColumnNotNull
	KindColumnUnique
	KindColumnCheck
	KindColumnDefault
	KindColumnReferences
	KindColumnCollate
	KindColumnGenerated
	KindAddColumn
	KindDropColumn
	KindRenameColumn
	KindRenameTable

	// --- Expressions ---.
	KindColumnRef
	KindStarExpr
	KindLiteral
	KindPlaceholder
	KindUnaryExpr
	KindBinaryExpr
	KindBetweenExpr
	KindInExpr
	KindLikeExpr
	KindIsNullExpr
	KindIsExpr
	KindCollateExpr
	KindFuncCall
	KindCastExpr
	KindCaseExpr
	KindWhenClause
	KindParenExpr
	KindExistsExpr
	KindSubquery
	KindExprList

	// KindUserBase is the first Kind value reserved for node types defined
	// outside this package. Built-in kinds will never be assigned a value at
	// or above this constant, so external node types may allocate their own
	// kinds starting here without fear of collision with future additions.
	KindUserBase Kind = 10000
)

// kindNames maps built-in kinds to their names for String().
var kindNames = map[Kind]string{
	KindInvalid:             "Invalid",
	KindSelectStmt:          "SelectStmt",
	KindInsertStmt:          "InsertStmt",
	KindUpdateStmt:          "UpdateStmt",
	KindDeleteStmt:          "DeleteStmt",
	KindCreateTableStmt:     "CreateTableStmt",
	KindDropTableStmt:       "DropTableStmt",
	KindAlterTableStmt:      "AlterTableStmt",
	KindCreateIndexStmt:     "CreateIndexStmt",
	KindDropIndexStmt:       "DropIndexStmt",
	KindCreateViewStmt:      "CreateViewStmt",
	KindDropViewStmt:        "DropViewStmt",
	KindBeginStmt:           "BeginStmt",
	KindCommitStmt:          "CommitStmt",
	KindRollbackStmt:        "RollbackStmt",
	KindSavepointStmt:       "SavepointStmt",
	KindReleaseStmt:         "ReleaseStmt",
	KindSelectCore:          "SelectCore",
	KindCompoundSelect:      "CompoundSelect",
	KindWithClause:          "WithClause",
	KindCTE:                 "CTE",
	KindResultColumn:        "ResultColumn",
	KindTableRef:            "TableRef",
	KindSubqueryRef:         "SubqueryRef",
	KindJoinClause:          "JoinClause",
	KindOrderByTerm:         "OrderByTerm",
	KindLimitClause:         "LimitClause",
	KindSetClause:           "SetClause",
	KindOnConflictClause:    "OnConflictClause",
	KindReturningClause:     "ReturningClause",
	KindInsertValues:        "InsertValues",
	KindInsertSelect:        "InsertSelect",
	KindInsertDefaultValues: "InsertDefaultValues",
	KindColumnDef:           "ColumnDef",
	KindTypeName:            "TypeName",
	KindTablePrimaryKey:     "TablePrimaryKey",
	KindTableUnique:         "TableUnique",
	KindTableForeignKey:     "TableForeignKey",
	KindTableCheck:          "TableCheck",
	KindColumnPrimaryKey:    "ColumnPrimaryKey",
	KindColumnNotNull:       "ColumnNotNull",
	KindColumnUnique:        "ColumnUnique",
	KindColumnCheck:         "ColumnCheck",
	KindColumnDefault:       "ColumnDefault",
	KindColumnReferences:    "ColumnReferences",
	KindColumnCollate:       "ColumnCollate",
	KindColumnGenerated:     "ColumnGenerated",
	KindAddColumn:           "AddColumn",
	KindDropColumn:          "DropColumn",
	KindRenameColumn:        "RenameColumn",
	KindRenameTable:         "RenameTable",
	KindColumnRef:           "ColumnRef",
	KindStarExpr:            "StarExpr",
	KindLiteral:             "Literal",
	KindPlaceholder:         "Placeholder",
	KindUnaryExpr:           "UnaryExpr",
	KindBinaryExpr:          "BinaryExpr",
	KindBetweenExpr:         "BetweenExpr",
	KindInExpr:              "InExpr",
	KindLikeExpr:            "LikeExpr",
	KindIsNullExpr:          "IsNullExpr",
	KindIsExpr:              "IsExpr",
	KindCollateExpr:         "CollateExpr",
	KindFuncCall:            "FuncCall",
	KindCastExpr:            "CastExpr",
	KindCaseExpr:            "CaseExpr",
	KindWhenClause:          "WhenClause",
	KindParenExpr:           "ParenExpr",
	KindExistsExpr:          "ExistsExpr",
	KindSubquery:            "Subquery",
	KindExprList:            "ExprList",
}

// String returns the name of the kind. External kinds (>= KindUserBase) that
// are not registered render as "User(n)".
//
// Defining a String() string method makes Kind satisfy the standard
// fmt.Stringer interface. That's not visible anywhere in this file — but it
// means that anywhere a Kind value is passed to fmt.Printf/Sprintf/Println
// (with %v, %s, or by being an argument to Println), the fmt package
// notices the String() method and calls it automatically instead of
// printing the bare integer. The same pattern is used below for LitKind and
// PlaceholderStyle, and in ../token.go for tokenKind.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}

	if k >= KindUserBase {
		return "User(" + itoa(int(k)) + ")"
	}

	return "Kind(" + itoa(int(k)) + ")"
}

// itoa is a tiny integer-to-string formatter used by String() below (and by
// Dialect.String() in node.go) for the rare fallback case of an
// unregistered Kind. The standard library already provides this —
// strconv.Itoa — but this package defines its own copy, matching the same
// small helper in the sibling internal/language/parse/ast package, so the
// two AST packages stay stylistically interchangeable for anyone moving
// between them.
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
