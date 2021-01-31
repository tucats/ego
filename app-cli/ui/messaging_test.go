// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages. It also includes functions for controlling
// whether those messages are displayed or not.
package ui

import "testing"

func TestLogMessage(t *testing.T) {
	type args struct {
		class  string
		format string
		args   []interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple message",
			args: args{
				class:  "TEST",
				format: "string text",
				args:   []interface{}{},
			},
			want: "TEST   : string text",
		},
		{
			name: "parameterized message",
			args: args{
				class:  "TEST",
				format: "digits %d",
				args:   []interface{}{42},
			},
			want: "TEST   : digits 42",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LogMessage(tt.args.class, tt.args.format, tt.args.args...)
			// Mask out the parts that are variable and un-testable, which
			// includes the current date/time and a sequence number
			got = got[23:]
			if got != tt.want {
				t.Errorf("LogMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
