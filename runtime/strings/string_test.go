package strings

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionLeft(t *testing.T) {
	type args struct {
		args data.List
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple test",
			args: args{data.NewList("Abraham", 4)},
			want: "Abra",
		},
		{
			name: "negative length test",
			args: args{data.NewList("Abraham", -5)},
			want: "",
		},
		{
			name: "length too long test",
			args: args{data.NewList("Abraham", 50)},
			want: "Abraham",
		},
		{
			name: "unicode string",
			args: args{data.NewList("\u2318foo\u2318", 3)},
			want: "\u2318fo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := leftSubstring(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionLeft() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionLeft() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionRight(t *testing.T) {
	type args struct {
		args data.List
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple test",
			args: args{data.NewList("Abraham", 3)},
			want: "ham",
		},
		{
			name: "length too small test",
			args: args{data.NewList("Abraham", -5)},
			want: "",
		},
		{
			name: "length too long test",
			args: args{data.NewList("Abraham", 103)},
			want: "Abraham",
		},
		{
			name: "empty string test",
			args: args{data.NewList("", 3)},
			want: "",
		},
		{
			name: "unicode string",
			args: args{data.NewList("\u2318foo\u2318", 3)},
			want: "oo\u2318",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rightSubstring(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionRight() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionRight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubstring(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "left case",
			args: data.NewList("simple", 1, 3),
			want: "sim",
		},
		{
			name: "right case",
			args: data.NewList("simple", 3, 4),
			want: "mple",
		},
		{
			name: "middle case",
			args: data.NewList("simple", 3, 1),
			want: "m",
		},
		{
			name: "invalid start case",
			args: data.NewList("simple", -5, 3),
			want: "sim",
		},
		{
			name: "invalid len case",
			args: data.NewList("simple", 1, 355),
			want: "simple",
		},
		{
			name: "simple ASCII string starting at 1",
			args: data.NewList("foobar", 1, 3),
			want: "foo",
		},
		{
			name: "simple ASCII string starting at 3",
			args: data.NewList("foobar", 3, 2),
			want: "ob",
		},
		{
			name: "simple ASCII string with zero len",
			args: data.NewList("foobar", 3, 0),
			want: "",
		},
		{
			name: "simple ASCII string with len too big",
			args: data.NewList("foobar", 3, 10),
			want: "obar",
		},
		{
			name: "Unicode string with zero len",
			args: data.NewList("\u2318foo\u2318", 3, 0),
			want: "",
		},
		{
			name: "Unicode string starting at 1",
			args: data.NewList("\u2318foo\u2318", 1, 3),
			want: "\u2318fo",
		},
		{
			name: "Unicode string starting at 2",
			args: data.NewList("\u2318foo\u2318", 2, 3),
			want: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substring(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Substring() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Substring() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStrLen(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "length of ASCII string",
			args: data.NewList("foo"),
			want: 3,
		},
		{
			name: "length of empty string",
			args: data.NewList(""),
			want: 0,
		},
		{
			name: "length of Unicode string",
			args: data.NewList("\u2318Foo\u2318"),
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := length(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("StrLen() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StrLen() = %v, want %v", got, tt.want)
			}
		})
	}
}
