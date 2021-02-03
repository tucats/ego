package tables

import "fmt"

const (
	TableErrorPrefix          = "table processing"
	EmptyColumnListError      = "empty column list"
	IncorrectColumnCountError = "incorred number of columns: %d"
	InvalidAlignmentError     = "invalid alignment specification: %d"
	InvalidColumnNameError    = "invalid column name: %s"
	InvalidColumnNumberError  = "invalid column number: %d"
	InvalidColumnWidthError   = "invalid column width: %d"
	InvalidOutputFormatError  = "invalid output format: %s"
	InvalidRowNumberError     = "invalid row number: %d"
	InvalidSpacingError       = "invalid spacing value: %d"
)

type TableErr struct {
	err error
}

func NewTableErr(msg string, args ...interface{}) TableErr {
	return TableErr{
		err: fmt.Errorf(msg, args...),
	}
}

func (e TableErr) Error() string {
	return fmt.Sprintf("%s, %s", TableErrorPrefix, e.err.Error())
}
