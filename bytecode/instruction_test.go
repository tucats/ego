package bytecode

import (
	"testing"

	"github.com/tucats/ego/tokenizer"
)

func Test_instruction_String(t *testing.T) {
	tests := []struct {
		name      string
		Operation Opcode
		Operand   any
		want      string
	}{
		{
			name:      "no operand",
			Operation: PushScope,
			want:      "PushScope",
		},
		{
			name:      "integer operand",
			Operation: Branch,
			Operand:   100,
			want:      "Branch 100",
		},
		{
			name:      "string operand",
			Operation: Push,
			Operand:   "value",
			want:      "Push \"value\"",
		},
		{
			name:      "token operand",
			Operation: Push,
			Operand:   tokenizer.BoolToken,
			want:      `Push Type "bool"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := instruction{
				Operation: tt.Operation,
				Operand:   tt.Operand,
			}
			if got := i.String(); got != tt.want {
				t.Errorf("instruction.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
