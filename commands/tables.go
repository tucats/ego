package commands

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	filterParseError = "==error== "
)

func TableList(c *cli.Context) *errors.EgoError {
	resp := defs.TableInfo{}

	rowCounts := true
	if c.WasFound("no-row-counts") {
		rowCounts = false
	}

	url := runtime.URLBuilder(defs.TablesPath)

	if limit, found := c.Integer("limit"); found {
		url.Parameter(defs.LimitParameterName, limit)
	}

	if start, found := c.Integer("start"); found {
		url.Parameter(defs.StartParameterName, start)
	}

	if !rowCounts {
		url.Parameter(defs.RowCountParameterName, false)
	}

	err := runtime.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.TablesMediaType)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			if rowCounts {
				t, _ := tables.New([]string{"Schema", "Name", "Columns", "Rows"})
				_ = t.SetOrderBy("Name")
				_ = t.SetAlignment(2, tables.AlignmentRight)
				_ = t.SetAlignment(3, tables.AlignmentRight)

				for _, row := range resp.Tables {
					_ = t.AddRowItems(row.Schema, row.Name, row.Columns, row.Rows)
				}

				t.Print(ui.OutputFormat)
			} else {
				t, _ := tables.New([]string{"Schema", "Name", "Columns"})
				_ = t.SetOrderBy("Name")
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

	return errors.New(err)
}

func TableShow(c *cli.Context) *errors.EgoError {
	resp := defs.TableColumnsInfo{}
	table := c.GetParameter(0)

	urlString := runtime.URLBuilder(defs.TablesNamePath, table).String()

	err := runtime.Exchange(urlString, http.MethodGet, nil, &resp, defs.TableAgent, defs.TableMetadataMediaType)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			t, _ := tables.New([]string{"Name", "Type", "Size", "Nullable", "Unique"})
			_ = t.SetOrderBy("Name")
			_ = t.SetAlignment(2, tables.AlignmentRight)

			for _, row := range resp.Columns {
				_ = t.AddRowItems(row.Name, row.Type, row.Size, row.Nullable, row.Unique)
			}

			t.Print(ui.OutputFormat)
		} else {
			_ = commandOutput(resp)
		}
	}

	return errors.New(err)
}

func TableDrop(c *cli.Context) *errors.EgoError {
	var count int

	var err error

	var table string

	resp := defs.TableColumnsInfo{}

	for i := 0; i < 999; i++ {
		table = c.GetParameter(i)
		if table == "" {
			break
		}

		urlString := runtime.URLBuilder(defs.TablesNamePath, table).String()

		err = runtime.Exchange(urlString, http.MethodDelete, nil, &resp, defs.TableAgent)
		if errors.Nil(err) {
			count++

			ui.Say("Table %s deleted", table)
		} else {
			break
		}
	}

	if err == nil && count > 1 {
		ui.Say("Deleted %d tables", count)
	} else if !errors.Nil(err) {
		return errors.New(err)
	}

	return nil
}

func TableContents(c *cli.Context) *errors.EgoError {
	resp := defs.DBRowSet{}
	table := c.GetParameter(0)
	url := runtime.URLBuilder(defs.TablesRowsPath, table)

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

			return errors.NewMessage(msg)
		}
	}

	err := runtime.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.RowSetMediaType)
	if errors.Nil(err) {
		err = printRowSet(resp, c.Boolean("row-ids"), c.Boolean("row-numbers"))
	}

	return errors.New(err)
}

func printRowSet(resp defs.DBRowSet, showRowID bool, showRowNumber bool) *errors.EgoError {
	if ui.OutputFormat == ui.TextFormat {
		if len(resp.Rows) == 0 {
			ui.Say("No rows in query")

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

func TableInsert(c *cli.Context) *errors.EgoError {
	resp := defs.DBRowCount{}
	table := c.GetParameter(0)
	payload := map[string]interface{}{}

	// If there is a JSON file to initialize the payload with, do it now.
	if c.WasFound("file") {
		fn, _ := c.String("file")

		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		err = json.Unmarshal(b, &payload)
		if err != nil {
			return errors.New(err)
		}
	}

	for i := 1; i < 999; i++ {
		p := c.GetParameter(i)
		if p == "" {
			break
		}

		t := tokenizer.New(p)
		column := t.Next()

		if !t.IsNext("=") {
			return errors.New(errors.ErrMissingAssignment)
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

	if len(payload) == 0 {
		ui.Say("Nothing to insert")

		return nil
	}

	urlString := runtime.URLBuilder(defs.TablesRowsPath, table).String()

	err := runtime.Exchange(urlString, "PUT", payload, &resp, defs.TableAgent)
	if errors.Nil(err) {
		ui.Say("Added %d row(s) to table %s", resp.Count, table)

		return nil
	}

	return errors.New(err)
}

func TableCreate(c *cli.Context) *errors.EgoError {
	table := c.GetParameter(0)
	fields := map[string]defs.DBColumn{}
	payload := make([]defs.DBColumn, 0)
	resp := defs.DBRowCount{}

	defined := map[string]bool{}

	// If the user specified a file with a JSON payload, use that to seed the
	// field definitions map. We will also look for command line parameters to
	// extend or modify the field list.
	if c.WasFound("file") {
		fn, _ := c.String("file")

		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		err = json.Unmarshal(b, &payload)
		if err != nil {
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

		columnDefText := c.GetParameter(i)
		if columnDefText == "" {
			break
		}

		t := tokenizer.New(columnDefText)
		column := t.Next()

		if !t.IsNext(":") {
			return errors.New(errors.ErrInvalidColumnDefinition).Context(columnDefText)
		}

		// If we've already defined this one, complain
		if _, ok := defined[column]; ok {
			return errors.New(errors.ErrDuplicateColumnName).Context(column)
		}

		columnType := t.Next()
		found := false

		for _, typeName := range defs.TableColumnTypeNames {
			if strings.EqualFold(columnType, typeName) {
				found = true

				break
			}
		}

		if !found {
			return errors.New(errors.ErrInvalidType).Context(columnType)
		}

		for t.IsNext(",") {
			flag := t.Next()

			switch strings.ToLower(flag) {
			case "unique":
				columnInfo.Unique = true
			case "nullable":
				columnInfo.Nullable = true

			default:
				return errors.New(errors.ErrInvalidKeyword).Context(flag)
			}
		}

		columnInfo.Name = column
		columnInfo.Type = columnType
		defined[column] = true
		fields[column] = columnInfo
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

	urlString := runtime.URLBuilder(defs.TablesNamePath, table).String()

	// Send the array to the server
	err := runtime.Exchange(
		urlString,
		"PUT",
		payload,
		&resp,
		defs.TableAgent)

	if errors.Nil(err) {
		ui.Say("Created table %s with %d columns", table, len(payload))
	}

	return err
}

func TableUpdate(c *cli.Context) *errors.EgoError {
	resp := defs.DBRowCount{}
	table := c.GetParameter(0)

	payload := map[string]interface{}{}

	for i := 1; i < 999; i++ {
		p := c.GetParameter(i)
		if p == "" {
			break
		}

		t := tokenizer.New(p)
		column := t.Next()

		if !t.IsNext("=") {
			return errors.New(errors.ErrMissingAssignment)
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

	url := runtime.URLBuilder(defs.TablesRowsPath, table)

	if filter, ok := c.StringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			url.Parameter(defs.FilterParameterName, f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)

			return errors.NewMessage(msg)
		}
	}

	err := runtime.Exchange(
		url.String(),
		http.MethodPatch,
		payload,
		&resp,
		defs.TableAgent,
		defs.RowCountMediaType)

	if errors.Nil(err) {
		ui.Say("Updated %d rows in table %s", resp.Count, table)
	}

	return err
}

func TableDelete(c *cli.Context) *errors.EgoError {
	resp := defs.DBRowCount{}
	table := c.GetParameter(0)

	url := runtime.URLBuilder(defs.TablesRowsPath, table)

	if filter, ok := c.StringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			url.Parameter(defs.FilterParameterName, f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)

			return errors.NewMessage(msg)
		}
	}

	err := runtime.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.TableAgent, defs.RowCountMediaType)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			if resp.Count == 0 {
				ui.Say("No rows deleted")

				return nil
			}

			ui.Say("%d rows deleted", resp.Count)
		} else {
			_ = commandOutput(resp)
		}
	}

	return errors.New(err)
}

func makeFilter(filters []string) string {
	terms := make([]string, 0)

	for _, filter := range filters {
		var term strings.Builder

		t := tokenizer.New(filter)
		term1 := t.Next()

		if t.AtEnd() {
			terms = append(terms, term1)

			continue
		}

		op := t.Next()

		if util.InList(term1, "!", "not", "NOT") {
			term.WriteString("NOT(")
			term.WriteString(op)
			term.WriteString(")")
			terms = append(terms, term.String())

			continue
		}

		term2 := t.Next()

		if term1 == "" || term2 == "" {
			return filterParseError + "Missing filter term"
		}

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
			return filterParseError + "Unrecognized operator: " + op
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
func TableSQL(c *cli.Context) *errors.EgoError {
	var sql string

	showRowNumbers := c.Boolean("row-numbers")

	for i := 0; i < 999; i++ {
		sqlItem := c.GetParameter(i)
		if sqlItem == "" {
			break
		}

		sql = sql + " " + sqlItem
	}

	if c.WasFound("sql-file") {
		fn, _ := c.String("sql-file")

		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		if len(sql) > 0 {
			sql = sql + " "
		}

		sql = sql + string(b)
	}

	if len(strings.TrimSpace(sql)) == 0 {
		ui.Say("Enter a blank line to terminate SQL command input")

		for {
			line := runtime.ReadConsoleText("sql> ")
			if len(strings.TrimSpace(line)) == 0 {
				break
			}

			sql = sql + " " + line
		}
	}

	sqlPayload := []string{strings.TrimSpace(sql)}

	if strings.Contains(strings.ToLower(sql), "select ") {
		rows := defs.DBRowSet{}

		err := runtime.Exchange(defs.TablesSQLPath, "PUT", sqlPayload, &rows, defs.TableAgent, defs.RowSetMediaType)
		if !errors.Nil(err) {
			return err
		}

		_ = printRowSet(rows, true, showRowNumbers)
	} else {
		resp := defs.DBRowCount{}

		err := runtime.Exchange(defs.TablesSQLPath, "PUT", sqlPayload, &resp, defs.TableAgent, defs.RowCountMediaType)
		if !errors.Nil(err) {
			return err
		}

		if resp.Count == 0 {
			ui.Say("No rows modified")
		} else if resp.Count == 1 {
			ui.Say("1 row modified")
		} else {
			ui.Say("%d rows modified", resp.Count)
		}
	}

	return nil
}

func TablePermissions(c *cli.Context) *errors.EgoError {
	permissions := defs.AllPermissionResponse{}
	url := runtime.URLBuilder(defs.TablesPermissionsPath)

	user, found := c.String("user")
	if found {
		url.Parameter(defs.UserParameterName, user)
	}

	err := runtime.Exchange(url.String(), http.MethodGet, nil, &permissions, defs.TableAgent)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			t, _ := tables.New([]string{"User", "Schema", "Table", "Permissions"})
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

	return err
}

func TableGrant(c *cli.Context) *errors.EgoError {
	permissions, _ := c.StringList("permission")
	table := c.GetParameter(0)
	result := defs.PermissionObject{}

	url := runtime.URLBuilder(defs.TablesNamePermissionsPath, table)
	if user, found := c.String("user"); found {
		url.Parameter(defs.UserParameterName, user)
	}

	err := runtime.Exchange(url.String(), "PUT", permissions, &result, defs.TableAgent)
	if errors.Nil(err) {
		printPermissionObject(result)
	}

	return err
}

func TableShowPermission(c *cli.Context) *errors.EgoError {
	table := c.GetParameter(0)
	result := defs.PermissionObject{}
	url := runtime.URLBuilder(defs.TablesNamePermissionsPath, table)

	err := runtime.Exchange(url.String(), http.MethodGet, nil, &result, defs.TableAgent)
	if errors.Nil(err) {
		printPermissionObject(result)
	}

	return err
}

func printPermissionObject(result defs.PermissionObject) {
	if ui.OutputFormat == ui.TextFormat {
		plural := "s"
		verb := "are"

		if len(result.Permissions) < 1 {
			plural = ""
			verb = "is"

			if len(result.Permissions) == 0 {
				result.Permissions = []string{"none"}
			}
		}

		ui.Say("User %s permission%s for %s.%s %s %s",
			result.User,
			plural,
			result.Schema,
			result.Table,
			verb,
			strings.TrimPrefix(strings.Join(result.Permissions, ","), ","),
		)
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
