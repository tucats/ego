package filepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestJoin(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	// Test case 1: Join two paths
	args := data.NewList(
		"path/to",
		"file.txt",
	)

	expected := "path/to/file.txt"

	result, err := join(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test case 2: Join three paths
	args = data.NewList(
		"home",
		"documents",
		"report.pdf",
	)
	expected = "home/documents/report.pdf"

	result, err = join(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test case 3: Join an empty list of paths
	args = data.NewList()
	expected = ""

	result, err = join(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test case 4: Join a list containing an empty string
	args = data.NewList(
		"path",
		"",
		"file.txt",
	)
	expected = "path/file.txt"

	result, err = join(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test case 5: Join a list containing only empty strings
	args = data.NewList(
		"",
		"",
		"",
	)
	expected = ""

	result, err = join(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestBase(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple file name",
			path: "file.txt",
			want: "file.txt",
		},
		{
			name: "file name with directory",
			path: "path/to/file.txt",
			want: "file.txt",
		},
		{
			name: "file name with multiple directories",
			path: "path/to/sub/file.txt",
			want: "file.txt",
		},
		{
			name: "empty path",
			path: "",
			want: ".",
		},
		{
			name: "path with trailing slash",
			path: "path/to/",
			want: "to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := data.NewList(tt.path)

			got, err := base(s, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, got)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		name string
		path string
		want string
		err  bool
	}{
		{
			name: "relative path",
			path: "test/file.txt",
			want: filepath.Join(os.Getenv("PWD"), "test/file.txt"),
			err:  false,
		},
		{
			name: "absolute path",
			path: "/home/user/documents/report.pdf",
			want: "/home/user/documents/report.pdf",
			err:  false,
		},
		{
			name: "path with ..",
			path: "parent/child/../file.txt",
			want: filepath.Join(os.Getenv("PWD"), "parent/file.txt"),
			err:  false,
		},
		{
			name: "empty path",
			path: "",
			want: filepath.Join(os.Getenv("PWD"), ""),
			err:  false,
		},
		{
			name: "non-existent path",
			path: "/nonexistent/file.txt",
			want: "/nonexistent/file.txt",
			err:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := data.NewList(tt.path)

			got, err := abs(s, args)
			if (err != nil) != tt.err {
				t.Errorf("Expected error: %v, got: %v", tt.err, err != nil)
			}

			if err == nil && got != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, got)
			}
		})
	}
}

func TestExt(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "file with extension",
			path: "example.txt",
			want: ".txt",
		},
		{
			name: "file without extension",
			path: "example",
			want: "",
		},
		{
			name: "file with multiple extensions",
			path: "example.tar.gz",
			want: ".gz",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "path with leading dot",
			path: "./example.txt",
			want: ".txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := data.NewList(tt.path)

			got, err := ext(s, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, got)
			}
		})
	}
}

func TestDir(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple directory",
			path: "/home/user/documents",
			want: "/home/user",
		},
		{
			name: "directory with subdirectories",
			path: "/home/user/documents/reports/2022",
			want: "/home/user/documents/reports",
		},
		{
			name: "directory with trailing slash",
			path: "/home/user/documents/",
			want: "/home/user/documents",
		},
		{
			name: "root directory",
			path: "/",
			want: "/",
		},
		{
			name: "relative path",
			path: "documents/reports/2022",
			want: "documents/reports",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := data.NewList(tt.path)

			got, err := dir(s, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, got)
			}
		})
	}
}

func TestClean(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple path",
			path: "path/to/file.txt",
			want: "path/to/file.txt",
		},
		{
			name: "path with ..",
			path: "parent/child/../file.txt",
			want: "parent/file.txt",
		},
		{
			name: "path with multiple ..",
			path: "parent/child/../../file.txt",
			want: "file.txt",
		},
		{
			name: "path with trailing slash",
			path: "parent/child/../../file.txt/",
			want: "file.txt",
		},
		{
			name: "path with .",
			path: "parent/child/./file.txt",
			want: "parent/child/file.txt",
		},
		{
			name: "path with multiple slashes",
			path: "parent/child///file.txt",
			want: "parent/child/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := data.NewList(tt.path)

			got, err := clean(s, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, got)
			}
		})
	}

	// Test case for absolute path
	absPath := filepath.Join(os.Getenv("PWD"), "test/file.txt")
	args := data.NewList(absPath)
	want := filepath.Join(os.Getenv("PWD"), "test/file.txt")

	got, err := clean(s, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if got != want {
		t.Errorf("Expected '%s', got '%s'", want, got)
	}
}
