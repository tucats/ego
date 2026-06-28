package tables

import (
	"strings"
	"testing"
)

// splitPagelet splits a pagelet string on newlines, drops the trailing empty
// element that results from each line ending with '\n', and trims trailing
// spaces from every line so that assertions are not sensitive to right-side
// padding.
func splitPagelet(s string) []string {
	parts := strings.Split(s, "\n")
	// Drop the final empty string produced by the last '\n'.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimRight(p, " ")
	}

	return out
}

// makeTable is a test helper that creates a Table with explicit terminal
// dimensions so the tests are independent of the real terminal size.
func makeTable(t *testing.T, headings []string, termHeight, termWidth int) *Table {
	t.Helper()

	tb, err := New(headings)
	if err != nil {
		t.Fatalf("New(%v): unexpected error: %v", headings, err)
	}

	tb.SetPagination(termHeight, termWidth)

	return tb
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_FitsOnOnePage
//
// The terminal is large enough to show all columns and all rows in a single
// pagelet. Verifies that exactly one pagelet is returned and that its content
// matches the expected header, underline, and data rows exactly.
//
// The default spacing is 4 spaces. renderRow and renderUnderline both use
// t.spacing as the inter-column separator, consistent with the width budget
// computed by determineFittingColumns.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_FitsOnOnePage(t *testing.T) {
	tb := makeTable(t, []string{"Name", "Age", "City"}, 20, 200)
	_ = tb.AddRowItems("Alice", "30", "Boston")
	_ = tb.AddRowItems("Bob", "25", "Denver")
	_ = tb.AddRowItems("Carol", "42", "Austin")

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 1 {
		t.Fatalf("expected 1 pagelet, got %d", len(pagelets))
	}

	lines := splitPagelet(pagelets[0])

	// header + underline + 3 data rows = 5 lines.
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %q", len(lines), lines)
	}

	// Column widths: Name=5, Age=3, City=6. Inter-column gap = 4 spaces (default spacing).
	wantLines := []string{
		"Name     Age    City",   // "Name " + 4sp + "Age" + 4sp + "City  " (trailing spaces trimmed)
		"=====    ===    ======", // underline uses widths[colIndex]
		"Alice    30     Boston",
		"Bob      25     Denver",
		"Carol    42     Austin",
	}

	for i, want := range wantLines {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_TooManyColumns
//
// The terminal is too narrow to display all four columns at once. The columns
// should be split into two width-slices, each rendered as its own pagelet.
//
// Column widths are all 2. spacing is "    " (4 chars).
// availableWidth = termWidth(12) - len(indent)(0) - len(spacing)(4) = 8.
//
// Slice 1: AA(2) fits; AA + 4sp + BB(2) = 8 ≤ 8 fits; +CC would be 14 > 8.
// Slice 2: CC(2), CC + 4sp + DD(2) = 8 ≤ 8 fits.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_TooManyColumns(t *testing.T) {
	tb := makeTable(t, []string{"AA", "BB", "CC", "DD"}, 20, 12)
	_ = tb.AddRowItems("11", "22", "33", "44")

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 2 {
		t.Fatalf("expected 2 pagelets (one per column-slice), got %d", len(pagelets))
	}

	// ── Pagelet 1: columns AA and BB ─────────────────────────────────────────

	p1 := splitPagelet(pagelets[0])

	if len(p1) != 3 {
		t.Fatalf("pagelet 1: expected 3 lines (header+underline+row), got %d: %q", len(p1), p1)
	}

	// 4-space separator matches t.spacing default.
	wantP1 := []string{"AA    BB", "==    ==", "11    22"}
	for i, want := range wantP1 {
		if p1[i] != want {
			t.Errorf("pagelet 1 line %d: got %q, want %q", i, p1[i], want)
		}
	}

	// ── Pagelet 2: columns CC and DD ─────────────────────────────────────────

	p2 := splitPagelet(pagelets[1])

	if len(p2) != 3 {
		t.Fatalf("pagelet 2: expected 3 lines (header+underline+row), got %d: %q", len(p2), p2)
	}

	wantP2 := []string{"CC    DD", "==    ==", "33    44"}
	for i, want := range wantP2 {
		if p2[i] != want {
			t.Errorf("pagelet 2 line %d: got %q, want %q", i, p2[i], want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_TooManyRows
//
// The table has more rows than fit in the terminal height. Verifies correct
// distribution across height-pages and that no pagelet exceeds terminalHeight.
//
// terminalHeight=6. showHeadings and showUnderlines are both true, so
// startingRowOnPage()=2. The page-break condition is:
//   currentPageHeight + rowHeight > terminalHeight-1  (i.e., > 5)
//
// After filling 3 rows: currentPageHeight = 2+1+1+1 = 5; adding a 4th would
// give 6 > 5, triggering a break. Distribution: [3, 3, 1] rows per page.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_TooManyRows(t *testing.T) {
	tb := makeTable(t, []string{"N", "V"}, 6, 200)

	for _, row := range [][]string{
		{"a", "1"}, {"b", "2"}, {"c", "3"},
		{"d", "4"}, {"e", "5"}, {"f", "6"},
		{"g", "7"},
	} {
		_ = tb.AddRow(row)
	}

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 3 {
		t.Fatalf("expected 3 pagelets, got %d", len(pagelets))
	}

	// Every pagelet must carry its own header + underline (every page is
	// self-contained) and must not exceed the terminal height.
	for i, p := range pagelets {
		lines := splitPagelet(p)

		if len(lines) > 6 {
			t.Errorf("pagelet %d exceeds terminal height: %d lines (max 6)", i, len(lines))
		}

		if lines[0] != "N    V" {
			t.Errorf("pagelet %d header: got %q, want %q", i, lines[0], "N    V")
		}

		if lines[1] != "=    =" {
			t.Errorf("pagelet %d underline: got %q, want %q", i, lines[1], "=    =")
		}
	}

	// Verify row distribution: [3, 3, 1].
	wantDataRows := []int{3, 3, 1}

	for i, p := range pagelets {
		lines := splitPagelet(p)

		got := len(lines) - 2 // subtract header and underline
		if got != wantDataRows[i] {
			t.Errorf("pagelet %d: expected %d data rows, got %d", i, wantDataRows[i], got)
		}
	}

	// Spot-check first and last data lines of each pagelet.
	cases := []struct {
		pagelet   int
		firstLine int
		firstWant string
		lastLine  int
		lastWant  string
	}{
		{0, 2, "a    1", 4, "c    3"},
		{1, 2, "d    4", 4, "f    6"},
		{2, 2, "g    7", 2, "g    7"},
	}

	for _, c := range cases {
		lines := splitPagelet(pagelets[c.pagelet])
		if lines[c.firstLine] != c.firstWant {
			t.Errorf("pagelet %d line %d: got %q, want %q", c.pagelet, c.firstLine, lines[c.firstLine], c.firstWant)
		}

		if lines[c.lastLine] != c.lastWant {
			t.Errorf("pagelet %d line %d: got %q, want %q", c.pagelet, c.lastLine, lines[c.lastLine], c.lastWant)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_TooManyColumnsAndRows
//
// The table exceeds both the terminal width and height. The expected layout is:
//   2 column-slices  (AA+BB, then CC+DD — same as TooManyColumns)
//   × 3 height-pages (rows 1-3, 4-6, 7 — same as TooManyRows)
//   = 6 total pagelets
//
// The outer loop in RenderPagelets iterates over width-slices; within each
// slice the inner loop produces height-pages. So pagelets are ordered as:
//   [0] AA+BB rows 1-3  [1] AA+BB rows 4-6  [2] AA+BB row 7
//   [3] CC+DD rows 1-3  [4] CC+DD rows 4-6  [5] CC+DD row 7
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_TooManyColumnsAndRows(t *testing.T) {
	tb := makeTable(t, []string{"AA", "BB", "CC", "DD"}, 6, 12)

	for _, row := range [][]string{
		{"a1", "a2", "a3", "a4"},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2", "c3", "c4"},
		{"d1", "d2", "d3", "d4"},
		{"e1", "e2", "e3", "e4"},
		{"f1", "f2", "f3", "f4"},
		{"g1", "g2", "g3", "g4"},
	} {
		_ = tb.AddRow(row)
	}

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 6 {
		t.Fatalf("expected 6 pagelets (2 column-slices × 3 height-pages), got %d", len(pagelets))
	}

	// No pagelet may exceed the terminal height.
	for i, p := range pagelets {
		lines := splitPagelet(p)
		if len(lines) > 6 {
			t.Errorf("pagelet %d exceeds terminal height: %d lines", i, len(lines))
		}
	}

	// Pagelets 0-2 show columns AA and BB; pagelets 3-5 show CC and DD.
	for i, p := range pagelets {
		lines := splitPagelet(p)
		header := lines[0]

		if i < 3 {
			if !strings.Contains(header, "AA") || !strings.Contains(header, "BB") {
				t.Errorf("pagelet %d header should show AA/BB, got %q", i, header)
			}

			if strings.Contains(header, "CC") || strings.Contains(header, "DD") {
				t.Errorf("pagelet %d header should not show CC/DD, got %q", i, header)
			}
		} else {
			if !strings.Contains(header, "CC") || !strings.Contains(header, "DD") {
				t.Errorf("pagelet %d header should show CC/DD, got %q", i, header)
			}

			if strings.Contains(header, "AA") || strings.Contains(header, "BB") {
				t.Errorf("pagelet %d header should not show AA/BB, got %q", i, header)
			}
		}
	}

	// Verify row distribution: [3,3,1] within each column-slice.
	wantDataRows := []int{3, 3, 1, 3, 3, 1}

	for i, p := range pagelets {
		lines := splitPagelet(p)
		
		got := len(lines) - 2 // subtract header + underline
		if got != wantDataRows[i] {
			t.Errorf("pagelet %d: expected %d data rows, got %d", i, wantDataRows[i], got)
		}
	}

	// Spot-check specific cells to confirm both column-slice and height-page
	// boundaries are drawn at the right places.
	spotChecks := []struct {
		pagelet  int
		line     int
		wantLine string
		desc     string
	}{
		{0, 2, "a1    a2", "first row of AA+BB slice, page 1"},
		{0, 4, "c1    c2", "last row of AA+BB slice, page 1"},
		{1, 2, "d1    d2", "first row of AA+BB slice, page 2"},
		{1, 4, "f1    f2", "last row of AA+BB slice, page 2"},
		{2, 2, "g1    g2", "only row of AA+BB slice, page 3"},
		{3, 2, "a3    a4", "first row of CC+DD slice, page 1"},
		{3, 4, "c3    c4", "last row of CC+DD slice, page 1"},
		{4, 2, "d3    d4", "first row of CC+DD slice, page 2"},
		{5, 2, "g3    g4", "only row of CC+DD slice, page 3"},
	}

	for _, sc := range spotChecks {
		lines := splitPagelet(pagelets[sc.pagelet])
		if lines[sc.line] != sc.wantLine {
			t.Errorf("pagelet %d line %d (%s): got %q, want %q",
				sc.pagelet, sc.line, sc.desc, lines[sc.line], sc.wantLine)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_EmptyTable
//
// A table with no data rows should return a single empty pagelet without
// panicking or returning nil.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_EmptyTable(t *testing.T) {
	tb := makeTable(t, []string{"A", "B"}, 20, 80)

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 1 {
		t.Fatalf("expected 1 (empty) pagelet, got %d", len(pagelets))
	}

	if pagelets[0] != "" {
		t.Errorf("empty table pagelet: got %q, want empty string", pagelets[0])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_DefaultDimensions
//
// When SetPagination is called with 0,0 (simulating a non-terminal environment
// where New() leaves both fields at 0), RenderPagelets must fall back to the
// built-in defaults of 80 wide × 24 high and produce valid output.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_DefaultDimensions(t *testing.T) {
	tb, err := New([]string{"X"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Force both dimensions to 0 to exercise the fallback path inside
	// RenderPagelets that sets them to 80 and 24 respectively.
	tb.SetPagination(0, 0)
	_ = tb.AddRowItems("hello")

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 1 {
		t.Fatalf("expected 1 pagelet with default dimensions, got %d", len(pagelets))
	}

	if !strings.Contains(pagelets[0], "hello") {
		t.Errorf("pagelet missing row data %q: %q", "hello", pagelets[0])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_HeadingPermutations
//
// Verifies that the line count per pagelet and the vertical pagination are
// correct for all four combinations of showHeadings / showUnderlines.
//
// startingRowOnPage() returns:
//   headings=true,  underlines=true  → 2 (header + underline rows reserved)
//   headings=true,  underlines=false → 1 (header row only)
//   headings=false, underlines=false → 0 (no overhead; ignored when headings off)
//   headings=false, underlines=true  → 0 (underlines are suppressed when there
//                                        is no heading to underline)
//
// With terminalHeight=5 and threshold = terminalHeight-1 = 4:
//
//   overhead=2 → 2 data rows per page (2+1+1=4 ≤ 4; +1 more would be 5 > 4)
//   overhead=1 → 3 data rows per page (1+1+1+1=4 ≤ 4; +1 more would be 5 > 4)
//   overhead=0 → 4 data rows per page (0+1+1+1+1=4 ≤ 4; +1 more would be 5 > 4)
//
// All cases use 5 data rows so that the page-break boundary is exercised.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_HeadingPermutations(t *testing.T) {
	dataRows := [][]string{
		{"a", "1"}, {"b", "2"}, {"c", "3"}, {"d", "4"}, {"e", "5"},
	}

	tests := []struct {
		name           string
		showHeadings   bool
		showUnderlines bool
		// overhead is the number of lines consumed by headers per pagelet.
		overhead int
		// rowsPerPage is how many data rows fit on a full page given the overhead.
		rowsPerPage int
		// wantPagelets is the expected number of pagelets for 5 rows.
		wantPagelets int
	}{
		{
			name:           "headings and underlines on (default)",
			showHeadings:   true,
			showUnderlines: true,
			overhead:       2,
			rowsPerPage:    2,
			wantPagelets:   3, // pages of [2, 2, 1]
		},
		{
			name:           "headings on, underlines off",
			showHeadings:   true,
			showUnderlines: false,
			overhead:       1,
			rowsPerPage:    3,
			wantPagelets:   2, // pages of [3, 2]
		},
		{
			name:           "headings off, underlines off",
			showHeadings:   false,
			showUnderlines: false,
			overhead:       0,
			rowsPerPage:    4,
			wantPagelets:   2, // pages of [4, 1]
		},
		{
			name:           "headings off, underlines on (underlines suppressed without headings)",
			showHeadings:   false,
			showUnderlines: true,
			overhead:       0,
			rowsPerPage:    4,
			wantPagelets:   2, // same as headings off — underlines are not emitted without headings
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb := makeTable(t, []string{"N", "V"}, 5, 200)
			tb.ShowHeadings(tc.showHeadings)
			tb.ShowUnderlines(tc.showUnderlines)

			for _, row := range dataRows {
				_ = tb.AddRow(row)
			}

			pagelets := tb.RenderPagelets()

			if len(pagelets) != tc.wantPagelets {
				t.Fatalf("expected %d pagelets, got %d", tc.wantPagelets, len(pagelets))
			}

			// No pagelet may exceed the terminal height.
			for i, p := range pagelets {
				lines := splitPagelet(p)
				if len(lines) > 5 {
					t.Errorf("pagelet %d: %d lines exceeds terminal height of 5", i, len(lines))
				}
			}

			// Every pagelet's data rows should reflect the expected page capacity.
			// The last pagelet may have fewer rows; all others should be full.
			totalRows := 0

			for i, p := range pagelets {
				lines := splitPagelet(p)
				dataLines := len(lines) - tc.overhead
				totalRows += dataLines

				// All but the last pagelet should be full.
				if i < len(pagelets)-1 && dataLines != tc.rowsPerPage {
					t.Errorf("pagelet %d (not last): expected %d data rows, got %d",
						i, tc.rowsPerPage, dataLines)
				}
			}

			if totalRows != len(dataRows) {
				t.Errorf("total data rows across all pagelets: got %d, want %d",
					totalRows, len(dataRows))
			}

			// When headings are enabled, verify every pagelet starts with the header
			// and that the underline row is present or absent as configured.
			if tc.showHeadings {
				for i, p := range pagelets {
					lines := splitPagelet(p)

					if !strings.Contains(lines[0], "N") || !strings.Contains(lines[0], "V") {
						t.Errorf("pagelet %d line 0: expected header containing N and V, got %q",
							i, lines[0])
					}

					if tc.showUnderlines {
						if lines[1] != "=    =" {
							t.Errorf("pagelet %d line 1: expected underline %q, got %q",
								i, "=    =", lines[1])
						}
					} else {
						// Line 1 should be the first data row, not an underline.
						if strings.Contains(lines[1], "=") {
							t.Errorf("pagelet %d line 1: underlines should be off, got %q",
								i, lines[1])
						}
					}
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_StateRestored
//
// RenderPagelets mutates t.rows, t.names, and t.alignment in-place during
// formatting and then restores them via a deferred function. This test verifies
// that the table's observable state is identical before and after the call —
// specifically that calling RenderPagelets twice produces identical output.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_StateRestored(t *testing.T) {
	tb := makeTable(t, []string{"Col1", "Col2"}, 20, 80)
	_ = tb.AddRowItems("foo", "bar")
	_ = tb.AddRowItems("baz", "qux")

	first := tb.RenderPagelets()
	second := tb.RenderPagelets()

	if len(first) != len(second) {
		t.Fatalf("pagelet count differs between calls: %d vs %d", len(first), len(second))
	}

	for i := range first {
		if first[i] != second[i] {
			t.Errorf("pagelet %d differs between first and second call:\nfirst:  %q\nsecond: %q",
				i, first[i], second[i])
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderPagelets_CustomSpacing
//
// Verifies that SetSpacing is respected by both the width-budget calculation
// (determineFittingColumns) and the rendered output (renderRow, renderUnderline).
// Uses spacing=2 so that the column separator in output is "  " (2 spaces).
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPagelets_CustomSpacing(t *testing.T) {
	tb := makeTable(t, []string{"A", "B"}, 20, 200)
	_ = tb.SetSpacing(2) // override default 4-space spacing
	_ = tb.AddRowItems("x", "y")

	pagelets := tb.RenderPagelets()

	if len(pagelets) != 1 {
		t.Fatalf("expected 1 pagelet, got %d", len(pagelets))
	}

	lines := splitPagelet(pagelets[0])

	// With 2-space spacing the inter-column separator is "  ".
	if lines[0] != "A  B" {
		t.Errorf("header with spacing=2: got %q, want %q", lines[0], "A  B")
	}

	if lines[1] != "=  =" {
		t.Errorf("underline with spacing=2: got %q, want %q", lines[1], "=  =")
	}

	if lines[2] != "x  y" {
		t.Errorf("data row with spacing=2: got %q, want %q", lines[2], "x  y")
	}
}
