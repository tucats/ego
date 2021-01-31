package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
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
					"static":  true,
					"type":    "usertype",
					"replica": 1,
					"parent": map[string]interface{}{
						"__metadata": map[string]interface{}{
							"static": true,
							"type":   "usertype",
						},
						"flag": false,
						"test": 0,
					},
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
					"type":    "struct",
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
					"type":    "struct",
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
					"type":    "struct",
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
					"type":   "struct",
					"static": true,
				}},
			wantErr: true,
			static:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				stack:   tt.stack,
				sp:      len(tt.stack),
				Static:  tt.static,
				symbols: symbols.NewSymbolTable("test bench"),
			}

			model := map[string]interface{}{
				"test": 0,
				"flag": false,
				datatypes.MetadataKey: map[string]interface{}{
					datatypes.TypeMDKey:   "usertype",
					datatypes.StaticMDKey: true,
				},
			}
			_ = ctx.symbols.SetAlways("usertype", model)

			err := StructImpl(ctx, tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructImpl() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil {
				got, _ := ctx.Pop()
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("StructImpl() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
