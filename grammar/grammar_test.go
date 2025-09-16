package grammar

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/errors"
)

// Test to verify that all the actions referenced in the traditional RPM grammar area also
// available in the verb-subject grammar.
func Test_grammarMissingActions(t *testing.T) {
	t.Run("missing in action", func(t *testing.T) {
		// Make a map for each named action in the traditional grammar.
		a1 := map[string]int{}

		actionScanner(ClassActionGrammar, a1)

		// Do the same for the verb-subject grammar
		a2 := map[string]int{}

		actionScanner(VerbSubjectGrammar, a2)

		// Check that all actions in the traditional grammar have been
		// captured in the verb-subject grammar.
		for key, count1 := range a1 {
			if count1 > 0 {
				if count2, ok := a2[key]; !ok || count2 == 0 {
					t.Errorf("missing action in verb-subject grammar: %s", key)
				}
			}
		}

		// Check that all actions in the traditional grammar have been
		// captured in the verb-subject grammar. Note that some actions are
		// unique to the subject-verb grammar, so there is an exclude list
		// here we honor as well.
		var excludeActions = map[string]bool{
			"commands.TableRevoke":                true,
			"app-cli/config.SetAction":            true,
			"commands.LoggingFile":                true,
			"commands.LoggingStatus":              true,
			"app-cli/app.VersionAction":           true,
			"app-cli/config.ListAction":           true,
			"app-cli/config.ShowAction":           true,
			"app-cli/config.DescribeAction":       true,
			"app-cli/config.DeleteProfileAction":  true,
			"app-cli/config.DeleteAction":         true,
			"app-cli/config.SetDescriptionAction": true,
			"app-cli/config.SetOutputAction":      true,
		}

		for key, count2 := range a2 {
			if count2 > 0 {
				if count1, ok := a1[key]; !ok || count1 == 0 {
					if !excludeActions[key] {
						t.Errorf("missing action in RPN grammar: %s", key)
					}
				}
			}
		}
	})
}

func actionScanner(g []cli.Option, actions map[string]int) {
	// Scan over the traditional grammar and capture all the actions.
	// For each action, use the reflection package to get the function
	// name to use as the key string. When an action is found, increment
	// the map item to show that it has been referenced.
	for _, option := range g {
		if option.Action != nil {
			vv := reflect.ValueOf(option.Action)

			// If it's an internal function, show it's name. If it is a standard builtin from the
			// function library, show the short form of the name.
			if vv.Kind() == reflect.Func {
				fn := runtime.FuncForPC(reflect.ValueOf(option.Action).Pointer())
				name := fn.Name()
				name = strings.TrimPrefix(name, "github.com/tucats/ego/")

				if _, ok := actions[name]; !ok {
					actions[name] = 1
				} else {
					actions[name]++
				}
			}
		}

		if option.Value != nil {
			if subgrammar, ok := option.Value.([]cli.Option); ok {
				actionScanner(subgrammar, actions)
			}
		}
	}
}

// Search the grammars to see if there are any option trees that do not include. an action at the
// end of the tree.
func Test_validateGrammar(t *testing.T) {
	var tests = []struct {
		name    string
		grammar []cli.Option
		wantErr bool
	}{
		{
			name:    "verb-subject grammar",
			grammar: VerbSubjectGrammar,

			wantErr: false,
		},
		{
			name:    "rpn grammar",
			grammar: ClassActionGrammar,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGrammar(tt.grammar, "", false)

			if !errors.Nil(err) != tt.wantErr {
				t.Errorf("validateGrammar(%s) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// Recursively scan through the grammar, looking for nodes that are
// of type cli.SubCommand but have no action or sub-grammar.
func validateGrammar(g []cli.Option, text string, mustHaveAction bool) error {
	var (
		err       *errors.Error
		hasAction bool
	)

	for _, entry := range g {
		if entry.OptionType != cli.Subcommand {
			continue
		}

		if entry.Action != nil {
			//name := text + " " + entry.LongName
			//fmt.Println(name + " has an action")
			hasAction = true

			continue
		}

		if entry.Value == nil {
			name := entry.LongName

			e := errors.Message(text + " " + name + " has no action")
			if errors.Nil(err) {
				err = errors.New(e)
			} else {
				err = errors.Chain(err, e)
			}

			continue
		}

		name := text + " " + entry.LongName

		e := validateGrammar(entry.Value.([]cli.Option), name, true)
		if !errors.Nil(e) {
			if errors.Nil(err) {
				err = errors.New(e)
			} else {
				err = errors.Chain(err, errors.New(e))
			}
		}
	}

	if mustHaveAction && !hasAction {
		e := errors.Message(text + " has no action")
		if errors.Nil(err) {
			err = errors.New(e)
		} else {
			err = errors.Chain(err, e)
		}
	}

	return err
}
