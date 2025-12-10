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

func TestTokenizer_Delete(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		src     string
		isCode  bool
		start   int
		end     int
		want    string
		wantErr bool
	}{
		{
			name:    "delete single token",
			src:     "Hello, world!",
			isCode:  false,
			start:   0,
			end:     1,
			want:    ", world !",
			wantErr: false,
		},
		{
			name:    "delete multiple tokens",
			src:     "one two three four five",
			isCode:  false,
			start:   1,
			end:     3,
			want:    "one four five",
			wantErr: false,
		},
		{
			name:    "delete invalid start",
			src:     "one two three four five",
			isCode:  false,
			start:   15,
			end:     3,
			want:    "one four five",
			wantErr: true,
		},
		{
			name:    "delete invalid end",
			src:     "one two three four five",
			isCode:  false,
			start:   3,
			end:     2,
			want:    "one four five",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to := tokenizer.New(tt.src, tt.isCode)

			gotErr := to.Delete(tt.start, tt.end)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Delete() failed: %v", gotErr)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("Delete() succeeded unexpectedly")
			}

			if to.GetTokenText(0, -1) != tt.want {
				t.Errorf("Delete() = %v, want %v", to.GetTokenText(0, -1), tt.want)
			}
		})
	}
}
