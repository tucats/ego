package ui

import (
	"reflect"
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
			name: "Message with session with session parameter in localized string",
			arg:  `{"time":"2022-01-11 12:34:56","id":"12345","seq":1,"session":42,"class":"info","msg":"log.test.test3","args":{"path":"bar"}}`,
			want: "[2022-01-11 12:34:56]    1       INFO : [42] test session bar",
		},
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
			name: "Message with session value but not present in localized string",
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

func Test_getArgMap(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want map[string]interface{}
	}{
		{
			name: "ui.A arguments",
			args: []interface{}{A{"key1": "value1", "key2": "value2"}},
			want: map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
		{
			name: "map[string]interface{} arguments",
			args: []interface{}{map[string]interface{}{"key1": "value1", "key2": "value2"}},
			want: map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
		{
			name: "No arguments",
			args: []interface{}{},
			want: nil,
		},
		{
			name: "Single non-map argument",
			args: []interface{}{123},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getArgMap(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArgMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
