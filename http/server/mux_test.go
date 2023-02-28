package server

import (
	"testing"
)

func TestMux_findRoute(t *testing.T) {
	tests := []struct {
		name  string
		found bool
	}{
		{
			name:  "/services/admin/",
			found: true,
		},
		{
			name:  "/services/admin/users",
			found: true,
		},
		{
			name:  "/services",
			found: false,
		},
		{
			name:  "/",
			found: false,
		},
		{
			name:  "/services/admin/cache",
			found: true,
		},
	}

	// Set up the test routes
	m := NewRouter("testing")
	m.NewRoute("/services/admin/users/", nil)
	m.NewRoute("/services/admin/cache/", nil)
	m.NewRoute("/services/admin/", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.findRoute(tt.name, "")
			if (got != nil) != tt.found {
				t.Errorf("Mux.findRoute() = %v, want %v", got != nil, tt.found)
			}
		})
	}
}
