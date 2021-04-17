package runtime

// Definitions, etc.

const (
	// Type names for runtime types.

	databaseTypeDefinitionName     = "db.Client"
	databaseRowsTypeDefinitionName = "db.Rows"
	restTypeDefinitionName         = "rest.Client"
	tableTypeDefinitionName        = "tables.Table"

	// Field names for runtime types.

	asStructFieldName    = "asStruct"
	baseURLFieldName     = "baseURL"
	clientFieldName      = "client"
	constrFieldName      = "constr"
	dbFieldName          = "db"
	headersFieldName     = "headers"
	headingsFieldName    = "headings"
	mediaTypeFieldName   = "mediaType"
	responseFieldName    = "response"
	rowCountFieldName    = "rowCount"
	rowsFieldName        = "rows"
	statusFieldName      = "status"
	tableFieldName       = "table"
	transactionFieldName = "transaction"
	verifyFieldName      = "verify"
)
