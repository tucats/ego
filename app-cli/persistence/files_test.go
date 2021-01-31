// Package persistence manages the persistent user profile used by the command
// application infrastructure. This includes automatically reading any
// profile in as part of startup, and of updating the profile as needed.
package persistence

import "testing"

func TestLoad(t *testing.T) {
	type args struct {
		application string
		name        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"read existing config", args{application: "cli-driver", name: "default"}, false},
		{"read non-existant config", args{application: "no-such-app", name: "default"}, true},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Load(tt.args.application, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
