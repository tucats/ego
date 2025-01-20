package profiling

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
)

// PrintProfileReport prints a formatted report of the performance data collected during profiling.
func PrintProfileReport() error {
	if len(PerformanceData) == 0 {
		return nil
	}

	performanceMux.Lock()
	defer performanceMux.Unlock()

	keys := make([]string, 0, len(PerformanceData))

	for name := range PerformanceData {
		parts := strings.Split(name, ":")
		key := fmt.Sprintf("%s:%4s#%s", parts[0], parts[1], name)
		keys = append(keys, key)
	}

	sort.Strings(keys)

	t, err := tables.New([]string{"Location", "Count"})
	if err != nil {
		return err
	}

	err = t.SetAlignment(1, tables.AlignmentRight)
	if err != nil {
		return err
	}

	// No pagination for this report.
	t.SetPagination(0, 0)

	for _, key := range keys {
		parts := strings.Split(key, "#")
		count := PerformanceData[parts[1]]

		err = t.AddRowItems(parts[1], count.Load())
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
