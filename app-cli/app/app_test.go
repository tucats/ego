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

	if v, found := symbols.Get(defs.BuildTimeVariable); found {
		t1 := data.String(v)
		t2 := "01 Jan 22 12:34 +0000"

		if t1 != t2 {
			t.Errorf("BuildTime variable is incorrect.\nExpected: %s\nGot     : %v", t2, t1)
		}
	}

	// Test case 2: Invalid time format
	invalidBuildTime := "2022ZOG0101"
	app.SetBuildTime(invalidBuildTime)

	if app.BuildTime != invalidBuildTime {
		t.Errorf("BuildTime is incorrect. Expected: %s, Got: %s", invalidBuildTime, app.BuildTime)
	}

	v, found := symbols.Get(defs.BuildTimeVariable)
	if !found || data.String(v) != invalidBuildTime {
		t.Errorf("BuildTime variable is incorrect. Expected: %s, Got: %v", invalidBuildTime, v)
	}
}
