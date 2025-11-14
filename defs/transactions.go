package defs

type TXError struct {
	Condition string `json:"condition"`
	Status    int    `json:"status"`
	Message   string `json:"msg"`
}

// This defines a single operation performed as part of a transaction.
type TXOperation struct {
	Opcode     string         `json:"operation"            validate:"required, enum=(symbols,insert,delete,update,drop,select,readrows,sql)"`
	Table      string         `json:"table,omitempty"`
	Filters    []string       `json:"filters,omitempty"`
	Columns    []string       `json:"columns,omitempty"`
	EmptyError bool           `json:"emptyError,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Errors     []TXError      `json:"errors,omitempty"`
	SQL        string         `json:"sql,omitempty"`
}
