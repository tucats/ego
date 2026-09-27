package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// usePidDirectory points the pid directory setting at a fresh temporary
// directory for the duration of the test, and returns that directory.
func usePidDirectory(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	saved := settings.Get(defs.PidDirectorySetting)

	settings.Set(defs.PidDirectorySetting, dir)
	t.Cleanup(func() { settings.Set(defs.PidDirectorySetting, saved) })

	return dir
}

func TestPidFileNameAlwaysHasPort(t *testing.T) {
	dir := usePidDirectory(t)

	if got, want := PidFileName(443), filepath.Join(dir, "ego-server-443.pid"); got != want {
		t.Errorf("PidFileName(443) = %q, want %q", got, want)
	}

	if got, want := PidFileName(8080), filepath.Join(dir, "ego-server-8080.pid"); got != want {
		t.Errorf("PidFileName(8080) = %q, want %q", got, want)
	}
}

func TestPidFileRoundTrip(t *testing.T) {
	usePidDirectory(t)

	status := defs.ServerStatus{PID: 1234, Args: []string{"ego", "server", "run", "--port", "8443"}}

	if err := WritePidFile(&cli.Context{Version: "test"}, 8443, status); err != nil {
		t.Fatalf("WritePidFile: %v", err)
	}

	got, err := ReadPidFile(8443)
	if err != nil {
		t.Fatalf("ReadPidFile: %v", err)
	}

	if got.PID != 1234 || got.Port != 8443 {
		t.Errorf("ReadPidFile(8443) = pid %d port %d, want pid 1234 port 8443", got.PID, got.Port)
	}

	// Another port's pid file is a different server.
	if _, err := ReadPidFile(443); err == nil {
		t.Error("ReadPidFile(443) found a pid file, but only port 8443 was written")
	}

	if err := RemovePidFile(8443); err != nil {
		t.Fatalf("RemovePidFile: %v", err)
	}

	if _, err := os.Stat(PidFileName(8443)); !os.IsNotExist(err) {
		t.Errorf("pid file still present after RemovePidFile: %v", err)
	}

	// Removing a pid file that is not there is not an error.
	if err := RemovePidFile(8443); err != nil {
		t.Errorf("RemovePidFile of a missing file: %v", err)
	}
}

// A server started before the port was always in the pid file name has a pid
// file named "ego-server.pid". It is used for its own port only, and removed
// along with that port's pid file.
func TestLegacyPidFile(t *testing.T) {
	dir := usePidDirectory(t)
	legacy := filepath.Join(dir, "ego-server.pid")

	b, _ := json.Marshal(defs.ServerStatus{PID: 99, Args: []string{"ego", "server", "run", "--port", "443"}})
	if err := os.WriteFile(legacy, b, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPidFile(443)
	if err != nil {
		t.Fatalf("ReadPidFile(443) with only a legacy pid file: %v", err)
	}

	if got.PID != 99 || got.Port != 443 {
		t.Errorf("ReadPidFile(443) = pid %d port %d, want pid 99 port 443", got.PID, got.Port)
	}

	if _, err := ReadPidFile(8080); err == nil {
		t.Error("ReadPidFile(8080) used a legacy pid file that records port 443")
	}

	if err := RemovePidFile(8080); err != nil {
		t.Fatalf("RemovePidFile(8080): %v", err)
	}

	if _, err := os.Stat(legacy); err != nil {
		t.Error("RemovePidFile(8080) removed a legacy pid file for port 443")
	}

	if err := RemovePidFile(443); err != nil {
		t.Fatalf("RemovePidFile(443): %v", err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("RemovePidFile(443) left the legacy pid file for port 443 in place")
	}
}

func TestPortFromArgs(t *testing.T) {
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"ego", "server", "run", "--port", "443"}, 443},
		{[]string{"ego", "-p", "8080", "server"}, 8080},
		{[]string{"ego", "server", "run"}, 0},
		{[]string{"ego", "server", "run", "--port"}, 0},
		{[]string{"ego", "--port", "abc"}, 0},
	}

	for _, tt := range tests {
		if got := PortFromArgs(tt.args); got != tt.want {
			t.Errorf("PortFromArgs(%v) = %d, want %d", tt.args, got, tt.want)
		}
	}
}
