package profiling

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/internal/cli/tables"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/i18n"
)

// PrintProfileReport prints a formatted report of the performance data collected during profiling.
func PrintProfileReport() error {
	if len(PerformanceData) == 0 {
		return nil
	}

	performanceMux.Lock()
	defer performanceMux.Unlock()

	// Build a sort key for each entry. A profile key is "module:line", so the
	// sort key right-aligns the line number ("%4s") to make the report sort
	// numerically within a module rather than lexically ("10" before "9").
	//
	// INDEX-16: this used parts[0] and parts[1] from splitting on ":", and the
	// loop below used parts[1] from splitting on "#". Neither index is
	// trustworthy: the module component is a source file name supplied by the
	// user, so it can contain both separators. A module name containing ":"
	// silently mis-parsed, and one containing "#" made the recovered lookup key
	// wrong, so PerformanceData returned a nil counter and count.Load() panicked
	// on a nil pointer. Splitting from the right on the last ":" identifies the
	// line number correctly, and the original key is now carried alongside the
	// sort key instead of being re-parsed out of it.
	type profileEntry struct {
		sortKey string
		name    string
	}

	entries := make([]profileEntry, 0, len(PerformanceData))

	for name := range PerformanceData {
		module, line := name, ""
		if at := strings.LastIndex(name, ":"); at >= 0 {
			module, line = name[:at], name[at+1:]
		}

		entries = append(entries, profileEntry{
			sortKey: fmt.Sprintf("%s:%4s", module, line),
			name:    name,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].sortKey < entries[j].sortKey
	})

	t, err := tables.New([]string{i18n.L("Location"), i18n.L("Count")})
	if err != nil {
		return err
	}

	err = t.SetAlignment(1, tables.AlignmentRight)
	if err != nil {
		return err
	}

	// No pagination for this report.
	t.SetPagination(0, 0)

	for _, entry := range entries {
		// The name came from ranging over PerformanceData, so the counter is
		// always present; the check keeps a nil map value from reaching Load().
		count := PerformanceData[entry.name]
		if count == nil {
			continue
		}

		err = t.AddRowItems(entry.name, count.Load())
		if err != nil {
			return err
		}
	}

	err = t.Print(ui.TextFormat)
	if err != nil {
		return err
	}

	// Empty out the performance data for the next report.
	PerformanceData = make(map[string]*atomic.Uint32)

	return nil
}
