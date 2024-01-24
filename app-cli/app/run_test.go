// Package app provides the top-level framework for CLI execution. This includes
// the Run() method to run the program, plus a number of action routines that can
// be invoked from the grammar or by a user action routine.
package app

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/errors"
)

var testGrammar1 = []cli.Option{
	{
		LongName:    "stuff",
		OptionType:  cli.BooleanType,
		Description: "stuff mart",
	},
	{
		LongName:    "sub1",
		OptionType:  cli.Subcommand,
		Description: "sub1 subcommand",
		Action:      testAction0,
	},
	{
		LongName:    "sub2",
		OptionType:  cli.Subcommand,
		Description: "sub2 subcommand has options",
		Action:      testAction0,
		Value: []cli.Option{
			{
				ShortName:   "x",
				LongName:    "explode",
				Description: "Make something blow up",
				OptionType:  cli.StringType,
				Action:      testAction1,
			},
			{
				LongName:    "count",
				Description: "Count of things to blow up",
				OptionType:  cli.IntType,
				Action:      testAction2,
			},
		},
	},
}

func testAction0(c *cli.Context) error {
	return nil
}

func testAction1(c *cli.Context) error {
	v, _ := c.String("explode")
	fmt.Printf("Found the option value %s\n", v)

	if v != "bob" {
		return errors.Message("Invalid explode name: " + v)
	}

	return nil
}

func testAction2(c *cli.Context) error {
	v, _ := c.Integer("count")
	fmt.Printf("Found the option value %v\n", v)

	if v != 42 {
		return errors.Message("Invalid count: " + strconv.Itoa(v))
	}

	return nil
}

func TestRun(t *testing.T) {
	type args struct {
		grammar []cli.Option
		args    []string
		appName string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Invalid global option",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "--booboo"},
				"testing: the test app",
			},
			wantErr: true,
		},
		{
			name: "Valid global option",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "--stuff"},
				"testing: the test app",
			},
			wantErr: false,
		},
		{
			name: "Valid subcommand",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub1"},
				"testing: the test app",
			},
			wantErr: false,
		},
		{
			name: "Invalid subcommand",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "subzero"},
				"testing: the test app",
			},
			wantErr: true,
		},
		{
			name: "Valid subcommand with valid short option name",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "-x", "bob"},
				"testing: the test app",
			},
			wantErr: false,
		},
		{
			name: "Valid subcommand with valid long option",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "--explode", "bob"},
				"testing: the test app",
			},
			wantErr: false,
		},
		{
			name: "Valid subcommand with valid option but invalid value",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "-x", "bob2"},
				"testing: the test app",
			},
			wantErr: true,
		},
		{
			name: "Valid subcommand with valid option but missing value",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "-x"},
				"testing: the test app",
			},
			wantErr: true,
		},
		{
			name: "Valid subcommand with invalid int valid option ",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "--count", "42F"},
				"testing: the test app",
			},
			wantErr: true,
		},
		{
			name: "Valid subcommand with valid option ",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "--count", "42"},
				"testing: the test app",
			},
			wantErr: false,
		},
		{
			name: "Valid subcommand with valid option with wrong value",
			args: args{
				testGrammar1,
				[]string{"test-driver", "--log", "debug", "sub2", "--count", "43"},
				"testing: the test app",
			},
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New("tt.args.appName")
			app.SetCopyright("(c) 2020 Tom Cole. All rights reserved.")
			app.SetVersion(1, 1, 0)

			if err := app.Run(tt.args.grammar, tt.args.args); (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_loadEnvSettings(t *testing.T) {
	t.Run("load env", func(t *testing.T) {
		count := loadEnvSettings()
		if count != 0 {
			t.Errorf("loadEnvSettings() count = %v, want %v", count, 0)
		}

		os.Setenv("EGO_COMPILER_EXTENSIONS", "true")

		count = loadEnvSettings()
		if count != 1 {
			t.Errorf("loadEnvSettings() count = %v, want %v", count, 1)
		}

		v := settings.Get("ego.compiler.extensions")
		if v != "true" {
			t.Errorf("loadEnvSettings() setting = %v, want %v", v, "true")
		}

		os.Setenv("EGO_NOT_A_SETTING", "true")

		count = loadEnvSettings()
		if count != 1 {
			t.Errorf("loadEnvSettings() count = %v, want %v", count, 1)
		}
	})
}
