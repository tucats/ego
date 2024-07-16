package bytecode

import (
	"testing"

	"github.com/tucats/ego/data"
)

func Test_Serialize(t *testing.T) {

	t.Run("serialization tst", func(t *testing.T) {
		b := New("double")
		b.SetDeclaration(&data.Declaration{
			Name:    "double",
			Type:    data.Float64Type,
			Returns: []*data.Type{data.Float64Type},
		})

		b.Emit(ArgCheck, data.NewList(0, 0, "double"))
		b.Emit(Load, 3.0)
		b.Emit(Load, 2)
		b.Emit(Mul)
		b.Emit(Load, data.StringType)
		b.Emit(Call, 1)
		b.Emit(Print, 1)
		b.Emit(Stop)

		serialized, err := b.Serialize()
		if err != nil {
			t.Error(err)
		}

		t.Error(serialized)
	})
}
