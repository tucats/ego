package math

import (
	"reflect"
	"testing"
)

func TestFunctionMin(t *testing.T) {
	type args struct {
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: args{[]interface{}{5, 7, 2.0}},
			want: 2,
		},
		{
			name: "float64 list",
			args: args{[]interface{}{5.5, 5.1, "9.0"}},
			want: 5.1,
		},
		{
			name: "string list",
			args: args{[]interface{}{"dog", "cake", "pony"}},
			want: "cake",
		},
		{
			name: "bool list",
			args: args{[]interface{}{true, 33, false}},
			want: false,
		},
		{
			name:    "Invalid tyoe",
			args:    args{[]interface{}{map[string]interface{}{"age": 55}, 5}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    args{[]interface{}{15, []interface{}{5, 5}}},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := minimum(nil, tt.args.args)
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
	type args struct {
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: args{[]interface{}{5, 7, 2.0}},
			want: 7,
		},
		{
			name: "float64 list",
			args: args{[]interface{}{5.5, 5.1, "9.0"}},
			want: 9.0,
		},
		{
			name: "string list",
			args: args{[]interface{}{"dog", "cake", "pony"}},
			want: "pony",
		},
		{
			name: "bool list",
			args: args{[]interface{}{true, 33, false}},
			want: true,
		},
		{
			name:    "Invalid type",
			args:    args{[]interface{}{map[string]interface{}{"age": 55}, 5}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    args{[]interface{}{15, []interface{}{5, 5}}},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := maximum(nil, tt.args.args)
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
	type args struct {
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer list",
			args: args{[]interface{}{5, 7, 2.0}},
			want: 14,
		},
		{
			name: "float64 list",
			args: args{[]interface{}{5.5, 5.1, "9.0"}},
			want: 19.6,
		},
		{
			name: "string list",
			args: args{[]interface{}{"dog", "cake", 55}},
			want: "dogcake55",
		},
		{
			name: "bool list",
			args: args{[]interface{}{true, 33, false}},
			want: true,
		},
		{
			name:    "Invalid type",
			args:    args{[]interface{}{map[string]interface{}{"age": 55}, 5}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid int",
			args:    args{[]interface{}{15, []interface{}{5, 5}}},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sum(nil, tt.args.args)
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
