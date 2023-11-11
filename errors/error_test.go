package errors

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	type fields struct {
		err      error
		location *location
		context  string
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test with nil error",
			fields: fields{
				err:      nil,
				location: nil,
				context:  "",
			},
			want: "",
		},
		{
			name: "test with nil error and location",
			fields: fields{
				err:      nil,
				location: &location{name: "test", line: 1, column: 1},
				context:  "",
			},
			want: "",
		},
		{
			name: "test with error and location",
			fields: fields{
				err:      errors.New("test error"),
				location: &location{name: "test", line: 1, column: 1},
				context:  "",
			},
			want: "at test(line 1:1), test error",
		},
		{
			name: "test with error, location, and context",
			fields: fields{
				err:      errors.New("test error"),
				location: &location{name: "test", line: 1, column: 1},
				context:  "test context",
			},
			want: "at test(line 1:1), test error: test context",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Error{
				err:      tt.fields.err,
				location: tt.fields.location,
				context:  tt.fields.context,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error.Error(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
