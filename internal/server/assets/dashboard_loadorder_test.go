// This test guards an invariant of the dashboard's browser assets that nothing
// else can check. It lives in this package because this is the package that
// serves those assets, and because lib/ is archived wholesale into unzip.go by
// go:generate — a test file placed there would ship inside the asset bundle.
//
// The dashboard's JavaScript is split across several files, loaded in the order
// dashboard.html lists them. They are plain scripts sharing one global scope,
// so a function declaration is hoisted only within its own file. Code that runs
// immediately when a file is evaluated may therefore only reference names
// already declared by an earlier file. Deferred code — event handlers,
// callbacks, timers — is unrestricted, because by the time it runs every file
// has loaded.
//
// Breaking that rule throws a ReferenceError during page load, which silently
// kills the rest of that file. The dashboard then renders but does nothing,
// with no clue in the UI as to why. This has happened once: the tabLoaders map
// was left in a file that loaded before two of the loader functions it names.

package assets

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dashboardDir is the dashboard asset directory, relative to this package.
var dashboardDir = filepath.Join("..", "..", "..", "lib", "assets", "dashboard")

// scriptTagRE pulls the dashboard script file names out of dashboard.html.
// Deriving the load order from the HTML rather than hard-coding it here means
// this test cannot drift out of step with what is actually served.
var scriptTagRE = regexp.MustCompile(`<script src="[^"]*/(dashboard-[A-Za-z0-9_-]+\.js)"`)

// declRE matches a file-scope declaration: function, const, let, var or class
// at column zero. Anything indented sits inside some enclosing block and is not
// a file-scope name.
var declRE = regexp.MustCompile(`(?m)^(?:async\s+)?(?:function|const|let|var|class)\s+([A-Za-z_$][\w$]*)`)

// initializerRE matches a file-scope const/let/var declaration together with
// its initializer, stopping at the column-zero line that closes the statement.
var initializerRE = regexp.MustCompile(`(?ms)^(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(.*?)^(?:\};|\];|\);)\s*$`)

// identifierRE matches any identifier-shaped token.
var identifierRE = regexp.MustCompile(`[A-Za-z_$][\w$]*`)

// TestDashboardScriptLoadOrder checks that no file-scope declaration's
// initializer references a name that a later script declares.
//
// The check is deliberately conservative. An initializer containing "function"
// or "=>" builds a closure, so the names inside are resolved when that closure
// runs rather than at declaration time, and such initializers are skipped. Only
// initializers evaluated outright — object and array literals, direct
// references, calls — are examined. That is exactly the shape that broke the
// dashboard, and skipping the rest keeps this free of false alarms that would
// train people to ignore it.
func TestDashboardScriptLoadOrder(t *testing.T) {
	html, err := os.ReadFile(filepath.Join(dashboardDir, "dashboard.html"))
	if err != nil {
		t.Skip("dashboard.html not readable from this location:", err)
	}

	var order []string
	for _, match := range scriptTagRE.FindAllStringSubmatch(string(html), -1) {
		order = append(order, match[1])
	}

	if len(order) < 2 {
		t.Fatalf("expected several dashboard scripts in dashboard.html, found %d", len(order))
	}

	// Record which file declares each file-scope name, by load position.
	declaredIn := map[string]int{}
	sources := make([]string, len(order))

	for i, name := range order {
		body, err := os.ReadFile(filepath.Join(dashboardDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		sources[i] = string(body)

		for _, match := range declRE.FindAllStringSubmatch(sources[i], -1) {
			if _, seen := declaredIn[match[1]]; !seen {
				declaredIn[match[1]] = i
			}
		}
	}

	examined := 0

	for i, name := range order {
		for _, match := range initializerRE.FindAllStringSubmatch(sources[i], -1) {
			target, initializer := match[1], match[2]

			// A closure defers its name lookups; nothing to check here.
			if strings.Contains(initializer, "function") || strings.Contains(initializer, "=>") {
				continue
			}

			examined++

			for _, identifier := range identifierRE.FindAllString(initializer, -1) {
				at, known := declaredIn[identifier]
				if !known || at <= i {
					continue
				}

				t.Errorf("%s declares %q, whose initializer references %q, "+
					"but %q is declared in %s, which loads later.\n"+
					"An initializer is evaluated immediately, so this throws a "+
					"ReferenceError at page load and silently kills the rest of %s.\n"+
					"Move the declaration to a file at or after %s, or defer the "+
					"reference behind a function.",
					name, target, identifier, identifier, order[at], name, order[at])
			}
		}
	}

	// Guard the guard: if the patterns stop matching, this test would pass
	// while checking nothing at all.
	if examined == 0 {
		t.Error("no file-scope initializers were examined; the patterns are probably stale")
	}
}
