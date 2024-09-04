package util

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGibberish(t *testing.T) {
	// Set up test cases
	tests := []struct {
		name string
		u    uuid.UUID
		want string
	}{
		{
			name: "test 1 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000000"),
			want: "-empty-",
		},
		{
			name: "test 2 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			want: "b",
		},
		{
			name: "test 3 synthetic UUID",
			u:    uuid.MustParse("10000000-0000-0000-0000-000000000000"),
			want: "bpefgcxt4wnrb",
		},
		{
			name: "test 1 random UUID",
			u:    uuid.MustParse("ab34d542-a437-408a-b0ca-38ea5d78696f"),
			want: "uv3n6jjm5qhca2yz6aryyvtxs",
		},
		{
			name: "test 2 random UUID",
			u:    uuid.MustParse("4867dd02-3b98-4d68-9843-06179aa8553e"),
			want: "zyvbd7qsta2dk6jfqx57tmwg",
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Gibberish(tt.u)
			if got != tt.want {
				t.Errorf("Gibberish(%s) = %v, want %v", tt.u, got, tt.want)
			}
		})
	}
}

func TestFormatDuration_SubSecondDuration(t *testing.T) {
	d := 355 * time.Millisecond
	expected := "355ms"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NegativeDuration(t *testing.T) {
	d := -1 * time.Minute
	expected := "-1m"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NonIntegerDuration(t *testing.T) {
	d, _ := time.ParseDuration("15m30s")
	expected := "15m 30s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NonIntegerDurationWithoutSpaces(t *testing.T) {
	d, _ := time.ParseDuration("15m30s")
	expected := "15m30s"

	result := FormatDuration(d, false)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_ZeroDuration(t *testing.T) {
	d := 0 * time.Second
	expected := "0s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_LessThanSecond(t *testing.T) {
	d := 300 * time.Millisecond
	expected := "300ms"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_Days(t *testing.T) {
	d := time.Hour * 24 * 32
	expected := "32d"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_MoreThanADay(t *testing.T) {
	d, _ := time.ParseDuration("774h23m15s")
	expected := "32d 6h 23m 15s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_MoreThanADayUsingDefault(t *testing.T) {
	d, _ := time.ParseDuration("774h23m15s")
	expected := "774h23m15s"
	
	result := FormatDuration(d, false)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "string with no special characters",
			input: "Hello, World!",
			want:  "Hello, World!",
		},
		{
			name:  "string with a double quote",
			input: `Hello, "World!"`,
			want:  `Hello, \"World!\"`,
		},
		{
			name: "string with a newline",
			input: `Hello,
World!`,
			want: `Hello,\nWorld!`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Escape(tt.input)
			if got != tt.want {
				t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
