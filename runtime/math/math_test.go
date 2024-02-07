package math

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionMin(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: data.NewList(5, 7, 2.0),
			want: 2,
		},
		{
			name: "float64 list",
			args: data.NewList(5.5, 5.1, "9.0"),
			want: 5.1,
		},
		{
			name: "string list",
			args: data.NewList("dog", "cake", "pony"),
			want: "cake",
		},
		{
			name: "bool list",
			args: data.NewList(true, 33, false),
			want: false,
		},
		{
			name:    "Invalid tyoe",
			args:    data.NewList(map[string]interface{}{"age": 55}, 5),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    data.NewList(15, []interface{}{5, 5}),
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := minimum(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionMin() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionMin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionMax(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: data.NewList(5, 7, 2.0),
			want: 7,
		},
		{
			name: "float64 list",
			args: data.NewList(5.5, 5.1, "9.0"),
			want: 9.0,
		},
		{
			name: "string list",
			args: data.NewList("dog", "cake", "pony"),
			want: "pony",
		},
		{
			name: "bool list",
			args: data.NewList(true, 33, false),
			want: true,
		},
		{
			name:    "Invalid type",
			args:    data.NewList(map[string]interface{}{"age": 55}, 5),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    data.NewList(15, []interface{}{5, 5}),
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := maximum(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionMax() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionMax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionSum(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: data.NewList(5, 7, 2.0),
			want: 14,
		},
		{
			name: "float64 list",
			args: data.NewList(5.5, 5.1, "9.0"),
			want: 19.6,
		},
		{
			name: "string list",
			args: data.NewList("dog", "cake", 55),
			want: "dogcake55",
		},
		{
			name: "bool list",
			args: data.NewList(true, 33, false),
			want: true,
		},
		{
			name:    "Invalid type",
			args:    data.NewList(map[string]interface{}{"age": 55}, 5),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    data.NewList(15, []interface{}{5, 5}),
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sum(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionSum() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionSum() = %v, want %v", got, tt.want)
			}
		})
	}
}
