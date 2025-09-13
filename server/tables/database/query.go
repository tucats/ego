package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// Query is a shim to pass through to the underlying database handle.
func (d *Database) Query(sqlText string, parameters ...any) (*sql.Rows, error) {
	if ui.IsActive(ui.SQLLogger) {
		if strings.HasPrefix(sqlText, "SELECT * FROM ") && strings.HasSuffix(sqlText, " WHERE 1=0") {
			table := strings.TrimPrefix(strings.TrimSuffix(sqlText, " WHERE 1=0"), "SELECT * FROM ")

			if d.Session != nil && d.Session.ID > 0 {
				ui.Log(ui.SQLLogger, "sql.metadata.query", ui.A{
					"session": d.Session.ID,
					"table":   table,
				})
			} else {
				ui.Log(ui.SQLLogger, "sql.metadata.query", ui.A{
					"table": table,
				})
			}
		} else {
			if d.Session != nil && d.Session.ID > 0 {
				ui.Log(ui.SQLLogger, "sql.query", ui.A{
					"session": d.Session.ID,
					"sql":     sqlText,
				})
			} else {
				ui.Log(ui.SQLLogger, "sql.query", ui.A{
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
	}

	if d.Transaction != nil {
		return d.Transaction.Query(sqlText, parameters...)
	} else {
		return d.Handle.Query(sqlText, parameters...)
	}
}
