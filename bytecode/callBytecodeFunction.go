package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func callBytecodeFunction(c *Context, function *ByteCode, args []any) error {
	var parentTable *symbols.SymbolTable

	isLiteral := function.IsLiteral()

	if isLiteral {
		if function.capturedScope != nil {
			parentTable = function.capturedScope
		} else {
			parentTable = c.symbols
		}
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

		// For a closure with a captured scope, create the function's symbol table
		// as a child of the captured scope (not c.symbols) so the closure can find
		// variables from the scope where it was defined even after that scope has
		// been popped from the active parent chain.
		if isLiteral && function.capturedScope != nil {
			table := symbols.NewChildSymbolTable("function "+function.name, parentTable).
				Shared(false).Boundary(false)
			c.callFramePushWithTable(table, function, 0)
		} else {
			c.callFramePush("function "+function.name, function, 0, !isLiteral)
		}
	} else {
		c.callFramePushWithTable(functionSymbols.Clone(parentTable), function, 0)
	}

	c.setAlways(defs.ArgumentListVariable,
		data.NewArrayFromInterfaces(data.InterfaceType, args...),
	)

	return nil
}
