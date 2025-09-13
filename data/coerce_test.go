package data

import (
	"testing"
)

func TestCoerce(t *testing.T) {
	type args struct {
		v     any
		model any
	}

	tests := []struct {
		name string
		args args
		want any
	}{
		{
			name: "test with float32 int64 model",
			args: args{
				v:     float32(100.5),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with float32 int32 model",
			args: args{
				v:     float32(100.5),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with nil byte model",
			args: args{
				v:     1,
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with bool byte model",
			args: args{
				v:     true,
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with byte/byte model",
			args: args{
				v:     byte(1),
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with int byte model",
			args: args{
				v:     100,
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with int32 byte model",
			args: args{
				v:     int32(100),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with int64 byte model",
			args: args{
				v:     int64(100),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with float32 byte model",
			args: args{
				v:     float32(100.5),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with float64 byte model",
			args: args{
				v:     float64(100.5),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with string byte model",
			args: args{
				v:     "100",
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with nil int32 model",
			args: args{
				v:     1,
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with bool int32 model",
			args: args{
				v:     true,
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with byte int32 model",
			args: args{
				v:     byte(1),
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with int int32 model",
			args: args{
				v:     100,
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with int32/int32 model",
			args: args{
				v:     int32(100),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with int64 int32 model",
			args: args{
				v:     int64(100),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with float64 int32 model",
			args: args{
				v:     float64(100.5),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with string int32 model",
			args: args{
				v:     "100",
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with nil int64 model",
			args: args{
				v:     1,
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with bool int64 model",
			args: args{
				v:     true,
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with byte int64 model",
			args: args{
				v:     byte(1),
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with int int64 model",
			args: args{
				v:     100,
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with int32 int64 model",
			args: args{
				v:     int32(100),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with int64/int64 model",
			args: args{
				v:     int64(100),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with float64 int64 model",
			args: args{
				v:     float64(100.5),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with string int64 model",
			args: args{
				v:     "100",
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with nil int model",
			args: args{
				v:     1,
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with bool int model",
			args: args{
				v:     true,
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with byte int model",
			args: args{
				v:     byte(1),
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with int/int model",
			args: args{
				v:     100,
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with int32 int model",
			args: args{
				v:     int32(100),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with int64 int model",
			args: args{
				v:     int64(100),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with float32 int model",
			args: args{
				v:     float32(100.5),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with float64 int model",
			args: args{
				v:     float64(100.5),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with string int model",
			args: args{
				v:     "100",
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with nil float32 model",
			args: args{
				v:     1,
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with bool float32 model",
			args: args{
				v:     true,
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with byte float32 model",
			args: args{
				v:     byte(1),
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with int32 float32 model",
			args: args{
				v:     int32(100),
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with int float32 model",
			args: args{
				v:     100,
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with int64 float32 model",
			args: args{
				v:     int64(100),
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with float32/float32 model",
			args: args{
				v:     float32(100.5),
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with float64 float32 model",
			args: args{
				v:     float64(100.5),
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with string float32 model",
			args: args{
				v:     "100.5",
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with nil float64 model",
			args: args{
				v:     1,
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with bool float64 model",
			args: args{
				v:     true,
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with byte float64 model",
			args: args{
				v:     byte(1),
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with int32 float64 model",
			args: args{
				v:     int32(100),
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with int float64 model",
			args: args{
				v:     100,
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with int64 float64 model",
			args: args{
				v:     int64(100),
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with float32 float64 model",
			args: args{
				v:     float32(100.5),
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with float64/float64 model",
			args: args{
				v:     float64(100.5),
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with string float64 model",
			args: args{
				v:     "100.5",
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with bool string model",
			args: args{
				v:     true,
				model: "",
			},
			want: "true",
		},
		{
			name: "test with byte string model",
			args: args{
				v:     byte(1),
				model: "",
			},
			want: "1",
		},
		{
			name: "test with int string model",
			args: args{
				v:     100,
				model: "",
			},
			want: "100",
		},
		{
			name: "test with int32 string model",
			args: args{
				v:     int32(100),
				model: "",
			},
			want: "100",
		},
		{
			name: "test with int64 string model",
			args: args{
				v:     int64(100),
				model: "",
			},
			want: "100",
		},
		{
			name: "test with float32 string model",
			args: args{
				v:     float32(100.5),
				model: "",
			},
			want: "100.5",
		},
		{
			name: "test with float64 string model",
			args: args{
				v:     float64(100.5),
				model: "",
			},
			want: "100.5",
		},
		{
			name: "test with string/string model",
			args: args{
				v:     "hello",
				model: "",
			},
			want: "hello",
		},
		{
			name: "test with nil bool model",
			args: args{
				v:     nil,
				model: false,
			},
			want: false,
		},
		{
			name: "test with bool/bool model",
			args: args{
				v:     true,
				model: false,
			},
			want: true,
		},
		{
			name: "test with byte bool model",
			args: args{
				v:     byte(1),
				model: false,
			},
			want: true,
		},
		{
			name: "test with int32 bool model",
			args: args{
				v:     int32(100),
				model: false,
			},
			want: true,
		},
		{
			name: "test with int bool model",
			args: args{
				v:     100,
				model: false,
			},
			want: true,
		},
		{
			name: "test with int64 bool model",
			args: args{
				v:     int64(100),
				model: false,
			},
			want: true,
		},
		{
			name: "test with float32 bool model",
			args: args{
				v:     float32(100.5),
				model: false,
			},
			want: true,
		},
		{
			name: "test with float64 bool model",
			args: args{
				v:     float64(100.5),
				model: false,
			},
			want: true,
		},
		{
			name: "test with string bool model",
			args: args{
				v:     "true",
				model: false,
			},
			want: true,
		},
		{
			name: "test with empty string bool model",
			args: args{
				v:     "",
				model: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := Coerce(tt.args.v, tt.args.model); got != tt.want {
				t.Errorf("Coerce(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
