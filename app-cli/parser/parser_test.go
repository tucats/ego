package parser

import (
	"encoding/json"
	"reflect"
	"testing"
)

func Test_parse(t *testing.T) {
	type object1 struct {
		Field1 string
	}

	tests := []struct {
		name    string
		item    string
		query   string
		want    []any
		wantErr bool
	}{
		{
			name: "all array index with bracket notation",
			item: `
			{
			   "items": [
			 		{"name": "John", "age": 30},
			   		{"name": "Jane", "age": 25}
				]
			}`,
			query: ".items[*].name[1]",
			want:  []any{"Jane"},
		},
		{
			name: "all array index",
			item: `
			{
			   "items": [
			 		{"name": "John", "age": 30},
			   		{"name": "Jane", "age": 25}
				]
			}`,
			query: ".items.*.name.1",
			want:  []any{"Jane"},
		},
		{
			name:  "single integer",
			item:  `22`,
			query: ".",
			want:  []any{22.0},
		},
		{
			name:  "single float",
			item:  `3.14`,
			query: ".",
			want:  []any{3.14},
		},
		{
			name:  "single float representation of integer value",
			item:  `42.0`,
			query: ".",
			want:  []any{42.0},
		},
		{
			name:  "object field name",
			item:  `{ "name": "John Doe", "age": 30 }`,
			query: "name",
			want:  []any{"John Doe"},
		},
		{
			name:  "object field age",
			item:  `{ "name": "John Doe", "age": 30 }`,
			query: "age",
			want:  []any{30.0},
		},
		{
			name:  "nested object field",
			item:  `{ "person": { "name": "John Doe", "age": 30 } }`,
			query: "person.name",
			want:  []any{"John Doe"},
		},
		{
			name:  "array index",
			item:  `[1, 2, 3, 4, 5]`,
			query: "2",
			want:  []any{3.0},
		},
		{
			name: "array of elements",
			item: `
			[
				{"Field1": "one"},
				{"Field1": "two"},
                {"Field1": "three"}
			]`,
			query: "*.Field1",
			want:  []any{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b any

			err := json.Unmarshal([]byte(tt.item), &b)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
			}

			got, err := parse(b, tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
