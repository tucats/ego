package i18n

import (
	"os"
	"testing"

	"github.com/tucats/ego/defs"
)

func TestT(t *testing.T) {
	// Set up test cases
	tests := []struct {
		name      string
		key       string
		valueMap  map[string]interface{}
		want      string
		wantError bool
	}{
		{
			name: "test with valid key and no valueMap",
			key:  "ego.hello",
			want: "Hello, {{name}}!",
		},
		{
			name: "test with valid key and valueMap",
			key:  "ego.hello",
			valueMap: map[string]interface{}{
				"name": "Tom",
			},
			want: "Hello, Tom!",
		},
		{
			name:      "test with invalid key",
			key:       "invalid",
			want:      "invalid",
			wantError: true,
		},
	}

	// Set up environment variables
	os.Setenv(defs.EgoLangEnv, "en")

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := T(tt.key, tt.valueMap)
			if got != tt.want {
				t.Errorf("T(%s) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
