package main

import "testing"

func Test_TestFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "first test",
			filename: "tests/test1.json",
			wantErr:  false,
		},
	}

	LoadDictionary("tests/dictionary.json")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TestFile(tt.filename); (err != nil) != tt.wantErr {
				t.Errorf("TestFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
