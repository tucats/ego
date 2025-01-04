package compiler

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

type scope struct {
	module string
	depth  int
	usage  map[string]*errors.Error
}

type scopeStack []scope

// Flag used to turn on logging for symbol tracking, used during development debugging.
var symbolUsageDebugging = false

func newScope(name string, line int) scope {
	return scope{
		module: name,
		depth:  line,
		usage:  make(map[string]*errors.Error),
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
	var err *errors.Error

	if !c.flags.unusedVars {
		return nil
	}

	pos := len(c.scopes) - 1
	if pos < 0 {
		return nil
	}

	scope := c.scopes[pos]
	for name, usageError := range scope.usage {
		if usageError != nil {
			if symbolUsageDebugging {
				ui.Log(ui.InternalLogger, "Usage error     %s, %v", name, usageError)
			}

			err = errors.Chain(err, usageError)
		}
	}

	c.scopes = c.scopes[:pos]

	if err == nil {
		return nil
	}

	return err
}

func (c *Compiler) CreateVariable(name string) *Compiler {
	if len(c.scopes) == 0 {
		c.PushScope()
	}

	pos := len(c.scopes) - 1
	if _, found := c.scopes[pos].usage[name]; !found {
		err := c.error(errors.ErrUnusedVariable).Context(name)
		c.scopes[pos].usage[name] = err

		if symbolUsageDebugging {
			ui.Log(ui.InternalLogger, "Create variable %s, %v", name, err)
		}
	} else if symbolUsageDebugging {
		ui.Log(ui.InternalLogger, "Write  variable %s", name)
	}

	return c
}

func (c *Compiler) UseVariable(name string) *Compiler {
	// Scan the scopes stack in reverse order and search for an entry for the
	// given variable. If found, mark it as used.
	if len(c.scopes) == 0 {
		return c
	}

	pos := len(c.scopes) - 1

	for i := pos; i >= 0; i-- {
		if _, found := c.scopes[i].usage[name]; found {
			c.scopes[i].usage[name] = nil

			if symbolUsageDebugging {
				ui.Log(ui.InternalLogger, "Use    variable %s", name)
			}

			break
		}
	}

	return c
}
