// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages. It also includes functions for controlling
// whether those messages are displayed or not.
package ui

import (
	"os"
	"testing"
)

func TestLogMessage(t *testing.T) {
	type args struct {
		class  string
		format string
		args   A
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple message",
			args: args{
				class:  "USER",
				format: "string text",
			},
			want: "     USER   : string text",
		},
		{
			name: "parameterized message",
			args: args{
				class:  "USER",
				format: "digits {{d}}",
				args:   A{"d": 42},
			},
			want: "     USER   : digits 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Tests assume TextFormat is the default output format
			LogFormat = TextFormat

			os.Setenv("EGO_LOG_FORMAT", "text")

			logger := LoggerByName(tt.args.class)
			got := formatLogMessage(logger, tt.args.format, tt.args.args)
			// Mask out the parts that are variable and un-testable, which
			// includes the current date/time and a sequence number
			got = got[23:]
			if got != tt.want {
				t.Errorf("LogMessage() got\n %v\nwant\n %v", got, tt.want)
			}
		})
	}
}
