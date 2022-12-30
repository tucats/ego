package tokenizer

import "testing"

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name       string
		extensions bool
		want       bool
	}{
		{
			name:       "var",
			extensions: false,
			want:       true,
		},
		{
			name:       "zoinks",
			extensions: false,
			want:       false,
		},
		{
			name:       "print",
			extensions: false,
			want:       false,
		},
		{
			name:       "print",
			extensions: true,
			want:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewReservedToken(tt.name).IsReserved(tt.extensions); got != tt.want {
				t.Errorf("IsReserved() = %v, want %v", got, tt.want)
			}
		})
	}
}
