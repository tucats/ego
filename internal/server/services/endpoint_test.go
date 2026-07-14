package services

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
)

// writeServiceFile creates a scratch .ego file containing body and returns
// its path, for use as input to parseEndpoint/parseAuthenticated.
func writeServiceFile(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	name := filepath.Join(dir, "service.ego")

	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write scratch service file: %v", err)
	}

	return name
}

func TestParseEndpoint_NoDirective(t *testing.T) {
	name := writeServiceFile(t, "func main() {\n\tfmt.Println(\"hi\")\n}\n")

	spec, err := parseEndpoint(name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec != nil {
		t.Fatalf("expected nil spec when no @endpoint directive is present, got %+v", spec)
	}
}

func TestParseEndpoint_LegacyBareString(t *testing.T) {
	name := writeServiceFile(t, `@endpoint "GET /services/foo"`+"\n")

	spec, err := parseEndpoint(name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Path != "/services/foo" {
		t.Errorf("Path = %q, want %q", spec.Path, "/services/foo")
	}

	if spec.Method != http.MethodGet {
		t.Errorf("Method = %q, want %q", spec.Method, http.MethodGet)
	}
}

func TestParseEndpoint_LegacyBareStringWithParameters(t *testing.T) {
	name := writeServiceFile(t, `@endpoint "POST /services/foo?name=string&age=int"`+"\n")

	spec, err := parseEndpoint(name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Path != "/services/foo" {
		t.Errorf("Path = %q, want %q", spec.Path, "/services/foo")
	}

	if spec.Method != http.MethodPost {
		t.Errorf("Method = %q, want %q", spec.Method, http.MethodPost)
	}

	if spec.Parameters["name"] != "string" || spec.Parameters["age"] != "int" {
		t.Errorf("Parameters = %+v, want name:string, age:int", spec.Parameters)
	}
}

func TestParseEndpoint_NewSyntax(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *endpointSpec
	}{
		{
			name: "method and path=",
			body: `@endpoint get path="/services/foo"` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: http.MethodGet, Parameters: map[string]string{}},
		},
		{
			name: "path= before method (order independence)",
			body: `@endpoint path="/services/foo" get` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: http.MethodGet, Parameters: map[string]string{}},
		},
		{
			name: "update method",
			body: `@endpoint update path="/services/foo"` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: "UPDATE", Parameters: map[string]string{}},
		},
		{
			name: "media list",
			body: `@endpoint path="/services/foo" media="application/json","text/plain"` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{}, MediaTypes: []string{"application/json", "text/plain"}},
		},
		{
			name: "permissions list",
			body: `@endpoint path="/services/foo" permissions="ego.write","ego.read"` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{}, Permissions: []string{"ego.write", "ego.read"}},
		},
		{
			name: "parameter list",
			body: `@endpoint path="/services/foo" parameter="filter:string","limit:int"` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{"filter": "string", "limit": "int"}},
		},
		{
			name: "authenticated bare token",
			body: `@endpoint path="/services/foo" authenticated` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{}, Authenticated: true},
		},
		{
			name: "admin bare token requires admin and contributes permissions=ego.root",
			body: `@endpoint path="/services/foo" admin` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{}, Permissions: []string{"ego.root"}, Admin: true},
		},
		{
			name: "root bare token requires admin and contributes permissions=ego.root",
			body: `@endpoint path="/services/foo" root` + "\n",
			want: &endpointSpec{Path: "/services/foo", Method: router.AnyMethod, Parameters: map[string]string{}, Permissions: []string{"ego.root"}, Admin: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := writeServiceFile(t, tt.body)

			got, err := parseEndpoint(name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Path != tt.want.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.want.Path)
			}

			if got.Method != tt.want.Method {
				t.Errorf("Method = %q, want %q", got.Method, tt.want.Method)
			}

			if got.Authenticated != tt.want.Authenticated {
				t.Errorf("Authenticated = %v, want %v", got.Authenticated, tt.want.Authenticated)
			}

			if got.Admin != tt.want.Admin {
				t.Errorf("Admin = %v, want %v", got.Admin, tt.want.Admin)
			}

			if !stringSliceEqual(got.MediaTypes, tt.want.MediaTypes) {
				t.Errorf("MediaTypes = %v, want %v", got.MediaTypes, tt.want.MediaTypes)
			}

			if !stringSliceEqual(got.Permissions, tt.want.Permissions) {
				t.Errorf("Permissions = %v, want %v", got.Permissions, tt.want.Permissions)
			}

			if !stringMapEqual(got.Parameters, tt.want.Parameters) {
				t.Errorf("Parameters = %v, want %v", got.Parameters, tt.want.Parameters)
			}
		})
	}
}

func TestParseEndpoint_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unrecognized bare term", `@endpoint bogus path="/x"` + "\n"},
		{"unrecognized parameter kind", `@endpoint path="/x" parameter="n:bogus"` + "\n"},
		{"missing path", `@endpoint get` + "\n"},
		{"duplicate method", `@endpoint get put path="/x"` + "\n"},
		{"duplicate authentication term", `@endpoint path="/x" admin root` + "\n"},
		{"unrecognized keyword term", `@endpoint path="/x" bogus="y"` + "\n"},
		{"media with no value", `@endpoint path="/x" media=` + "\n"},
		{"malformed parameter, no colon", `@endpoint path="/x" parameter="badformat"` + "\n"},
		{"duplicate path term", `@endpoint path="/x" path="/y"` + "\n"},
		{"bare string after path= already set", `@endpoint path="/x" "/y"` + "\n"},
		{"duplicate media term", `@endpoint path="/x" media="a" media="b"` + "\n"},
		{"duplicate permissions term", `@endpoint path="/x" permissions="a" permissions="b"` + "\n"},
		{"duplicate parameter name", `@endpoint path="/x" parameter="n:int","n:string"` + "\n"},
		{"bare @endpoint with nothing after", "@endpoint\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := writeServiceFile(t, tt.body)

			spec, err := parseEndpoint(name)
			if err == nil {
				t.Fatalf("expected an error, got spec %+v", spec)
			}

			if !errors.Equals(err, errors.ErrInvalidEndPointDefinition) {
				t.Errorf("error = %v, want ErrInvalidEndPointDefinition", err)
			}

			if spec != nil {
				t.Errorf("expected nil spec on error, got %+v", spec)
			}
		})
	}
}

func TestParseAuthenticated(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantAuthenticate bool
		wantAdmin        bool
	}{
		{"no directive", "func main() {}\n", false, false},
		{"admin", "@authenticated admin\n", true, true},
		{"root", "@authenticated root\n", true, true},
		{"user", "@authenticated user\n", true, false},
		{"none", "@authenticated none\n", false, false},
		// A bare "@authenticated" with nothing after it reads the following
		// synthetic ";" as its "kind" token, which matches the same branch
		// as "none" -- so, somewhat surprisingly, it behaves as NOT
		// authenticated. This is pre-existing behavior (unchanged from the
		// original getPattern implementation), not something introduced by
		// this rewrite -- pinning it down here so a future change to this
		// logic doesn't silently invert it.
		{"bare directive is equivalent to none", "@authenticated\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := writeServiceFile(t, tt.body)

			authenticate, admin := parseAuthenticated(name)
			if authenticate != tt.wantAuthenticate {
				t.Errorf("authenticate = %v, want %v", authenticate, tt.wantAuthenticate)
			}

			if admin != tt.wantAdmin {
				t.Errorf("admin = %v, want %v", admin, tt.wantAdmin)
			}
		})
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}
