package scripting

type symbolTable struct {
	symbols map[string]any
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

	tableMetadataQuery = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0`

	resultSetSymbolName = "$$RESULT$$SET$$"
)
