package db

// db.Client type specification.
const dbTypeSpec = `
type db.Client struct {
	client 		interface{},
	asStruct 	bool,
	rowCount 	int,
	transaction	interface{},
	constr 		string,
}`

// db.Rows type specification.
const dbRowsTypeSpec = `
type db.Rows struct {
	client 	interface{},
	rows 	interface{},
	db 		interface{},
}`

const (
	clientFieldName      = "client"
	constrFieldName      = "constr"
	dbFieldName          = "db"
	rowCountFieldName    = "rowCount"
	rowsFieldName        = "rows"
	asStructFieldName    = "asStruct"
	transactionFieldName = "transaction"
)
