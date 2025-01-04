package compiler

import (
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

type scope struct {
	module string
	depth  int
	usage  map[string]bool
}

type scopeStack []scope

func newScope(name string, line int) scope {
	return scope{
		module: name,
		depth:  line,
		usage:  make(map[string]bool),
	}
}

func (c *Compiler) PushScope() {
	module := c.activePackageName + "." + c.b.Name()
	if module == "." {
		module = ""
	} else if module[:1] == "." {
		module = module[1:]
	}

	c.scopes = append(c.scopes, newScope(module, c.blockDepth))
}

func (c *Compiler) PopScope() error {
	if !c.flags.unusedVars {
		return nil
	}

	if len(c.scopes) == 0 {
		return nil
	}

	names := []string{}

	scope := c.scopes[len(c.scopes)-1]
	for name, used := range scope.usage {
		if !used {
			names = append(names, name)
		}
	}

	if len(names) > 0 {
		sort.Strings(names)

		module := scope.module
		if module != "" {
			c.b.SetName(module)
			module = ", at " + module
		}

		return c.error(errors.ErrUnusedVariable, strings.Join(names, ", ")+module)
	}

	c.scopes = c.scopes[:len(c.scopes)-1]

	return nil
}

func (c *Compiler) CreateVariable(name string) *Compiler {
	if len(c.scopes) == 0 {
		c.PushScope()
	}

	c.scopes[len(c.scopes)-1].usage[name] = false

	return c
}

func (c *Compiler) UseVariable(name string) *Compiler {
	// Scan the scopes stack in reverse order and search for an entry for the
	// given variable. If found, mark it as used.
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if _, found := c.scopes[i].usage[name]; found {
			c.scopes[i].usage[name] = true

			break
		}
	}

	return c
}
