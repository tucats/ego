package app

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func TestSetBuildTime(t *testing.T) {
	app := &App{}

	symbols := symbols.NewSymbolTable("test")

	// Test case 1: Valid time format
	buildTime := "20220101123456"
	expectedTime := "2022-01-01 12:34:56 +0000 UTC"

	app.SetBuildTime(buildTime)

	if app.BuildTime != expectedTime {
		t.Errorf("BuildTime is incorrect. Expected: %s, Got: %s", expectedTime, app.BuildTime)
	}

	if v, found := symbols.Get(defs.BuildTimeVariable); found && data.String(v) != expectedTime {
		t.Errorf("BuildTime variable is incorrect. Expected: %s, Got: %v", expectedTime, v)
	}

	// Test case 2: Invalid time format
	invalidBuildTime := "20220101"
	app.SetBuildTime(invalidBuildTime)

	if app.BuildTime != invalidBuildTime {
		t.Errorf("BuildTime is incorrect. Expected: %s, Got: %s", invalidBuildTime, app.BuildTime)
	}

	v, found := symbols.Get(defs.BuildTimeVariable)
	if !found || data.String(v) != invalidBuildTime {
		t.Errorf("BuildTime variable is incorrect. Expected: %s, Got: %v", invalidBuildTime, v)
	}
}
