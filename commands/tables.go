package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	filterParseError = "==error== "
)

func TableList(c *cli.Context) error {
	resp := defs.TableInfo{}

	rowCounts := true
	if c.WasFound("no-row-counts") {
		rowCounts = false
	}

	url := rest.URLBuilder(defs.TablesPath)
	if parms := c.FindGlobal().Parameters; len(parms) > 0 && settings.GetBool(defs.TableAutoparseDSN) {
		dsn := parms[0]
		url = rest.URLBuilder(defs.DSNTablesPath, dsn)
	}

	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		url = rest.URLBuilder(defs.DSNTablesPath, dsn)
	}

	if dsn, found := c.String("dsn"); found {
		url = rest.URLBuilder(defs.DSNTablesPath, dsn)
	}

	if limit, found := c.Integer("limit"); found {
		url.Parameter(defs.LimitParameterName, limit)
	}

	if start, found := c.Integer("start"); found {
		url.Parameter(defs.StartParameterName, start)
	}

	if !rowCounts {
		url.Parameter(defs.RowCountParameterName, false)
	}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.TablesMediaType)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)
		} else {
			if ui.OutputFormat == ui.TextFormat {
				if rowCounts {
					t, _ := tables.New([]string{i18n.L("Schema"), i18n.L("Name"), i18n.L("Columns"), i18n.L("Rows")})
					_ = t.SetOrderBy(i18n.L("Name"))
					_ = t.SetAlignment(2, tables.AlignmentRight)
					_ = t.SetAlignment(3, tables.AlignmentRight)

					for _, row := range resp.Tables {
						_ = t.AddRowItems(row.Schema, row.Name, row.Columns, row.Rows)
					}

					t.Print(ui.OutputFormat)
				} else {
					t, _ := tables.New([]string{i18n.L("Schema"), i18n.L("Name"), i18n.L("Columns")})
					_ = t.SetOrderBy(i18n.L("Name"))
					_ = t.SetAlignment(2, tables.AlignmentRight)

					for _, row := range resp.Tables {
						_ = t.AddRowItems(row.Schema, row.Name, row.Columns)
					}

					t.Print(ui.OutputFormat)
				}
			} else {
				_ = commandOutput(resp)
			}
		}
	}

	if err != nil {
		err = errors.New(err)

		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(resp)
		}
	}

	return err
}

func TableShow(c *cli.Context) error {
	resp := defs.TableColumnsInfo{}
	table := c.Parameter(0)

	urlString := rest.URLBuilder(defs.TablesNamePath, table).String()
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
	}

	if dsn, found := c.String("dsn"); found {
		urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]

		urlString = rest.URLBuilder(defs.DSNTablesNamePath, schema, table).String()
	}

	err := rest.Exchange(urlString, http.MethodGet, nil, &resp, defs.TableAgent, defs.TableMetadataMediaType)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)
		} else {
			if ui.OutputFormat == ui.TextFormat {
				t, _ := tables.New([]string{
					i18n.L("Name"),
					i18n.L("Type"),
					i18n.L("Size"),
					i18n.L("Nullable"),
					i18n.L("Unique"),
				})
				_ = t.SetOrderBy(i18n.L("Name"))
				_ = t.SetAlignment(2, tables.AlignmentRight)

				for _, row := range resp.Columns {
					nullable := "default"
					unique := "default"

					if row.Nullable.Specified {
						if row.Nullable.Value {
							nullable = "yes"
						} else {
							nullable = "no"
						}
					}

					if row.Unique.Specified {
						if row.Unique.Value {
							unique = "yes"
						} else {
							unique = "no"
						}
					}

					_ = t.AddRowItems(row.Name, row.Type, row.Size, nullable, unique)
				}

				t.Print(ui.OutputFormat)
			} else {
				_ = commandOutput(resp)
			}
		}
	}

	if err != nil {
		err = errors.New(err)

		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(resp)
		}
	}

	return err
}

func TableDrop(c *cli.Context) error {
	var (
		count int
		err   error
		table string
		resp  = defs.TableColumnsInfo{}
	)

	for i := 0; i < 999; i++ {
		table = c.Parameter(i)
		if table == "" {
			break
		}

		urlString := rest.URLBuilder(defs.TablesNamePath, table).String()
		if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
			urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
		}

		if dsn, found := c.String("dsn"); found {
			urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
		} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
			parts := strings.SplitN(table, ".", 2)
			schema := parts[0]
			table = parts[1]

			urlString = rest.URLBuilder(defs.DSNTablesNamePath, schema, table).String()
		}

		err = rest.Exchange(urlString, http.MethodDelete, nil, &resp, defs.TableAgent)
		if err == nil {
			if resp.Status > 200 {
				err = errors.Message(resp.Message)
			} else {
				count++

				ui.Say("msg.table.deleted", map[string]interface{}{"name": table})
			}
		} else {
			break
		}
	}

	if err == nil && count > 1 {
		ui.Say("msg.table.delete.count", map[string]interface{}{"count": count})
	} else if err != nil {
		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(resp)
		}

		return errors.New(err)
	}

	return nil
}

func TableContents(c *cli.Context) error {
	resp := defs.DBRowSet{}
	table := c.Parameter(0)
	url := rest.URLBuilder(defs.TablesRowsPath, table)

	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	}

	if dsn, found := c.String("dsn"); found {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]

		url = rest.URLBuilder(defs.DSNTablesRowsPath, schema, table)
	}

	if columns, ok := c.StringList("columns"); ok {
		url.Parameter(defs.ColumnParameterName, toInterfaces(columns)...)
	}

	if order, ok := c.StringList("order-by"); ok {
		url.Parameter(defs.SortParameterName, toInterfaces(order)...)
	}

	if limit, found := c.Integer("limit"); found {
		url.Parameter(defs.LimitParameterName, limit)
	}

	if start, found := c.Integer("start"); found {
		url.Parameter(defs.StartParameterName, start)
	}

	if filter, ok := c.StringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			url.Parameter(defs.FilterParameterName, f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)

			return errors.Message(msg)
		}
	}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.RowSetMediaType)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)
		} else {
			err = printRowSet(resp, c.Boolean("row-ids"), c.Boolean("row-numbers"))
		}
	}

	if err != nil {
		err = errors.New(err)

		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(resp)
		}
	}

	return err
}

func printRowSet(resp defs.DBRowSet, showRowID bool, showRowNumber bool) error {
	if ui.OutputFormat == ui.TextFormat {
		if len(resp.Rows) == 0 {
			ui.Say("msg.table.empty.rowset")

			return nil
		}

		keys := make([]string, 0)

		for k := range resp.Rows[0] {
			if k == defs.RowIDName && !showRowID {
				continue
			}

			keys = append(keys, k)
		}

		sort.Strings(keys)

		t, _ := tables.New(keys)
		t.ShowRowNumbers(showRowNumber)

		for _, row := range resp.Rows {
			values := make([]interface{}, 0)

			for _, key := range keys {
				if key == defs.RowIDName && !showRowID {
					continue
				}

				values = append(values, row[key])
			}

			_ = t.AddRowItems(values...)
		}

		t.Print(ui.OutputFormat)
	} else {
		_ = commandOutput(resp)
	}

	return nil
}

func TableInsert(c *cli.Context) error {
	resp := defs.DBRowCount{}
	table := c.Parameter(0)
	payload := map[string]interface{}{}

	// If there is a JSON file to initialize the payload with, do it now.
	if c.WasFound("file") {
		fn, _ := c.String("file")

		b, err := os.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		if err = json.Unmarshal(b, &payload); err != nil {
			return errors.New(err)
		}
	}

	for i := 1; i < 999; i++ {
		p := c.Parameter(i)
		if p == "" {
			break
		}

		// Tokenize the parameter, which should be of the form <id> = <value>
		t := tokenizer.New(p, false)

		// Get the column name, and confirm that it's an identifier.
		column := t.Next()
		if !column.IsIdentifier() {
			return errors.ErrInvalidIdentifier.Context(column)
		}

		columnName := column.Spelling()

		// Must be followed by a "=" token
		if !t.IsNext(tokenizer.AssignToken) {
			return errors.ErrMissingAssignment
		}

		// Rest of the string is handled without regard for
		// the text, as we want to treat the remainder of the
		// parameter as the value, regardless of how it tokenized.
		value := t.Remainder()

		if strings.EqualFold(strings.TrimSpace(value), defs.True) {
			payload[columnName] = true
		} else if strings.EqualFold(strings.TrimSpace(value), defs.False) {
			payload[columnName] = false
		} else if i, err := strconv.Atoi(value); err == nil {
			payload[columnName] = i
		} else {
			payload[columnName] = value
		}
	}

	if len(payload) == 0 {
		ui.Say("msg.tables.no.insert")

		return nil
	}

	urlString := rest.URLBuilder(defs.TablesRowsPath, table).String()
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		urlString = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table).String()
	}

	if dsn, found := c.String("dsn"); found {
		urlString = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table).String()
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]

		urlString = rest.URLBuilder(defs.DSNTablesRowsPath, schema, table).String()
	}

	err := rest.Exchange(urlString, http.MethodPut, payload, &resp, defs.TableAgent)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)

			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}
		} else {
			ui.Say("msg.table.insert.count", map[string]interface{}{
				"count": resp.Count,
				"name":  table,
			})
		}

		return err
	}

	err = errors.New(err)

	return err
}

func TableCreate(c *cli.Context) error {
	table := c.Parameter(0)
	fields := map[string]defs.DBColumn{}
	payload := make([]defs.DBColumn, 0)
	resp := defs.DBRowCount{}

	defined := map[string]bool{}

	// If the user specified a file with a JSON payload, use that to seed the
	// field definitions map. We will also look for command line parameters to
	// extend or modify the field list.
	if c.WasFound("file") {
		fn, _ := c.String("file")

		b, err := os.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		if err = json.Unmarshal(b, &payload); err != nil {
			return errors.New(err)
		}

		// Move the info read in to a map so we can replace fields from
		// the command line if specified.
		for _, field := range payload {
			fields[field.Name] = field
		}
	}

	for i := 1; i < 999; i++ {
		columnInfo := defs.DBColumn{}

		columnDefText := c.Parameter(i)
		if columnDefText == "" {
			break
		}

		t := tokenizer.New(columnDefText, false)
		column := t.Next()

		if !t.IsNext(tokenizer.ColonToken) {
			return errors.ErrInvalidColumnDefinition.Context(columnDefText)
		}

		columnName := column.Spelling()

		// If we've already defined this one, complain
		if _, ok := defined[columnName]; ok {
			return errors.ErrDuplicateColumnName.Context(columnName)
		}

		columnType := t.Next()
		if !columnType.IsIdentifier() {
			return errors.ErrInvalidType.Context(columnType)
		}

		columnTypeName := columnType.Spelling()
		found := false

		for _, typeName := range defs.TableColumnTypeNames {
			if strings.EqualFold(columnTypeName, typeName) {
				found = true

				break
			}
		}

		if !found {
			return errors.ErrInvalidType.Context(columnTypeName)
		}

		for t.IsNext(tokenizer.CommaToken) {
			flag := t.NextText()

			switch strings.ToLower(flag) {
			case "unique":
				columnInfo.Unique = defs.BoolValue{Specified: true, Value: true}

			case "nonunique":
				columnInfo.Unique = defs.BoolValue{Specified: true, Value: false}

			case "nullable", "null":
				columnInfo.Nullable = defs.BoolValue{Specified: true, Value: true}

			case "nonnullable", "nonnull":
				columnInfo.Nullable = defs.BoolValue{Specified: true, Value: false}

			default:
				return errors.ErrInvalidKeyword.Context(flag)
			}
		}

		columnInfo.Name = columnName
		columnInfo.Type = columnTypeName
		defined[columnName] = true
		fields[columnName] = columnInfo
	}

	// Convert the map to an array of fields
	keys := make([]string, 0)
	for k := range fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	payload = make([]defs.DBColumn, len(keys))
	for i, k := range keys {
		payload[i] = fields[k]
	}

	urlString := rest.URLBuilder(defs.TablesNamePath, table).String()
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
	}

	if dsn, found := c.String("dsn"); found {
		urlString = rest.URLBuilder(defs.DSNTablesNamePath, dsn, table).String()
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]
		urlString = rest.URLBuilder(defs.DSNTablesNamePath, schema, table).String()
	}

	// Send the array to the server
	err := rest.Exchange(urlString, http.MethodPut, payload, &resp, defs.TableAgent)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)

			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}
		} else {
			ui.Say("msg.table.created", map[string]interface{}{
				"name":  table,
				"count": len(payload),
			})
		}
	}

	return err
}

func TableUpdate(c *cli.Context) error {
	resp := defs.DBRowCount{}
	table := c.Parameter(0)

	payload := map[string]interface{}{}

	for i := 1; i < 999; i++ {
		p := c.Parameter(i)
		if p == "" {
			break
		}

		t := tokenizer.New(p, false)
		column := t.NextText()

		if !t.IsNext(tokenizer.AssignToken) {
			return errors.ErrMissingAssignment
		}

		value := t.Remainder()

		if strings.EqualFold(strings.TrimSpace(value), defs.True) {
			payload[column] = true
		} else if strings.EqualFold(strings.TrimSpace(value), defs.False) {
			payload[column] = false
		} else if i, err := strconv.Atoi(value); err == nil {
			payload[column] = i
		} else {
			payload[column] = value
		}
	}

	url := rest.URLBuilder(defs.TablesRowsPath, table)
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	}

	if dsn, found := c.String("dsn"); found {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]

		url = rest.URLBuilder(defs.DSNTablesRowsPath, schema, table)
	}

	if filter, ok := c.StringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			url.Parameter(defs.FilterParameterName, f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)

			return errors.Message(msg)
		}
	}

	err := rest.Exchange(url.String(), http.MethodPatch, payload, &resp, defs.TableAgent, defs.RowCountMediaType)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)

			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}
		} else {
			ui.Say("msg.table.update.count", map[string]interface{}{
				"name":  table,
				"count": len(payload),
			})
		}
	}

	return err
}

func TableDelete(c *cli.Context) error {
	resp := defs.DBRowCount{}
	table := c.Parameter(0)

	url := rest.URLBuilder(defs.TablesRowsPath, table)
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	}

	if dsn, found := c.String("dsn"); found {
		url = rest.URLBuilder(defs.DSNTablesRowsPath, dsn, table)
	} else if settings.GetBool(defs.TableAutoparseDSN) && strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema := parts[0]
		table = parts[1]

		url = rest.URLBuilder(defs.DSNTablesRowsPath, schema, table)
	}

	if filter, ok := c.StringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			url.Parameter(defs.FilterParameterName, f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)

			return errors.Message(msg)
		}
	}

	err := rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.TableAgent, defs.RowCountMediaType)
	if err == nil {
		if resp.Status > 200 {
			err = errors.Message(resp.Message)

			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}
		} else {
			if ui.OutputFormat == ui.TextFormat {
				if resp.Count == 0 {
					ui.Say("msg.table.deleted.no.rows")

					return nil
				}

				ui.Say("msg.table.deleted.rows", map[string]interface{}{"count": resp.Count})
			} else {
				_ = commandOutput(resp)
			}
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func makeFilter(filters []string) string {
	terms := make([]string, 0)

	for _, filter := range filters {
		var term strings.Builder

		t := tokenizer.New(filter, true)
		term1 := t.NextText()

		if t.AtEnd() {
			terms = append(terms, term1)

			continue
		}

		op := t.NextText()

		if util.InList(term1, "!", "not", "NOT") {
			term.WriteString("NOT(")
			term.WriteString(op)
			term.WriteString(")")
			terms = append(terms, term.String())

			continue
		}

		term2 := t.NextText()

		if term1 == "" || term2 == "" {
			return filterParseError + i18n.E("filter.term.missing")
		}

		// Handle the case where term2 is a signed number, so we must also
		// grab the term following the sign.
		if util.InList(term2, "+", "-") {
			term2 = term2 + t.NextText()
		}

		// Based on the operator, convert it to a standard form
		switch strings.ToUpper(op) {
		case "=", "IS", "EQ", "EQUAL_TO", "EQUALS":
			op = "EQ"

		// Not equals is a special case of compound operation
		case "!=", "NE", "NOT", "NOT_EQUAL", "NOT_EQUAL_TO":
			term.WriteString("NOT")
			term.WriteRune('(')
			term.WriteString("EQ")
			term.WriteRune('(')
			term.WriteString(term1)
			term.WriteRune(',')
			term.WriteString(term2)
			term.WriteRune(')')
			term.WriteRune(')')

		case ">", "GT", "GREATER_THAN":
			op = "GT"

		case ">=", "GE", "GREATER_THAN_OR_EQUAL_TO", "GREATER_THAN_EQUAL_TO":
			op = "GE"

		case "<", "LT", "LESS_THAN":
			op = "LT"

		case "<=", "LE", "LESS_THAN_OR_EQUAL_TO", "LESS_THAN_EQUAL_TO":
			op = "LE"

		default:
			return filterParseError + i18n.E("filter.term.invalid",
				map[string]interface{}{"term": op})
		}

		// Assuming nothing has been written to the buffer yet (such as
		// for the not-equals case) then write the simple diadic terms.
		if term.Len() == 0 {
			term.WriteString(op)
			term.WriteRune('(')
			term.WriteString(term1)
			term.WriteRune(',')
			term.WriteString(term2)
			term.WriteRune(')')
		}

		terms = append(terms, term.String())
	}

	termCount := len(terms)
	if termCount == 1 {
		return terms[0]
	}

	var b strings.Builder

	for i := 0; i < termCount-1; i++ {
		b.WriteString("AND(")
		b.WriteString(terms[i])
		b.WriteRune(',')
	}

	b.WriteString(terms[termCount-1])

	for i := 0; i < termCount-1; i++ {
		b.WriteRune(')')
	}

	return b.String()
}

// TableSQL executes arbitrary SQL against the server.
func TableSQL(c *cli.Context) error {
	var sql string

	showRowNumbers := c.Boolean("row-numbers")

	for i := 0; i < 999; i++ {
		sqlItem := c.Parameter(i)
		if sqlItem == "" {
			break
		}

		sql = sql + " " + sqlItem
	}

	if c.WasFound("sql-file") {
		fn, _ := c.String("sql-file")

		b, err := os.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		if len(sql) > 0 {
			sql = sql + " "
		}

		sql = sql + string(b)
	}

	if len(strings.TrimSpace(sql)) == 0 {
		ui.Say("msg.enter.blank.line")

		for {
			line := io.ReadConsoleText("sql> ")
			if len(strings.TrimSpace(line)) == 0 {
				break
			}

			sql = sql + " " + line
		}
	}

	sqlPayload := []string{strings.TrimSpace(sql)}

	path := rest.URLBuilder(defs.TablesSQLPath)
	if dsn := settings.Get(defs.DefaultDataSourceSetting); dsn != "" {
		path = rest.URLBuilder(defs.DSNSTablesSQLPath, dsn)
	}

	if dsn, found := c.String("dsn"); found {
		path = rest.URLBuilder(defs.DSNSTablesSQLPath, dsn)
	}

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(sql)), "select ") {
		rows := defs.DBRowSet{}

		err := rest.Exchange(path.String(), http.MethodPut, sqlPayload, &rows, defs.TableAgent, defs.RowSetMediaType)
		if err != nil {
			return err
		}

		if rows.Status > 200 {
			return errors.Message(rows.Message)
		} else {
			_ = printRowSet(rows, true, showRowNumbers)
		}
	} else {
		resp := defs.DBRowCount{}

		err := rest.Exchange(path.String(), http.MethodPut, sqlPayload, &resp, defs.TableAgent, defs.RowCountMediaType)
		if err != nil {
			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}

			return err
		}

		if resp.Status > 200 {
			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			}

			return errors.Message(resp.Message)
		}

		if resp.Count == 0 {
			ui.Say("msg.table.sql.no.rows")
		} else if resp.Count == 1 {
			ui.Say("msg.table.sql.one.row")
		} else {
			ui.Say("msg.table.sql.rows", map[string]interface{}{"count": resp.Count})
		}
	}

	return nil
}

func TablePermissions(c *cli.Context) error {
	permissions := defs.AllPermissionResponse{}
	url := rest.URLBuilder(defs.TablesPermissionsPath)

	user, found := c.String("user")
	if found {
		url.Parameter(defs.UserParameterName, user)
	}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &permissions, defs.TableAgent)
	if err == nil {
		if permissions.Status > 200 {
			err = errors.Message(permissions.Message)

			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(permissions)
			}
		} else {
			if ui.OutputFormat == ui.TextFormat {
				t, _ := tables.New([]string{
					i18n.L("User"),
					i18n.L("Schema"),
					i18n.L("Table"),
					i18n.L("Permissions"),
				})

				for _, permission := range permissions.Permissions {
					_ = t.AddRowItems(permission.User,
						permission.Schema,
						permission.Table,
						strings.TrimPrefix(strings.Join(permission.Permissions, ","), ","),
					)
				}

				t.Print(ui.TextFormat)
			} else {
				_ = commandOutput(permissions)
			}
		}
	}

	return err
}

func TableGrant(c *cli.Context) error {
	permissions, _ := c.StringList("permission")
	table := c.Parameter(0)
	result := defs.PermissionObject{}

	url := rest.URLBuilder(defs.TablesNamePermissionsPath, table)
	if user, found := c.String("user"); found {
		url.Parameter(defs.UserParameterName, user)
	}

	err := rest.Exchange(url.String(), http.MethodPut, permissions, &result, defs.TableAgent)
	if err == nil {
		printPermissionObject(result)
	}

	if ui.OutputFormat != ui.TextFormat {
		_ = commandOutput(result)
	}

	return err
}

func TableShowPermission(c *cli.Context) error {
	table := c.Parameter(0)
	result := defs.PermissionObject{}
	url := rest.URLBuilder(defs.TablesNamePermissionsPath, table)

	err := rest.Exchange(url.String(), http.MethodGet, nil, &result, defs.TableAgent)
	if err == nil {
		printPermissionObject(result)
	} else {
		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(result)
		}
	}

	return err
}

func printPermissionObject(result defs.PermissionObject) {
	if ui.OutputFormat == ui.TextFormat {
		if len(result.Permissions) < 1 {
			if len(result.Permissions) == 0 {
				result.Permissions = []string{"none"}
			}
		}

		ui.Say("msg.table.user.permissions", map[string]interface{}{
			"verb":   "",
			"user":   result.User,
			"schema": result.Schema,
			"table":  result.Table,
			"perms":  strings.TrimPrefix(strings.Join(result.Permissions, ","), ","),
		})
	} else {
		_ = commandOutput(result)
	}
}

func toInterfaces(items []string) []interface{} {
	result := make([]interface{}, len(items))
	for i, item := range items {
		result[i] = item
	}

	return result
}
