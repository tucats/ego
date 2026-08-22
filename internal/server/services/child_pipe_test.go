package services

// Tests for the socket-based ("pipe") child-service transport: the
// isPipeMode/runChildProcess helpers used by callChildServices (the
// parent), and ChildServicePipe/exchangeWithChild (the child and its
// counterpart). callChildServices itself spawns os.Args[0] as a real
// subprocess and so isn't practically unit-testable here (see
// child_status_test.go's equivalent note for the file transport); these
// tests cover everything below that boundary.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

func TestIsPipeMode(t *testing.T) {
	cases := []struct {
		dir  string
		want bool
	}{
		{"", true},
		{defs.ChildServicesPipeMode, true},
		{"/tmp", false},
		{"/var/tmp/ego-children", false},
		{"Pipe", false}, // reserved value match is exact, not case-insensitive
	}

	for _, c := range cases {
		if got := isPipeMode(c.dir); got != c.want {
			t.Errorf("isPipeMode(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
}

// TestHelperProcess isn't a real test -- it's a subprocess entry point that
// TestRunChildProcess_Timeout and TestRunChildProcess_NoTimeout exec via
// os.Args[0], the standard os/exec self-exec test pattern. This lets
// runChildProcess's start/wait/kill logic be exercised against a real OS
// process without depending on the real ego binary.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	defer os.Exit(0)

	fmt.Println("helper log line")

	if os.Getenv("GO_HELPER_SLEEP") == "1" {
		time.Sleep(10 * time.Second)
	}
}

func helperCommand(t *testing.T, sleep bool) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	env := append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

	if sleep {
		env = append(env, "GO_HELPER_SLEEP=1")
	}

	cmd.Env = env

	return cmd
}

func TestRunChildProcess_Timeout(t *testing.T) {
	cmd := helperCommand(t, true)

	start := time.Now()
	_, err := runChildProcess(cmd, 200*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Equals(err, errors.ErrChildRunTimeout) {
		t.Fatalf("err = %v, want ErrChildRunTimeout", err)
	}

	if elapsed > 5*time.Second {
		t.Fatalf("runChildProcess took %v to return -- the 200ms timeout did not kill the 10s sleep", elapsed)
	}
}

func TestRunChildProcess_NoTimeout(t *testing.T) {
	cmd := helperCommand(t, false)

	stdout, err := runChildProcess(cmd, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := strings.TrimSpace(string(stdout)); got != "helper log line" {
		t.Errorf("stdout = %q, want %q", got, "helper log line")
	}
}

func TestExchangeWithChild_TokenMismatch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer listener.Close()

	go func() {
		conn, dialErr := net.Dial("tcp", listener.Addr().String())
		if dialErr != nil {
			return
		}

		defer conn.Close()

		fmt.Fprintln(conn, "wrong-token")

		// Give exchangeWithChild a chance to read and reject the token
		// before this goroutine (and its connection) goes away.
		time.Sleep(100 * time.Millisecond)
	}()

	_, err = exchangeWithChild(listener, "expected-token", ChildServiceRequest{})
	if !errors.Equals(err, errors.ErrChildPipeAuth) {
		t.Fatalf("err = %v, want ErrChildPipeAuth", err)
	}
}

func TestChildServicePipe_MissingEnv(t *testing.T) {
	t.Setenv(defs.EgoChildPipeAddrEnv, "")
	t.Setenv(defs.EgoChildPipeTokenEnv, "")

	if err := ChildServicePipe(); !errors.Equals(err, errors.ErrChildPipeAuth) {
		t.Fatalf("err = %v, want ErrChildPipeAuth", err)
	}
}

// TestChildServicePipe_RoundTrip plays the parent's role directly (accept,
// verify the token, send the request, read the response) against a real
// ChildServicePipe() child, using the same fixture-writing helper
// (writeServiceFile, in endpoint_test.go) the file-transport tests in
// child_status_test.go use.
func TestChildServicePipe_RoundTrip(t *testing.T) {
	name := writeServiceFile(t, "import \"http\"\n\nfunc handler(req http.Request, w *http.ResponseWriter) {\n\tw.WriteHeader(200)\n\tw.Write(\"ok\")\n}\n")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer listener.Close()

	const token = "round-trip-token"

	t.Setenv(defs.EgoChildPipeAddrEnv, listener.Addr().String())
	t.Setenv(defs.EgoChildPipeTokenEnv, token)

	req := ChildServiceRequest{
		SessionID: 1,
		ServerID:  "pipe-test-server",
		Method:    http.MethodGet,
		Path:      "/services/pipe-test",
		Filename:  name,
	}

	childResult := make(chan error, 1)

	go func() {
		childResult <- ChildServicePipe()
	}()

	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	defer conn.Close()

	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read token: %v", err)
	}

	if strings.TrimSuffix(line, "\n") != token {
		t.Fatalf("token = %q, want %q", strings.TrimSuffix(line, "\n"), token)
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp ChildServiceResponse
	if err := json.NewDecoder(reader).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if err := <-childResult; err != nil {
		t.Fatalf("ChildServicePipe() error: %v", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d, want 200 -- Message: %q", resp.Status, resp.Message)
	}

	if strings.TrimSpace(resp.Body) != "ok" {
		t.Errorf("Body = %q, want %q", resp.Body, "ok")
	}
}
