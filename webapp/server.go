package webapp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// Where we listen.
var listenAddr = ":8080"

// sourceFile is the optional file whose contents seed the editor on startup.
var sourceFile string

// httpServer is kept so handleQuit can call Shutdown on it.
var httpServer *http.Server

// mu serializes code execution so stdout capture doesn't race.
var mu sync.Mutex

type runRequest struct {
	Code    string `json:"code"`
	Trace   bool   `json:"trace,omitempty"`
	Console bool   `json:"console,omitempty"` // true → reuse the persistent symbol table (REPL mode)
}

type runResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func Server(c *cli.Context) error {
	var application string

	port, found := c.Integer("port")
	if !found {
		port = 8080
	}

	if c.WasFound("launch") {
		application, _ = c.String("launch")
	}

	go launchBrowser(application, port)

	if len(c.FindGlobal().Parameters) > 0 {
		sourceFile = c.FindGlobal().Parameters[0]
	}

	listenAddr = fmt.Sprintf("localhost:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/style.css", serveCSS)
	mux.HandleFunc("/app.js", serveJS)
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/run", handleRun)
	mux.HandleFunc("/quit", handleQuit)

	httpServer = &http.Server{Addr: listenAddr, Handler: mux}

	// Catch Ctrl-C (SIGINT) and SIGTERM so the server shuts down cleanly
	// instead of being killed mid-request. The goroutine blocks until the
	// OS delivers a signal, then calls Shutdown which drains in-flight
	// requests before returning.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh

		_ = httpServer.Shutdown(context.Background())
	}()

	msg := i18n.M("webapp.start", ui.A{
		"address": listenAddr})

	ui.Say(msg)

	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		ui.Say(i18n.M("webapp.stopped"))

		return nil
	}

	if err != nil {
		return errors.New(err)
	}

	return nil
}

func launchBrowser(application string, port int) {
	launchCommand := map[string]string{
		"darwin": "open -a %s -u localhost:%d",
	}

	preferredBrowser := map[string]string{
		"darwin":  "Safari",
		"linux":   "Firefox",
		"windows": "Edge",
	}
	// Wait a moment before starting to ensure the server listener
	// has started.
	time.Sleep(1 * time.Second)

	// Using the platform-specific command string to launch the browser.
	cmd, found := launchCommand[runtime.GOOS]
	if !found {
		url := fmt.Sprintf("http://localhost:%d", port)
		ui.Say(i18n.M("webapp.browser.no.launch", ui.A{
			"os":  runtime.GOOS,
			"url": url}))

		return
	}

	// Inject the preferred browser name.
	if application == "" {
		application = preferredBrowser[runtime.GOOS]
	}

	// Form a complete command using the application name and port
	cmd = fmt.Sprintf(cmd, application, port)

	// Extract the arguments for the command to a string array so a
	// valid command invocation can be made.
	args := strings.Split(cmd, " ")

	// Execute the given command as a subprocess.
	err := exec.Command(args[0], args[1:]...).Start()
	if err != nil {
		ui.Say(i18n.M("webapp.browser.launch.error", ui.A{
			"err": err.Error()}))
	}
}
