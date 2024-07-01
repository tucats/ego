package strconv

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestDoFormatbool_NilArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    string
		wantErr bool
	}{
		{
			name:    "empty argument list",
			args:    data.NewList(),
			want:    "false",
			wantErr: false,
		},
		{
			name:    "argument list with one nil value",
			args:    data.NewList(nil),
			want:    "false",
			wantErr: false,
		},
		{
			name:    "argument list with one non-nil value",
			args:    data.NewList(false),
			want:    "false",
			wantErr: false,
		},
		{
			name:    "argument list with multiple values",
			args:    data.NewList(false, true, nil),
			want:    "false",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &symbols.SymbolTable{}
			got, err := doFormatbool(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("doFormatbool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("doFormatbool() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoFormatfloat(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    string
		wantErr bool
	}{
		{
			name:    "Format float with format 'f'",
			args:    data.NewList(3.14159, "f", 2, 64),
			want:    "3.14",
			wantErr: false,
		},
		{
			name:    "Format float with invalid format",
			args:    data.NewList(3.14159, "z", 4, 64),
			want:    "",
			wantErr: true,
		},
		{
			name:    "Format float with invalid bit size",
			args:    data.NewList(3.14159, "g", 5, 128),
			want:    "",
			wantErr: true,
		},
		{
			name:    "Format float with default precision",
			args:    data.NewList(3.14159, "e", -1, 64),
			want:    "3.14159e+00",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &symbols.SymbolTable{}
			got, err := doFormatfloat(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("doFormatfloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && got.(string) != tt.want {
				t.Errorf("doFormatfloat() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoFormatint_OutOfRangeBase(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		wantErr bool
	}{
		{
			name:    "base 1",
			args:    data.NewList(10, 1),
			wantErr: true,
		},
		{
			name:    "base 0",
			args:    data.NewList(10, 0),
			wantErr: true,
		},
		{
			name:    "base 37",
			args:    data.NewList(10, 37),
			wantErr: true,
		},
		{
			name:    "base -2",
			args:    data.NewList(10, -2),
			wantErr: true,
		},
		{
			name:    "base 2",
			args:    data.NewList(10, 2),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &symbols.SymbolTable{}
			_, err := doFormatint(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("doFormatint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
