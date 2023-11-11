package data

import (
	"reflect"
	"testing"
)

func TestNewList(t *testing.T) {
	type args struct {
		items []interface{}
	}

	tests := []struct {
		name string
		args args
		want List
	}{
		{
			name: "test with int values",
			args: args{
				items: []interface{}{1, 2, 3},
			},
			want: List{elements: []interface{}{1, 2, 3}},
		},
		{
			name: "test with string values",
			args: args{
				items: []interface{}{"a", "b", "c"},
			},
			want: List{elements: []interface{}{"a", "b", "c"}},
		},
		{
			name: "test with mixed values",
			args: args{
				items: []interface{}{1, "two", 3.0},
			},
			want: List{elements: []interface{}{1, "two", 3.0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewList(tt.args.items...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewList() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestList_Len(t *testing.T) {
	tests := []struct {
		name string
		l    List
		want int
	}{
		{
			name: "test with empty list",
			l:    List{},
			want: 0,
		},
		{
			name: "test with non-empty list",
			l:    List{elements: []interface{}{1, 2, 3}},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Len(); got != tt.want {
				t.Errorf("List.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestList_Get(t *testing.T) {
	tests := []struct {
		name string
		l    List
		n    int
		want interface{}
	}{
		{
			name: "test with valid index",
			l:    List{elements: []interface{}{1, 2, 3}},
			n:    1,
			want: 2,
		},
		{
			name: "test with invalid index",
			l:    List{elements: []interface{}{1, 2, 3}},
			n:    3,
			want: nil,
		},
		{
			name: "test with negative index",
			l:    List{elements: []interface{}{1, 2, 3}},
			n:    -1,
			want: nil,
		},
		{
			name: "test with empty list",
			l:    List{},
			n:    0,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Get(tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestList_Slice(t *testing.T) {
	tests := []struct {
		name  string
		l     List
		begin int
		end   int
		want  List
	}{
		{
			name:  "test with valid range",
			l:     List{elements: []interface{}{1, 2, 3, 4, 5}},
			begin: 1,
			end:   4,
			want:  List{elements: []interface{}{2, 3, 4}},
		},
		{
			name:  "test with invalid range",
			l:     List{elements: []interface{}{1, 2, 3, 4, 5}},
			begin: -1,
			end:   6,
			want:  List{elements: nil},
		},
		{
			name:  "test with empty list",
			l:     List{},
			begin: 0,
			end:   0,
			want:  List{elements: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Slice(tt.begin, tt.end); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List.Slice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestList_Append(t *testing.T) {
	tests := []struct {
		name     string
		l        List
		elements []interface{}
		want     int
	}{
		{
			name:     "test with empty list",
			l:        List{},
			elements: []interface{}{1, 2, 3},
			want:     3,
		},
		{
			name:     "test with non-empty list",
			l:        List{elements: []interface{}{1, 2, 3}},
			elements: []interface{}{"a", "b", "c"},
			want:     6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Append(tt.elements...); got != tt.want {
				t.Errorf("List.Append() = %v, want %v", got, tt.want)
			}
		})
	}
}
