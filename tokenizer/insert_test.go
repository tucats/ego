package tokenizer_test

import (
	"testing"

	"github.com/tucats/ego/tokenizer"
)

func TestTokenizer_Insert(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		src     string
		insert  string
		pos     int
		wantErr bool
		want    string
	}{
		{
			name:   "insert at start",
			src:    "Hello, world!",
			insert: "foo",
			pos:    0,
			want:   "foo Hello , world ! ;",
		},
		{
			name:   "insert in middle",
			src:    "Hello, world!",
			insert: "foo/bar",
			pos:    1,
			want:   "Hello foo / bar , world ! ;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceTokens := tokenizer.New(tt.src, true)
			insertTokens := tokenizer.New(tt.insert, false)

			gotErr := sourceTokens.Insert(tt.pos, insertTokens.Tokens...)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Insert() failed: %v", gotErr)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("Insert() succeeded unexpectedly")
			}

			text := sourceTokens.GetTokenText(0, -1)
			if text != tt.want {
				t.Errorf("Insert() = %v, want %v", text, tt.want)
			}
		})
	}
}
