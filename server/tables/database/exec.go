package database

import (
	"database/sql"
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// Exec is a shim to pass through to the underlying database handle.
func (d *Database) Exec(sqlText string, parameters ...any) (sql.Result, error) {
	if ui.IsActive(ui.SQLLogger) {
		if d.Session != nil && d.Session.ID > 0 {
			ui.Log(ui.SQLLogger, "sql.exec", ui.A{
				"session": d.Session.ID,
				"sql":     sqlText,
			})
		} else {
			ui.Log(ui.SQLLogger, "sql.exec", ui.A{
				"sql": sqlText,
			})
		}

		if len(parameters) > 0 {
			text := make([]string, len(parameters))
			for i, p := range parameters {
				text[i] = fmt.Sprintf("\n   $%d: %v", i+1, data.Format(p))
			}

			if d.Session != nil && d.Session.ID > 0 {
				ui.Log(ui.SQLLogger, "sql.parameters", ui.A{
					"session":    d.Session.ID,
					"parameters": text,
				})
			} else {
				ui.Log(ui.SQLLogger, "sql.parameters", ui.A{
					"parameters": text,
				})
			}
		}
	}

	if d.Transaction != nil {
		return d.Transaction.Exec(sqlText, parameters...)
	} else {
		return d.Handle.Exec(sqlText, parameters...)
	}
}
