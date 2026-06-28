package util

import (
	"testing"
	"time"
)

func TestFormatDuration_SubSecondDuration(t *testing.T) {
	d := 355 * time.Millisecond
	expected := "355ms"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NegativeDuration(t *testing.T) {
	d := -1 * time.Minute
	expected := "-1m"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NonIntegerDuration(t *testing.T) {
	d, _ := time.ParseDuration("15m30s")
	expected := "15m 30s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_NonIntegerDurationWithoutSpaces(t *testing.T) {
	d, _ := time.ParseDuration("15m30s")
	expected := "15m30s"

	result := FormatDuration(d, false)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_ZeroDuration(t *testing.T) {
	d := 0 * time.Second
	expected := "0s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_LessThanSecond(t *testing.T) {
	d := 300 * time.Millisecond
	expected := "300ms"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_Days(t *testing.T) {
	d := time.Hour * 24 * 32
	expected := "32d"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_MoreThanADay(t *testing.T) {
	d, _ := time.ParseDuration("774h23m15s")
	expected := "32d 6h 23m 15s"

	result := FormatDuration(d, true)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatDuration_MoreThanADayUsingDefault(t *testing.T) {
	d, _ := time.ParseDuration("774h23m15s")
	expected := "774h23m15s"

	result := FormatDuration(d, false)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestParseDuration(t *testing.T) {
	type args struct {
		durationString string
	}

	tests := []struct {
		name    string
		args    args
		want    time.Duration
		wantErr bool
	}{
		// Note that any test that doesn't contain a "d" is parsed using the default time.ParseDuration function.
		{
			name:    "Extended duration",
			args:    args{durationString: "1d1h30m"},
			want:    25*time.Hour + 30*time.Minute,
			wantErr: false,
		},
		{
			name:    "Extended duration with spaces",
			args:    args{durationString: "1d 1h 30m"},
			want:    25*time.Hour + 30*time.Minute,
			wantErr: false,
		},
		{
			name:    "Valid but fields in unexpected order",
			args:    args{durationString: "5ms3h1d"},
			want:    5*time.Millisecond + 3*time.Hour + 24*time.Hour,
			wantErr: false,
		},
		{
			name:    "Empty duration",
			args:    args{durationString: "  "},
			want:    0,
			wantErr: true,
		},
		{
			name:    "Valid milliseconds",
			args:    args{durationString: "1d5ms"},
			want:    5*time.Millisecond + 24*time.Hour,
			wantErr: false,
		},
		{
			name:    "Valid seconds and milliseconds",
			args:    args{durationString: "1d3s5ms"},
			want:    3*time.Second + 5*time.Millisecond + 24*time.Hour,
			wantErr: false,
		},
		{
			name:    "Valid whole seconds",
			args:    args{durationString: "1d3s"},
			want:    3*time.Second + 24*time.Hour,
			wantErr: false,
		},
		{
			name:    "Valid fractional seconds",
			args:    args{durationString: ".2s"},
			want:    200 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "Valid hours and minutes duration",
			args:    args{durationString: "1h30m"},
			want:    time.Hour + 30*time.Minute,
			wantErr: false,
		},
		{
			name:    "valid ms duration",
			args:    args{durationString: "500ms"},
			want:    500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "Bogus duration",
			args:    args{durationString: "3q"},
			want:    0,
			wantErr: true,
		},
		{
			name:    "Bogus duration with a day",
			args:    args{durationString: "1d3q"},
			want:    0,
			wantErr: true,
		},
		// Regression tests: before the fix, "30md" and "30mh" silently reused the
		// accumulated digit buffer ("30") for the 'd'/'h' unit, yielding 30 days or
		// 30 hours. The correct interpretation is 30 minutes (the 'd'/'h' with no
		// preceding number contributes 0).
		{
			name: "minutes-then-d with no day count",
			args: args{durationString: "30md"},
			want: 30 * time.Minute,
		},
		{
			// "1d30mh": 'd' triggers the extended parser; "30mh" tests that "30"
			// before 'm' is recorded as minutes and 'h' with no number gives 0 hours.
			name: "minutes-then-h with no hour count (extended parser)",
			args: args{durationString: "1d30mh"},
			want: 24*time.Hour + 30*time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.args.durationString)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("ParseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
