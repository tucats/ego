package commands

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// The functions in this file decide which local server a "server start",
// "stop", "restart", or "status --local" command is about. A server is
// identified by the port it listens on: its pid file is named for that port,
// and stop and restart send their REST requests to it.

// startPort returns the port a new server will be started on: the --port
// option if given, else the configured default port, else the standard port
// for the server's scheme.
func startPort(c *cli.Context, insecure bool) int {
	if port, ok := c.Integer("port"); ok && port > 0 {
		return port
	}

	return defaultServerPort(insecure)
}

// defaultServerPort returns the configured default server port, or if there
// is none, the standard port for the server's scheme (80 for an insecure HTTP
// server, 443 for HTTPS).
func defaultServerPort(insecure bool) int {
	if port := settings.GetInt(defs.ServerDefaultPortSetting); port > 0 {
		return port
	}

	if insecure {
		return 80
	}

	return 443
}

// managedServerPort returns the port of the running server that a stop,
// restart, or local status command is about. In order of preference, this is:
//
//   - the --port option, if given;
//   - the port of the application server (or, if there is none, the logon
//     server) in the configuration, if that server is on this machine. So after
//     starting a server and logging into it, "ego server restart" restarts the
//     server being used without having to repeat its port;
//   - the configured default server port;
//   - 443, the standard HTTPS port.
func managedServerPort(c *cli.Context) int {
	if port, ok := c.Integer("port"); ok && port > 0 {
		return port
	}

	if u := configuredLocalServer(); u != nil {
		if port := serverURLPort(u); port > 0 {
			return port
		}
	}

	return defaultServerPort(false)
}

// localServerURL builds the http(s)://host:port base URL for the locally
// managed server on the given port. The host comes from localServerHost. The
// scheme is taken from the pid file arguments (as they were passed to "server
// start") when there are any, since it must reflect how this specific server
// instance was actually launched; otherwise it is taken from the configured
// server, if that is this machine; otherwise it is https.
func localServerURL(status *defs.ServerStatus, port int) string {
	scheme := "https"

	if status != nil && len(status.Args) > 0 {
		for _, v := range status.Args {
			if v == "-k" || v == "--not-secure" {
				scheme = "http"
			}
		}
	} else if u := configuredLocalServer(); u != nil && strings.EqualFold(u.Scheme, "http") {
		scheme = "http"
	}

	// JoinHostPort adds the square brackets an IPv6 address needs in a URL.
	return scheme + "://" + net.JoinHostPort(localServerHost(), strconv.Itoa(port))
}

// localServerHost returns the host name to use when talking to the local
// server. This is the host from the application server setting (or the logon
// server setting if there is no application server), provided that host is
// this machine. The server's TLS certificate is issued for that name, so a
// request to "localhost" fails certificate verification unless the certificate
// happens to also list "localhost".
//
// If neither setting names this machine (for example, the user last logged
// into some remote server), the result is "localhost". Stop and restart only
// ever manage the server on this machine, so a remote name is never used.
func localServerHost() string {
	if u := configuredLocalServer(); u != nil {
		return u.Hostname()
	}

	return "localhost"
}

// configuredLocalServer returns the parsed application server setting (or the
// logon server setting if there is no application server), if it names a
// server on this machine. Otherwise it returns nil.
func configuredLocalServer() *url.URL {
	server := settings.Get(defs.ApplicationServerSetting)
	if server == "" {
		server = settings.Get(defs.LogonServerSetting)
	}

	if u := parseServerSetting(server); u != nil && isLocalHost(u.Hostname()) {
		return u
	}

	return nil
}

// parseServerSetting parses a server setting, which is normally a URL such as
// "https://myhost.local:443" but may also be a bare "host" or "host:port" (in
// which case the scheme is taken to be https). The result is nil if there is
// no host.
func parseServerSetting(server string) *url.URL {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil
	}

	if !strings.Contains(server, "://") {
		server = "https://" + server
	}

	u, err := url.Parse(server)
	if err != nil || u.Hostname() == "" {
		return nil
	}

	return u
}

// serverURLPort returns the port in a server URL: the explicit port if there
// is one, else the standard port for its scheme. It is zero if there is
// neither.
func serverURLPort(u *url.URL) int {
	if p := u.Port(); p != "" {
		port, _ := strconv.Atoi(p)

		return port
	}

	switch strings.ToLower(u.Scheme) {
	case "http":
		return 80
	case "https":
		return 443
	}

	return 0
}

// isLocalHost reports whether host names this machine. It is local if it is a
// loopback name or address, if it matches the machine's host name (ignoring
// case and any domain suffix, so "myhost.local" matches a host name of
// "MyHost"), or if it resolves to an address on one of this machine's network
// interfaces.
func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}

	if hostname, err := os.Hostname(); err == nil && sameHostName(host, hostname) {
		return true
	}

	addrs, err := net.LookupHost(host)
	if err != nil {
		return false
	}

	localAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}

		if ip.IsLoopback() {
			return true
		}

		for _, local := range localAddrs {
			if ipNet, ok := local.(*net.IPNet); ok && ipNet.IP.Equal(ip) {
				return true
			}
		}
	}

	return false
}

// sameHostName reports whether two host names name the same machine, comparing
// them without regard to case. If either name is not fully qualified, only the
// first label of the other is compared, so "myhost" and "MyHost.local" match.
func sameHostName(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}

	if !strings.Contains(a, ".") || !strings.Contains(b, ".") {
		a, _, _ = strings.Cut(a, ".")
		b, _, _ = strings.Cut(b, ".")

		return a != "" && strings.EqualFold(a, b)
	}

	return false
}
