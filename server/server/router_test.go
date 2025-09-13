package server

import (
	"reflect"
	"testing"
)

func TestMux_findRoute(t *testing.T) {
	tests := []struct {
		name  string
		found bool
	}{
		{
			name:  "/services/admin/use",
			found: false,
		},
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
	m.New("/services/admin/users/", nil, AnyMethod)
	m.New("/services/admin/cache/", nil, AnyMethod)
	m.New("/services/admin/", nil, AnyMethod)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := m.FindRoute(AnyMethod, tt.name)
			if (got != nil) != tt.found {
				t.Errorf("Mux.findRoute() = %v, want %v", got != nil, tt.found)
			}
		})
	}
}

func TestRoute_makeMap(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
		want    map[string]any
	}{
		{
			path:    "/services/sample",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": false, "name": "", "field": ""},
		},
		{
			path:    "/services/sample/users",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "", "field": ""},
		},
		{
			path:    "/services/sample/users/",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "", "field": ""},
		},
		{
			path:    "/services/sample/users/mary",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "mary", "field": ""},
		},
		{
			path:    "/services/sample/users/mary/",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "mary", "field": ""},
		},
		{
			path:    "/services/sample/users/mary/age",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "mary", "field": "age"},
		},
		{
			path:    "/services/sample/users/mary/age/",
			pattern: "/services/sample/users/{{name}}/{{field}}",
			want:    map[string]any{"services": true, "sample": true, "users": true, "name": "mary", "field": "age"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			r := &Route{
				endpoint: tt.pattern,
			}
			if got := r.partsMap(tt.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Route.makeMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
