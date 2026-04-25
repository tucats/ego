package tables_test

import (
	"strings"
	"testing"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
)

func TestTable_SortRows(t *testing.T) {
	t.Run("sort test for floating values", func(t *testing.T) {
		tb, _ := tables.New([]string{"floats"})
		tb.AddRowItems(3.501)
		tb.AddRowItems(3)
		tb.AddRowItems(33)
		tb.AddRowItems(3.2)

		_ = tb.SortRows(0, true)
		tb.ShowHeadings(false)

		text, _ := tb.String(ui.TextFormat)
		lines := strings.Split(text, "\n")

		expected := []string{"3", "3.2", "3.501", "33", ""}

		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line != expected[i] {
				t.Errorf("Unexpected result\ngot \"%s\", want \"%s\"", line, expected[i])
			}
		}
	})

	t.Run("sort test for integer values", func(t *testing.T) {
		tb, _ := tables.New([]string{"integers"})
		tb.AddRowItems(101)
		tb.AddRowItems(21)
		tb.AddRowItems(220)
		tb.AddRowItems(10)

		_ = tb.SortRows(0, true)
		tb.ShowHeadings(false)

		text, _ := tb.String(ui.TextFormat)
		lines := strings.Split(text, "\n")

		expected := []string{"10", "21", "101", "220", ""}

		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line != expected[i] {
				t.Errorf("Unexpected result\ngot \"%s\", want \"%s\"", line, expected[i])
			}
		}
	})

	t.Run("sort test for string values", func(t *testing.T) {
		tb, _ := tables.New([]string{"strings"})
		tb.AddRowItems("red")
		tb.AddRowItems("blue")
		tb.AddRowItems("yellow")
		tb.AddRowItems("green")

		_ = tb.SortRows(0, true)
		tb.ShowHeadings(false)

		text, _ := tb.String(ui.TextFormat)
		lines := strings.Split(text, "\n")

		expected := []string{"blue", "green", "red", "yellow", ""}

		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line != expected[i] {
				t.Errorf("Unexpected result\ngot \"%s\", want \"%s\"", line, expected[i])
			}
		}
	})
}
