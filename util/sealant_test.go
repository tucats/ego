package util

import "testing"

func TestSeal(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "test1",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Seal(tt.name)
			got := s.Unseal()

			if got != tt.name {
				t.Errorf("Seal() = %v, want %v", got, tt.name)
			}
		})
	}
}
