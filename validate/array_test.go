package validate

import "testing"

func TestArray_Validate(t *testing.T) {
	tests := []struct {
		name    string
		arg     Array
		value   any
		wantErr bool
	}{
		{
			name: "array of integers with invalid element type",
			arg: Array{
				Type: Item{
					Type:   IntType,
					HasMin: true,
					Min:    0,
					HasMax: true,
					Max:    10,
				},
				Min: 1,
				Max: 20,
			},
			value:   []any{6.23, 2, 3},
			wantErr: true,
		},
		{
			name: "array of integers with invalid element range",
			arg: Array{
				Type: Item{
					Type:   IntType,
					HasMin: true,
					Min:    0,
					HasMax: true,
					Max:    10,
				},
				Min: 1,
				Max: 20,
			},
			value:   []any{55, 2, 3},
			wantErr: true,
		},
		{
			name: "array of integers with too few elements",
			arg: Array{
				Type: Item{
					Type:   IntType,
					HasMin: true,
					Min:    0,
					HasMax: true,
					Max:    10,
				},
				Min: 10,
				Max: 20,
			},
			value:   []any{1, 2, 3},
			wantErr: true,
		},
		{
			name: "array of integers with too many elements",
			arg: Array{
				Type: Item{
					Type:   IntType,
					HasMin: true,
					Min:    0,
					HasMax: true,
					Max:    10,
				},
				Min: 1,
				Max: 2,
			},
			value:   []any{1, 2, 3},
			wantErr: true,
		},
		{
			name: "array of integers",
			arg: Array{
				Type: Item{
					Type:   IntType,
					HasMin: true,
					Min:    0,
					HasMax: true,
					Max:    10,
				},
				Min: 1,
				Max: 3,
			},
			value: []any{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.arg.Validate(tt.value); (err != nil) != tt.wantErr {
				t.Errorf("Array.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
