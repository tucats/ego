package dsns

import "testing"

func Test_encrypt(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name:    "simple text",
			text:    "this is a test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encrypt(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("encrypt() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			got, err = decrypt(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("decrypt() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.text {
				t.Errorf("decrypt() = %v, want %v", got, tt.text)
			}
		})
	}
}
