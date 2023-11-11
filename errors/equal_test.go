package errors

import (
	"errors"
	"testing"
)

func TestEqual(t *testing.T) {
	type args struct {
		err1 error
		err2 error
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "both errors are nil",
			args: args{
				err1: nil,
				err2: nil,
			},
			want: true,
		},
		{
			name: "one error is nil",
			args: args{
				err1: errors.New("test error"),
				err2: nil,
			},
			want: false,
		},
		{
			name: "both errors are not nil and have the same message",
			args: args{
				err1: errors.New("test error"),
				err2: errors.New("test error"),
			},
			want: true,
		},
		{
			name: "both errors are not nil and have different messages",
			args: args{
				err1: errors.New("test error 1"),
				err2: errors.New("test error 2"),
			},
			want: false,
		},
		{
			name: "both errors are not nil and have the same message but one is wrapped in an Error struct",
			args: args{
				err1: &Error{err: errors.New("test error")},
				err2: errors.New("test error"),
			},
			want: true,
		},
		{
			name: "both errors are not nil and have different messages and one is wrapped in an Error struct",
			args: args{
				err1: &Error{err: errors.New("test error 1")},
				err2: errors.New("test error 2"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.args.err1, tt.args.err2); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
