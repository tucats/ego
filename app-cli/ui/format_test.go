package ui

import (
	"strings"
	"testing"
)

func Test_formatJSONLogEntryAsText(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
		err  bool
	}{
		{
			name: "Message with path parameter",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":1,"class":"info","msg":"log.test.test1","args":{"path":"bar"}}`,
			want: "[2022-01-11 12:34:56]    1       INFO : Test message1 bar",
		},
		{
			name: "Message with missing parameter data",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":2,"class":"info","msg":"log.test.test1"}`,
			want: "[2022-01-11 12:34:56]    2       INFO : Test message1 {{path}}",
		},
		{
			name: "Message with session value",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":2,"class":"info","session":5, "msg":"log.test.test1","args":{"path":"foo"}}`,
			want: "[2022-01-11 12:34:56]    2       INFO : [5] Test message1 foo",
		},
		{
			name: "Message that has no parameter data",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":2,"class":"server","msg":"log.test.test2"}`,
			want: "[2022-01-11 12:34:56]    2     SERVER : Test message2",
		},
		{
			name: "Message with unused parameter data",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":2,"class":"server","msg":"log.test.test2","args":{"path":"bar"}}`,
			want: "[2022-01-11 12:34:56]    2     SERVER : Test message2",
		},
		{
			name: "Message with no localization",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":2,"class":"server","msg":"log.test.test0"}`,
			want: "[2022-01-11 12:34:56]    2     SERVER : log.test.test0",
		},
		{
			name: "Message not in JSON format",
			arg:  `Not a JSON string at all`,
			want: "Not a JSON string at all",
		},
		{
			name: "Message in invalid JSON format",
			arg:  `{{}`,
			want: "Error unmarshalling JSON log entry: invalid character '{' looking for beginning of object key string",
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatJSONLogEntryAsText(tt.arg); got != tt.want {
				// Depends on string constant in error reporting part of this function
				if !(tt.err && strings.HasPrefix(got, "Error unmarshalling JSON log entry:")) {
					t.Errorf("formatJSONLogEntryAsText()\ngot: %v, \nwant:%v", got, tt.want)
				}
			}
		})
	}
}
