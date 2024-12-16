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
		ui.Log(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
			c.threadID, c.symbols.Name, parentTable.Name)

		c.callframePush("function "+function.name, function, 0, !isLiteral)
	} else {
		c.callframePushWithTable(functionSymbols.Clone(parentTable), function, 0)
	}

	c.setAlways(defs.ArgumentListVariable,
		data.NewArrayFromInterfaces(data.InterfaceType, args...),
	)

	return nil
}
