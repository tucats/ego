package runtime

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/io"
	"github.com/tucats/ego/symbols"
)

/*  TEST ONLY IF YOU HAVE A LOCAL GREMLIN SERVER

func TestFunctionGremlinQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    interface{}
		wantErr bool
	}{
		{
			name:    "Simple vertex count",
			query:   "g.V().hasLabel('MADEUPNAME').count()",
			want:    0,
			wantErr: false,
		},
		{
			name:    "create vertex",
			query:   "g.addV('airport').property('name', 'Raleigh').property('code', 'RDU')",
			wantErr: false,
		},
		{
			name:  "value map",
			query: "g.V().hasLabel('airport').valueMap()",
			want: map[string]interface{}{
				"code": "RDU",
				"name": "Raleigh",
			},
			wantErr: false,
		},
		{
			name:    "cleanup vertex",
			query:   "g.V().hasLabel('airport').drop()",
			wantErr: false,
		},
	}

	syms := symbols.NewSymbolTable("test")
	client, err := GremlinOpen(syms, []interface{}{"ws://localhost:8182/gremlin"})
	if err != nil {
		t.Errorf("Error connecting to gremlin server: %v", err)
		t.Fail()
	}

	_ = syms.SetAlways("__this", client)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GremlinQuery(syms, []interface{}{
				tt.query,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionGremlinQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionGremlinQuery() = %#v, want %v", got, tt.want)
			}
		})
	}
}
*/

func Test_pad(t *testing.T) {
	type args struct {
		s string
		w int
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "pad left",
			args: args{s: "tom", w: 5},
			want: "tom  ",
		},
		{
			name: "pad right",
			args: args{s: "tom", w: -5},
			want: "  tom",
		},
		{
			name: "empty",
			args: args{s: "", w: 5},
			want: "     ",
		},
		{
			name: "truncate",
			args: args{s: "ezekiel", w: 5},
			want: "ezeki",
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := io.Pad(tt.args.s, tt.args.w); got != tt.want {
				t.Errorf("pad() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionTable(t *testing.T) {
	type args struct {
		symbols *symbols.SymbolTable
		args    []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "single string column and row",
			args: args{
				symbols: nil,
				args: []interface{}{
					[]interface{}{
						map[string]interface{}{
							"name": "tom",
						},
					},
				},
			},
			want: []interface{}{
				"name ",
				"==== ",
				"tom  ",
			},
			wantErr: false,
		},
		{
			name: "two string columns and row",
			args: args{
				symbols: nil,
				args: []interface{}{
					[]interface{}{
						map[string]interface{}{
							"name": "tom",
							"id":   "123456",
						},
					},
				},
			},
			want: []interface{}{
				"id     name ",
				"====== ==== ",
				"123456 tom  ",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Table(tt.args.symbols, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionTable() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionTable() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
