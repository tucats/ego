package runtime

// Definitions, etc.

const (
	// Type definition strings for runtime-created objects.

	// exec.Cmd type specification.
	commandTypeSpec = `
	type exec.Cmd struct {
		__cmd       interface{},
		Dir         string,
		Path		string,
		Args		[]string,
		Env			[]string,
		Stdout      []string,
		Stdin       []string,
	}`

	// db.Client type specification.
	dbTypeSpec = `
	type db.Client struct {
		client 		interface{},
		asStruct 	bool,
		rowCount 	int,
		transaction	interface{},
		constr 		string,
	}`

	// db.Rows type specification.
	dbRowsTypeSpec = `
	type db.Rows struct {
		client 	interface{},
		rows 	interface{},
		db 		interface{},
	}`

	// gremlin.Client type specification.
	gremlinTypeSpec = `
	type gremlin.Client struct {
		client	interface{}
	}`

	// rest.Client type specification.
	restTypeSpec = `
	type rest.Client struct {
		client 		interface{},
		baseURL 	string,
		mediaType 	string,
		response 	string,
		status 		int,
		verify 		bool,
		headers 	map[string]interface{},
	}`

	// tables.Table type specification.
	tableTypeSpec = `
	type tables.Table struct {
		table 	 interface{},
		headings []string,
	}`

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
