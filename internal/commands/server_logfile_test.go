package commands

import "testing"

func TestQualifyServerLogFileName(t *testing.T) {
	tests := []struct {
		name        string
		fn          string
		clusterName string
		port        int
		defaultPort int
		want        string
	}{
		{"cluster mode always qualifies", "ego-server.log", "gang", 8501, 443, "ego-server_gang_8501.log"},
		{"cluster mode at default port still qualifies", "ego-server.log", "gang", 443, 443, "ego-server_gang_443.log"},
		{"standalone non-default port", "ego-server.log", "", 8501, 443, "ego-server_8501.log"},
		{"standalone default port unchanged", "ego-server.log", "", 443, 443, "ego-server.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qualifyServerLogFileName(tt.fn, tt.clusterName, tt.port, tt.defaultPort); got != tt.want {
				t.Errorf("qualifyServerLogFileName(%q, %q, %d, %d) = %q, want %q",
					tt.fn, tt.clusterName, tt.port, tt.defaultPort, got, tt.want)
			}
		})
	}
}

func TestQualifyArchiveFileName(t *testing.T) {
	tests := []struct {
		name        string
		fn          string
		clusterName string
		want        string
	}{
		{"cluster mode qualifies by cluster name only", "archive.zip", "gang", "archive_gang.zip"},
		{"standalone unchanged", "archive.zip", "", "archive.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qualifyArchiveFileName(tt.fn, tt.clusterName); got != tt.want {
				t.Errorf("qualifyArchiveFileName(%q, %q) = %q, want %q", tt.fn, tt.clusterName, got, tt.want)
			}
		})
	}
}
