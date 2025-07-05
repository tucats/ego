package commands

import "testing"

func Test_makeFilter(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		want    string
	}{
		{
			name:    "char constant with space",
			filters: []string{`age = "test value"`},
			want:    `EQ(age,"test value")`,
		},
		{
			name:    "char constant",
			filters: []string{`age = "test"`},
			want:    `EQ(age,test)`,
		},
		{
			name:    "signed constant",
			filters: []string{"age = -1"},
			want:    "EQ(age,-1)",
		},
		{
			name:    "simple equals",
			filters: []string{"age=55"},
			want:    "EQ(age,55)",
		},
		{
			name:    "simple greater-than-or-equal-to",
			filters: []string{"age>=55"},
			want:    "GE(age,55)",
		},
		{
			name:    "simple not-equal-to",
			filters: []string{"age!=55"},
			want:    "NOT(EQ(age,55))",
		},
		{
			name:    "compound terms",
			filters: []string{"age>=18", "age < 65"},
			want:    "AND(GE(age,18),LT(age,65))",
		},
		{
			name:    "even more compound terms",
			filters: []string{"age>=18", "age < 65", "age != 0"},
			want:    "AND(GE(age,18),AND(LT(age,65),NOT(EQ(age,0))))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeFilter(tt.filters); got != tt.want {
				t.Errorf("makeFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
