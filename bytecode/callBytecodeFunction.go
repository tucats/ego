package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func callBytecodeFunction(c *Context, function *ByteCode, args []interface{}) error {
	var parentTable *symbols.SymbolTable

	isLiteral := function.IsLiteral()

	if isLiteral {
		parentTable = c.symbols
	} else {
		parentTable = c.symbols.FindNextScope()
	}

	functionSymbols := c.getPackageSymbols()
	if functionSymbols == nil {
		if parentTable == nil {
			parentTable = &symbols.SymbolTable{Name: "<none>"}
		}

		ui.Log(ui.SymbolLogger, "symbols.push.table", ui.A{
			"thread": c.threadID,
			"name":   c.symbols.Name,
			"parent": parentTable.Name})

		c.callFramePush("function "+function.name, function, 0, !isLiteral)
	} else {
		c.callFramePushWithTable(functionSymbols.Clone(parentTable), function, 0)
	}

	c.setAlways(defs.ArgumentListVariable,
		data.NewArrayFromInterfaces(data.InterfaceType, args...),
	)

	return nil
}
