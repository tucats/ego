package table

import (
	"sync"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
)

type Row []interface{}

var tableTypeDef *data.Type
var tableTypeDefLock sync.Mutex

func initTableTypeDef() {
	tableTypeDefLock.Lock()
	defer tableTypeDefLock.Unlock()

	if tableTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(tableTypeSpec)

		t.DefineFunctions(map[string]data.Function{
			"AddRow": {
				Declaration: &data.FunctionDeclaration{
					Name:         "AddRow",
					Variadic:     true,
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "value",
							ParmType: &data.InterfaceType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: AddRow,
			},

			"Close": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Close",
					ReceiverType: data.PointerType(t),
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: Close,
			},

			"Sort": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Sort",
					Variadic:     true,
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "columnName",
							ParmType: &data.StringType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: Sort,
			},

			"Print": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Print",
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "format",
							ParmType: &data.StringType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: TablePrint,
			},

			"Format": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Format",
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "headings",
							ParmType: data.BoolType,
						},
						{
							Name:     "underlines",
							ParmType: data.BoolType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: TableFormat,
			},

			"Align": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Align",
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "columnName",
							ParmType: &data.StringType,
						},
						{
							Name:     "alignment",
							ParmType: &data.StringType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: Align,
			},

			"String": {
				Declaration: &data.FunctionDeclaration{
					Name:         "String",
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "format",
							ParmType: &data.StringType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.StringType,
						&data.ErrorType,
					},
				},
				Value: String,
			},

			"Pagination": {
				Declaration: &data.FunctionDeclaration{
					Name:         "Pagination",
					ReceiverType: data.PointerType(t),
					Parameters: []data.FunctionParameter{
						{
							Name:     "width",
							ParmType: &data.IntType,
						},
						{
							Name:     "height",
							ParmType: &data.IntType,
						},
					},
					ReturnTypes: []*data.Type{
						&data.ErrorType,
					},
				},
				Value: Pagination,
			},
		})

		tableTypeDef = t
	}
}
