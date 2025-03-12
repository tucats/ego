package validate

import "testing"

func TestObject_Validate(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		arg     Object
		wantErr bool
	}{
		{
			name: "invalid object, missing required field",
			arg: Object{
				Fields: []Item{
					{Name: "name", Type: StringType, Required: true},
					{Name: "age", Type: IntType, Required: true},
				},
			},
			value: map[string]interface{}{
				"age": 25,
			},
			wantErr: true,
		},
		{
			name: "invalid object, unknown field",
			arg: Object{
				Fields: []Item{
					{Name: "name", Type: StringType, Required: true},
					{Name: "ages", Type: IntType, Required: true},
				},
			},
			value: map[string]interface{}{
				"name": "John",
				"age":  25,
			},
			wantErr: true,
		},
		{
			name: "valid object",
			arg: Object{
				Fields: []Item{
					{Name: "name", Type: StringType, Required: true},
					{Name: "age", Type: IntType, Required: true},
				},
			},
			value: map[string]interface{}{
				"name": "John",
				"age":  25,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.arg.Validate(tt.value); (err != nil) != tt.wantErr {
				t.Errorf("Object.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
