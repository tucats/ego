package expressions

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/tokenizer"
)

func TestExpression_Parse(t *testing.T) {
	type fields struct {
		Source string
		Tokens []string
		TokenP int
	}
	tests := []struct {
		name       string
		fields     fields
		wantErr    bool
		wantTokens []string
	}{
		{
			name: "Simple test",
			fields: fields{
				Source: "a = b",
			},
			wantErr:    false,
			wantTokens: []string{"a", "=", "b"},
		},
		{
			name: "Quote test",
			fields: fields{
				Source: "a = \"Tom\"",
			},
			wantErr:    false,
			wantTokens: []string{"a", "=", "\"Tom\""},
		},
		{
			name: "Compound operators",
			fields: fields{
				Source: "a >= \"Tom\"",
			},
			wantErr:    false,
			wantTokens: []string{"a", ">=", "\"Tom\""},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Expression{}
			e.t = tokenizer.New("")
			e.t.Tokens = tt.fields.Tokens
			e.t.TokenP = tt.fields.TokenP

			if err := e.Parse(tt.fields.Source); (err != nil) != tt.wantErr {
				t.Errorf("Expression.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(e.t.Tokens, tt.wantTokens) {
				t.Errorf("Expression.Parse() got %v, want %v", e.t.Tokens, tt.wantTokens)
			}
		})
	}
}
