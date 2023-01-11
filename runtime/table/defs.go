package table

// tables.Table type specification.
const tableTypeSpec = `
	type tables.Table struct {
		table 	 interface{},
		Headings []string,
	}`

const (
	headingsFieldName = "Headings"
	tableFieldName    = "table"
)
