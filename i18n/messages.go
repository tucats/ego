package i18n

// Internalized strings. The map is organized by language as the first key,
// and then the text ID passed into the i18n.T() function. If a key in a
// given language is not found, it reverts to using the "en" key.
var Messages = map[string]map[string]string{
	"en": {
		"ego.path.desc":              "Print the default ego path",
		"ego.desc":                   "run an Ego program",
		"ego.run.desc":               "Run an existing program",
		"ego.server.desc":            "Start to accept REST calls",
		"ego.sql.desc":               "Execute SQL in the database server",
		"ego.sql.file.desc":          "Filename of SQL command text",
		"ego.sql.row-ids.desc":       "Include the row UUID in any output",
		"ego.sql.row-numbers.desc":   "Include the row number in any output",
		"ego.table.desc":             "Operate on database tables",
		"ego.table.delete.desc":      "Delete rows from a table",
		"ego.table.drop.desc":        "Delete one or more tables",
		"ego.table.grant.desc":       "Set permissions for a given user and table",
		"ego.table.insert.desc":      "Insert a row to a table",
		"ego.table.list.desc":        "List tables",
		"ego.table.permission.desc":  "List table permissions",
		"ego.table.permissions.desc": "List all table permissions (requires admin privileges)",
		"ego.table.read.desc":        "Read contents of a table",
		"ego.table.show.desc":        "Show table metadata",
		"ego.table.sql.desc":         "Directly execute a SQL command",
		"ego.test.desc":              "Run a test suite",

		"error.label": "Error",

		"opt.sql.row.ids.desc":              "Include the row UUID in the output",
		"opt.sql.row.numbers.desc":          "Include the row number in the output",
		"opt.sql.file.desc":                 "Filename of SQL command text",
		"opt.filter.desc":                   "List of optional filter clauses",
		"opt.limit.desc":                    "If specified, limit the result set to this many rows",
		"opt.start.desc":                    "If specified, start result set at this row",
		"opt.table.delete.filter.desc":      "Filter for rows to delete. If not specified, all rows are deleted",
		"opt.table.grant.permission.desc":   "Permissions to set for this table updated",
		"opt.table.grant.user.desc":         "User (if other than current user) to update",
		"opt.table.insert.file.desc":        "File name containing JSON row info",
		"opt.table.list.no.row.counts.desc": "If specified, listing does not include row counts",
		"opt.table.permission.user.desc":    "User (if other than current user) to list)",
		"opt.table.permissions.user.desc":   "If specified, list only this user",
		"opt.table.read.columns.desc":       "List of optional column names to display; if not specified, all columns are returned",
		"opt.table.read.order.by.desc":      "List of optional columns use to sort output",
		"opt.table.read.row.ids.desc":       "Include the row UUID column in the output",
		"opt.table.read.row.numbers.desc":   "Include the row number in the output",

		"parm.file.desc":         "file",
		"parm.file.or.path.desc": "file or path",
		"parm.sql.text.desc":     "sql-text",
		"parm.table.name.desc":   "table-name",
		"parm.table.insert.desc": "table-name [column=value...]",

		"version.parse.error": "Unable to process version number {{v}; count={{c}}, err={{err}\n",
	},
}
