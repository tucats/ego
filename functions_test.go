package main

import (
	"reflect"
	"testing"

	"github.com/tucats/gopackages/symbols"
)

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
	client, err := FunctionGremlinOpen(syms, []interface{}{"ws://localhost:8182/gremlin"})
	if err != nil {
		t.Errorf("Error connecting to gremlin server: %v", err)
	}

	syms.SetAlways("_this", client)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FunctionGremlinQuery(syms, []interface{}{
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
