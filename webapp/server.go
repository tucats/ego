package webapp

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
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
	Code  string `json:"code"`
	Trace bool   `json:"trace,omitempty"`
}

type runResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func Server(c *cli.Context) error {
	port, found := c.Integer("port")
	if !found {
		port = 8080
	}

	if c.WasFound("launch") {
		go launchBrowser(port)
	}

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

func launchBrowser(port int) {
	launchCommand := map[string]string{
		"darwin": "open -a Safari -u localhost:%d",
	}

	// Wait a second before starting to ensure the server listener
	// has started.
	time.Sleep(1 * time.Second)

	// Using the platform-specific command string to launch the browser.
	cmd, found := launchCommand[runtime.GOOS]
	if !found {
		ui.Say(i18n.M("webapp.browser.no.launch", ui.A{
			"os": runtime.GOOS}))
	}

	// Extract the arguments for the command to a string
	// array.
	args := strings.Split(fmt.Sprintf(cmd, port), " ")

	// Execute the given command as a subprocess.
	err := exec.Command(args[0], args[1:]...).Start()
	if err != nil {
		ui.Say(i18n.M("webapp.browser.launch.error", ui.A{
			"err": err.Error()}))
	} else {
		ui.Say(i18n.M("webapp.browser.launch.success"))
	}
}
