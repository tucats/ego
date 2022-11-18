package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestStructImpl(t *testing.T) {
	typeDef := datatypes.TypeDefinition("usertype", datatypes.Structure(
		datatypes.Field{Name: "active", Type: &datatypes.BoolType},
		datatypes.Field{Name: "test", Type: &datatypes.IntType},
	))

	tests := []struct {
		name    string
		stack   []interface{}
		arg     interface{}
		want    interface{}
		wantErr bool
		static  bool
	}{
		{
			name:  "two member incomplete test",
			arg:   2,
			stack: []interface{}{typeDef, datatypes.TypeMDKey, true, "active"},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"active": true,
				"test":   0,
			}).SetStatic(true).AsType(typeDef),
			wantErr: false,
			static:  true,
		},
		{
			name:  "one member test",
			arg:   1,
			stack: []interface{}{123, "test"},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"test": 123,
			}).SetStatic(true),
			wantErr: false,
		},
		{
			name:  "two member test",
			arg:   2,
			stack: []interface{}{true, "active", 123, "test"},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"test":   123,
				"active": true,
			}).SetStatic(true),
			wantErr: false,
		},
		{
			name:  "two member invalid static test",
			arg:   3,
			stack: []interface{}{typeDef, datatypes.TypeMDKey, true, "invalid", 123, "test"},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"active": true,
				"test":   0,
			}).SetStatic(true).AsType(typeDef),
			wantErr: true,
			static:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				stack:        tt.stack,
				stackPointer: len(tt.stack),
				Static:       tt.static,
				symbols:      symbols.NewSymbolTable("test bench"),
			}

			_ = ctx.symbols.SetAlways("usertype", typeDef)

			err := structByteCode(ctx, tt.arg)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("StructImpl() error = %v, wantErr %v", err, tt.wantErr)
			} else if errors.Nil(err) {
				got, _ := ctx.Pop()
				f := reflect.DeepEqual(got, tt.want)

				if !f {
					t.Errorf("StructImpl()\n  got  %v\n  want %v", got, tt.want)
				}
			}
		})
	}
}
