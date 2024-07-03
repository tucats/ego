package time

import (
	"fmt"
	"testing"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func TestFormat_InvalidLayout(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	now := time.Now()
	this := makeTime(&now, s)
	s.SetAlways(defs.ThisVariable, this)

	args := data.NewList("invalid layout")
	got, err := format(s, args)
	if fmt.Sprintf("%v", got) != "invalid layout" {
		t.Errorf("format() got = %v, want nil", got)
	}

	if err != nil {
		t.Errorf("format() error = %v, want nil", err)
	}

}

func TestFormat_ValidLayout(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	now := time.Now()
	basicLayout := "2006-01-02T15:04:05Z"
	want := now.Format(basicLayout)

	this := makeTime(&now, s)
	s.SetAlways(defs.ThisVariable, this)

	args := data.NewList(basicLayout)
	got, err := format(s, args)
	if err != nil {
		t.Errorf("format() error = %v, want nil", err)
	}

	if got.(string) != want {
		t.Errorf("format() got = %v, want %v", got, want)
	}
}
