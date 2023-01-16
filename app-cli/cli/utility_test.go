package cli

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/defs"
)

func TestValidKeyword(t *testing.T) {
	type args struct {
		test  string
		valid []string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid",
			args: args{
				test:  "one",
				valid: []string{"zero", "one", "two"},
			},
			want: true,
		},
		{
			name: "invalid",
			args: args{
				test:  "one",
				valid: []string{"three", "four", "five"},
			},
			want: false,
		},
		{
			name: "empty",
			args: args{
				test:  "",
				valid: []string{"three", "four", "five"},
			},
			want: false,
		},
		{
			name: "no list",
			args: args{
				test:  "thing",
				valid: []string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validKeyword(tt.args.test, tt.args.valid); got != tt.want {
				t.Errorf("ValidKeyword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindKeyword(t *testing.T) {
	type args struct {
		test  string
		valid []string
	}

	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "valid1",
			args: args{
				test:  "one",
				valid: []string{"zero", "one", "two"},
			},
			want: 1,
		},
		{
			name: "valid2",
			args: args{
				test:  "two",
				valid: []string{"zero", "one", "two"},
			},
			want: 2,
		},
		{
			name: "invalid",
			args: args{
				test:  "one",
				valid: []string{"three", "four", "five"},
			},
			want: -1,
		},
		{
			name: "empty",
			args: args{
				test:  "",
				valid: []string{"three", "four", "five"},
			},
			want: -1,
		},
		{
			name: "no list",
			args: args{
				test:  "thing",
				valid: []string{},
			},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findKeyword(tt.args.test, tt.args.valid); got != tt.want {
				t.Errorf("FindKeyword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateBoolean(t *testing.T) {
	type args struct {
		value string
	}

	tests := []struct {
		name  string
		args  args
		want  bool
		want1 bool
	}{
		{
			name: "Valid1", args: args{value: "0"}, want: false, want1: true,
		},
		{
			name: "Valid2", args: args{value: "f"}, want: false, want1: true,
		},
		{
			name: "Valid3", args: args{value: defs.False}, want: false, want1: true,
		},
		{
			name: "Valid4", args: args{value: "n"}, want: false, want1: true,
		},
		{
			name: "Valid5", args: args{value: "no"}, want: false, want1: true,
		},
		{
			name: "Valid6", args: args{value: "1"}, want: true, want1: true,
		},
		{
			name: "Valid7", args: args{value: "T"}, want: true, want1: true,
		},
		{
			name: "Valid8", args: args{value: defs.True}, want: true, want1: true,
		},
		{
			name: "Valid9", args: args{value: "Y"}, want: true, want1: true,
		},
		{
			name: "Valid10", args: args{value: "yes"}, want: true, want1: true,
		},
		{
			name: "Invalid1", args: args{value: "yep"}, want: false, want1: false,
		},
		{
			name: "Invalid2", args: args{value: ""}, want: false, want1: false,
		},
		{
			name: "Invalid3", args: args{value: "3"}, want: false, want1: false,
		},
		{
			name: "Invalid4", args: args{value: "Nyet"}, want: false, want1: false,
		},
		{
			name: "Invalid5", args: args{value: "flase"}, want: false, want1: false,
		},
		{
			name: "Invalid6", args: args{value: "-1"}, want: false, want1: false,
		},
		{
			name: "Invalid7", args: args{value: "00"}, want: false, want1: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := validateBoolean(tt.args.value)
			if got != tt.want {
				t.Errorf("ValidateBoolean() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidateBoolean() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestMakeList(t *testing.T) {
	type args struct {
		value string
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "List1",
			args: args{
				value: "single",
			},
			want: []string{"single"},
		},
		{
			name: "List2",
			args: args{
				value: "first,second",
			},
			want: []string{"first", "second"},
		},
		{
			name: "List3",
			args: args{
				value: "several, items, here",
			},
			want: []string{"several", "items", "here"},
		},
		{
			name: "List4",
			args: args{
				value: "",
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeList(tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeList() = %v, want %v", got, tt.want)
			}
		})
	}
}
