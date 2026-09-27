package commands

import (
	"os"
	"strings"
	"testing"
)

func TestParseServerSetting(t *testing.T) {
	tests := []struct {
		server string
		host   string
		port   int
	}{
		{"https://babygrand.local:443", "babygrand.local", 443},
		{"https://babygrand.local:8443", "babygrand.local", 8443},
		{"https://babygrand.local", "babygrand.local", 443},
		{"http://myhost", "myhost", 80},
		{"https://myhost/", "myhost", 443},
		{"myhost:8080", "myhost", 8080},
		{"myhost", "myhost", 443},
		{"https://[::1]:4443", "::1", 4443},
		{"  https://myhost:443  ", "myhost", 443},
	}

	for _, tt := range tests {
		u := parseServerSetting(tt.server)
		if u == nil {
			t.Errorf("parseServerSetting(%q) = nil", tt.server)

			continue
		}

		if got := u.Hostname(); got != tt.host {
			t.Errorf("parseServerSetting(%q) host = %q, want %q", tt.server, got, tt.host)
		}

		if got := serverURLPort(u); got != tt.port {
			t.Errorf("serverURLPort(%q) = %d, want %d", tt.server, got, tt.port)
		}
	}

	for _, server := range []string{"", "   ", "https://"} {
		if u := parseServerSetting(server); u != nil {
			t.Errorf("parseServerSetting(%q) = %v, want nil", server, u)
		}
	}
}

func TestSameHostName(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"babygrand.local", "BabyGrand", true},
		{"BabyGrand", "babygrand.local", true},
		{"babygrand.local", "BABYGRAND.LOCAL", true},
		{"babygrand", "babygrand", true},
		// Two fully qualified names in different domains are different hosts.
		{"babygrand.local", "babygrand.example.com", false},
		{"otherhost.local", "BabyGrand", false},
		{"", "BabyGrand", false},
	}

	for _, tt := range tests {
		if got := sameHostName(tt.a, tt.b); got != tt.want {
			t.Errorf("sameHostName(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIsLocalHost(t *testing.T) {
	for _, host := range []string{"localhost", "LocalHost", "127.0.0.1", "::1"} {
		if !isLocalHost(host) {
			t.Errorf("isLocalHost(%q) = false, want true", host)
		}
	}

	// This machine's own host name, bare and with a domain suffix added, must
	// be recognized without depending on DNS.
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		short, _, _ := strings.Cut(hostname, ".")

		for _, host := range []string{hostname, strings.ToUpper(hostname), short + ".local"} {
			if !isLocalHost(host) {
				t.Errorf("isLocalHost(%q) = false, want true (host name %q)", host, hostname)
			}
		}
	}

	// 192.0.2.1 is in TEST-NET-1 (RFC 5737), reserved for documentation, so it
	// is never assigned to a real interface. Being an address literal, looking
	// it up does not touch the network.
	if isLocalHost("192.0.2.1") {
		t.Error("isLocalHost(\"192.0.2.1\") = true, want false")
	}
}
