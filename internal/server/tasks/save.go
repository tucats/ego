package tasks

import (
	"regexp"
	"strings"
	"sync"
)

// substitutionPattern matches a {{name}} token. Unlike tools/apitest's
// dictionary package (a separate Go module this package can't import),
// this only supports plain name substitution -- no "|format" pipe
// directives, no "$uuid"/"$hash" dynamic values. Tasks only ever need to
// carry a saved value from one call into another's endpoint, parameters,
// or body, not the richer templating apitest's test-authoring format needs.
var substitutionPattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

var (
	saveLock sync.RWMutex
	saved    = map[string]string{}
)

// setSaved stores a value in the global, cross-task substitution
// dictionary under the given name, per a task's "save" block.
func setSaved(name, value string) {
	saveLock.Lock()
	defer saveLock.Unlock()

	saved[name] = value
}

// substitute replaces every {{name}} token in text with the corresponding
// value from the global save dictionary. A token with no saved value --
// including every token before any task has ever populated one -- is left
// in the text unchanged, so a resulting malformed downstream request makes
// the missing substitution visible in the log rather than silently
// vanishing into an empty string.
func substitute(text string) string {
	if !strings.Contains(text, "{{") {
		return text
	}

	saveLock.RLock()
	defer saveLock.RUnlock()

	return substitutionPattern.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])

		if value, found := saved[name]; found {
			return value
		}

		return match
	})
}
