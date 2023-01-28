package io

import (
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

func Test_sandboxName(t *testing.T) {
	tests := []struct {
		name    string
		sandbox string
		want    string
	}{
		{
			name:    "/tmp/foo",
			sandbox: "",
			want:    "/tmp/foo",
		},
		{
			name:    "/tmp/foo",
			sandbox: "/tmp",
			want:    "/tmp/foo",
		},
		{
			name:    "/tmp/foo",
			sandbox: "/bar",
			want:    "/bar/tmp/foo",
		},
		{
			name:    "tmp/foo",
			sandbox: "/bar",
			want:    "/bar/tmp/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Set(defs.SandboxPathSetting, tt.sandbox)
			if got := sandboxName(tt.name); got != tt.want {
				t.Errorf("sandboxName() = %v, want %v", got, tt.want)
			}
		})
	}
}
