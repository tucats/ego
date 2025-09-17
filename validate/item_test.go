package validate

import "testing"

func TestItem_Validate(t *testing.T) {
	tests := []struct {
		name    string
		arg     Item
		value   any
		wantErr bool
	}{
		{
			name: "invalid string enum",
			arg: Item{
				Type: StringType,
				Enum: []any{"red", "blue"},
			},
			value:   "green",
			wantErr: true,
		},
		{
			name: "invalid integer enum",
			arg: Item{
				Type:   IntType,
				HasMin: true,
				Min:    5,
				HasMax: true,
				Max:    10,
				Enum:   []any{5, 6},
			},
			value:   7,
			wantErr: true,
		},
		{
			name: "invalid integer max",
			arg: Item{
				Type:   IntType,
				HasMin: true,
				Min:    5,
				HasMax: true,
				Max:    10,
			},
			value:   55,
			wantErr: true,
		},
		{
			name: "invalid integer min",
			arg: Item{
				Type:   IntType,
				HasMin: true,
				Min:    5,
				HasMax: true,
				Max:    10,
			},
			value:   3,
			wantErr: true,
		},
		{
			name: "valid integer",
			arg: Item{
				Type:   IntType,
				HasMin: true,
				Min:    0,
				HasMax: true,
				Max:    10,
			},
			value:   5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.arg.Validate(tt.value); (err != nil) != tt.wantErr {
				t.Errorf("Item.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
