package json

import (
	"testing"

	"github.com/tucats/ego/data"
)

func TestMarshal_Elements(t *testing.T) {
	tests := []struct {
		name     string
		input    data.List
		wantJSON string
		wantErr  bool
	}{
		{
			name:     "string element",
			input:    data.NewList("hello"),
			wantJSON: `"hello"`,
			wantErr:  false,
		},
		{
			name:     "integer element",
			input:    data.NewList(123),
			wantJSON: `123`,
			wantErr:  false,
		},
		{
			name:     "float element",
			input:    data.NewList(3.14),
			wantJSON: `3.14`,
			wantErr:  false,
		},
		{
			name:     "boolean element",
			input:    data.NewList(true),
			wantJSON: `true`,
			wantErr:  false,
		},
		{
			name:     "nil element",
			input:    data.NewList(nil),
			wantJSON: `null`,
			wantErr:  false,
		},
		{
			name:     "array of elements",
			input:    data.NewList(true, 3.14, "Tom"),
			wantJSON: `[true, 3.14, "Tom"]`,
			wantErr:  false,
		},
		{
			name: "struct of elements",
			input: data.NewList(data.NewStructFromMap(map[string]interface{}{
				"name": "Tom",
				"age":  55,
			})),
			wantJSON: `{"age":55,"name":"Tom"}`,
			wantErr:  false,
		},
		{
			name: "nested struct of elements",
			input: data.NewList(data.NewStructFromMap(map[string]interface{}{
				"name": data.NewStructFromMap(map[string]interface{}{
					"first": "Tom",
					"last":  "Smith",
				}).SetFieldOrder([]string{"first", "last"}),
				"age": 55,
			})),
			wantJSON: `{"age":55,"name":{"first":"Tom","last":"Smith"}}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := marshal(nil, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			jsonStr := string(got.(data.List).Get(0).(*data.Array).GetBytes())
			if jsonStr != tt.wantJSON {
				t.Errorf("marshal() = %v, want %v", jsonStr, tt.wantJSON)
			}
		})
	}
}
