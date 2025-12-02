package jaxon

import "testing"

func TestGetItem(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		item    string
		want    string
		wantErr bool
	}{
		{
			name: "escaped nested array of maps member",
			text: `[ { "user.name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "0.user\\.name",
			want: "Alice",
		},
		{
			name: "escaped nested array of maps member with bracket notation",
			text: `[ { "user.name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "[0]user\\.name",
			want: "Alice",
		},
		{
			name: "escaped nested map of maps member",
			text: `{ "first.one": { "user.name": "Alice", "age": 30 }, "second": { "name": "Bob", "age": 25 } }`,
			item: "first\\.one.user\\.name",
			want: "Alice",
		},
		{
			name: "nested array of maps member",
			text: `[ { "name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "0.name",
			want: "Alice",
		},
		{
			name: "nested array of maps member with bracket notation",
			text: `[ { "name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "[0].name",
			want: "Alice",
		},
		{
			name: "integer array member",
			text: `[ 1, 2, 3]`,
			item: "2",
			want: "3",
		},
		{
			name: "integer array member with bracket notation",
			text: `[ 1, 2, 3]`,
			item: "[2]",
			want: "3",
		},
		{
			name: "bool array member",
			text: `[ true, false]`,
			item: "0",
			want: "true",
		},
		{
			name: "nested integer map member",
			text: `{ "person": { "age": 43}}`,
			item: "person.age",
			want: "43",
		},
		{
			name: "integer map member",
			text: `{ "age": 42}`,
			item: "age",
			want: "42",
		},
		{
			name: "string map member",
			text: `{ "color": "brown"}`,
			item: "color",
			want: "brown",
		},
		{
			name: "bool map member",
			text: `{ "open": true}`,
			item: "open",
			want: "true",
		},
		{
			name:    "integer map member not found",
			text:    `{ "age": 42}`,
			item:    "ages",
			want:    "",
			wantErr: true,
		},
		{
			name: "simple integer",
			text: `42`,
			item: ".",
			want: "42",
		},
		{
			name: "simple string",
			text: `"brown"`,
			item: ".",
			want: "brown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetItem(tt.text, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetItem() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("GetItem() = %v, want %v", got, tt.want)
			}
		})
	}
}
