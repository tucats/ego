package strconv

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestDoAtoi(t *testing.T) {
	tests := []struct {
		name     string
		input    data.List
		want     interface{}
		wantErr  bool
		wantMsg  string
		wantData []interface{}
	}{
		{
			name:     "valid integer",
			input:    data.NewList("123"),
			want:     data.NewList(123, nil),
			wantErr:  false,
			wantMsg:  "",
			wantData: []interface{}{123},
		},
		{
			name:  "invalid integer",
			input: data.NewList("abc"),
			want: data.NewList(nil, errors.Message(
				"strconv.Atoi: parsing \"abc\": invalid syntax")),
			wantErr:  true,
			wantMsg:  "in Atoi, strconv.Atoi: parsing \"abc\": invalid syntax",
			wantData: []interface{}{},
		},
		{
			name:     "negative integer",
			input:    data.NewList("-456"),
			want:     data.NewList(-456, nil),
			wantErr:  false,
			wantMsg:  "",
			wantData: []interface{}{-456},
		},
		{
			name:     "large integer",
			input:    data.NewList("9223372036854775807"),
			want:     data.NewList(9223372036854775807, nil),
			wantErr:  false,
			wantMsg:  "",
			wantData: []interface{}{9223372036854775807},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("testing")
			got, err := doAtoi(s, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("doAtoi() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if err.Error() != tt.wantMsg {
					t.Errorf("doAtoi() error message = %v, wantMsg %v", err, tt.wantMsg)
				}
			}

			if r, ok := got.(data.List); ok {
				var v2, w2 *errors.Error

				v1 := r.Get(0)

				if r.Get(1) != nil {
					v2 = r.Get(1).(*errors.Error)
				}

				w1 := tt.want.(data.List).Get(0)
				if tt.want.(data.List).Get(1) != nil {
					w2 = tt.want.(data.List).Get(1).(*errors.Error)
				}

				if !reflect.DeepEqual(v1, w1) {
					t.Errorf("doAtoi() got = %v, want %v", got, tt.want)
				}

				if !errors.Equal(v2, w2) {
					t.Errorf("doAtoi() got = %v; want %v", v2, w2)
				}
			}

			if err == nil {
				list := got.(data.List)
				for i, want := range tt.wantData {
					if !reflect.DeepEqual(list.Get(i), want) {
						t.Errorf("doAtoi() data at index %d = %v, want %v", i, list.Get(i), want)
					}
				}
			}
		})
	}
}
