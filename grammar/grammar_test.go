package grammar

import (
	"testing"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/errors"
)

func Test_validateGrammar(t *testing.T) {
	var tests = []struct {
		name    string
		grammar []cli.Option
		wantErr bool
	}{
		{
			name:    "verb-subject grammar",
			grammar: EgoGrammar2,
			wantErr: false,
		},
		{
			name:    "rpn grammar",
			grammar: EgoGrammar,
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
