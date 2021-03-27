package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestStructImpl(t *testing.T) {
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
			stack: []interface{}{"usertype", "__type", true, "flag"},
			want: map[string]interface{}{
				"flag": true,
				"test": 0,
				"__metadata": map[string]interface{}{
					"static": true,
					"type": datatypes.Structure(
						datatypes.Field{Name: "flag", Type: datatypes.BoolType},
					),
					"replica": 0,
				}},
			wantErr: false,
			static:  true,
		},
		{
			name:  "one member test",
			arg:   1,
			stack: []interface{}{123, "test"},
			want: map[string]interface{}{
				"test": 123,
				"__metadata": map[string]interface{}{
					"replica": 0,
					"static":  true,
					"type": datatypes.Structure(
						datatypes.Field{Name: "test", Type: datatypes.IntType},
					),
				}},
			wantErr: false,
		},
		{
			name:  "two member test",
			arg:   2,
			stack: []interface{}{true, "active", 123, "test"},
			want: map[string]interface{}{
				"active": true,
				"test":   123,
				"__metadata": map[string]interface{}{
					"static":  true,
					"replica": 0,
					"type":    datatypes.StructType,
				}},
			wantErr: false,
		},
		{
			name:  "two member valid static test",
			arg:   2,
			stack: []interface{}{true, "active", 123, "test"},
			want: map[string]interface{}{
				"active": true,
				"test":   123,
				"__metadata": map[string]interface{}{
					"static":  true,
					"type":    datatypes.StructType,
					"replica": 0,
				}},
			wantErr: false,
			static:  true,
		},
		{
			name:  "two member invalid static test",
			arg:   3,
			stack: []interface{}{"usertype", "__type", true, "invalid", 123, "test"},
			want: map[string]interface{}{
				"active": true,
				"test":   123,
				"__metadata": map[string]interface{}{
					"type":   datatypes.StructType,
					"static": true,
				}},
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

			typeDef := datatypes.TypeDefinition("usertype", datatypes.Structure(
				datatypes.Field{Name: "active", Type: datatypes.BoolType},
				datatypes.Field{Name: "test", Type: datatypes.IntType},
			))

			_ = ctx.symbols.SetAlways("usertype", typeDef)

			err := structByteCode(ctx, tt.arg)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("StructImpl() error = %v, wantErr %v", err, tt.wantErr)
			} else if errors.Nil(err) {
				got, _ := ctx.Pop()
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("StructImpl()\n  got  %v\n  want %v", got, tt.want)
				}
			}
		})
	}
}
