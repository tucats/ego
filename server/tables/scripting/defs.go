package scripting

type txError struct {
	Condition string `json:"condition"`
	Status    int    `json:"status"`
	Message   string `json:"msg"`
}

// This defines a single operation performed as part of a transaction.
type txOperation struct {
	Opcode     string                 `json:"operation"`
	Table      string                 `json:"table,omitempty"`
	Filters    []string               `json:"filters,omitempty"`
	Columns    []string               `json:"columns,omitempty"`
	EmptyError bool                   `json:"emptyError,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Errors     []txError              `json:"errors,omitempty"`
	SQL        string                 `json:"sql,omitempty"`
}

type symbolTable struct {
	symbols map[string]interface{}
}

const (
	symbolsOpcode = "symbols"
	insertOpcode  = "insert"
	deleteOpcode  = "delete"
	updateOpcode  = "update"
	dropOpCode    = "drop"
	selectOpcode  = "select"
	rowsOpcode    = "readrows"
	sqlOpcode     = "sql"

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
	insertVerb = "INSERT"

	tableMetadataQuery = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0`

	resultSetSymbolName = "$$RESULT$$SET$$"
)
