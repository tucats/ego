package rest

import (
	"reflect"
	"testing"
)

func TestURLs(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want string
	}{
		{
			name: "localhost:8080/tables/%s/rows",
			args: []interface{}{"mytable"},
			want: "localhost:8080/tables/mytable/rows",
		},
		{
			name: "localhost:8080/tables/%s/rows",
			args: []interface{}{"mytable", "count=3"},
			want: "localhost:8080/tables/mytable/rows?count%3D3",
		},
		{
			name: "localhost:8080/tables/%s/rows",
			args: []interface{}{"mytable", "count=3", "nosort"},
			want: "localhost:8080/tables/mytable/rows?count%3D3&nosort",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := URLBuilder()
			u.Path(tt.name, tt.args...)

			if got := u.String(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("URL.WritePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestURL_WriteParameter(t *testing.T) {
	tests := []struct {
		base   string
		name   string
		fields []interface{}
		want   string
	}{
		{
			base:   "localhost:8080/tables",
			name:   "nosort",
			fields: []interface{}{},
			want:   "localhost:8080/tables?nosort",
		},
		{
			base:   "localhost:8080/tables",
			name:   "count",
			fields: []interface{}{"3"},
			want:   "localhost:8080/tables?count=3",
		},
		{
			base:   "localhost:8080/tables",
			name:   "columns",
			fields: []interface{}{"age", "name"},
			want:   "localhost:8080/tables?columns=age,name",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := URLBuilder(tt.base)
			u.Parameter(tt.name, tt.fields...)
		})
	}
}
