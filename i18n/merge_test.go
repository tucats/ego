package i18n

import (
	"reflect"
	"testing"
)

func TestMergeLocalization(t *testing.T) {
	tests := []struct {
		name      string
		additions map[string]map[string]string
		existing  map[string]map[string]string
		result    map[string]map[string]string
		want      int
	}{
		{
			name:      "merge single key to empty map",
			additions: map[string]map[string]string{"its.there": {"en": "there"}},
			existing:  map[string]map[string]string{},
			result:    map[string]map[string]string{"its.there": {"en": "there"}},
			want:      1,
		},
		{
			name: "merge two keys to empty map",
			additions: map[string]map[string]string{
				"its.there": {
					"en": "there",
					"fr": "voila",
				},
				"its.another": {"en": "another"},
			},
			existing: map[string]map[string]string{},
			result: map[string]map[string]string{
				"its.there": {
					"en": "there",
					"fr": "voila",
				},
				"its.another": {"en": "another"},
			},
			want: 3,
		},
		{
			name:      "merge existing key to map",
			additions: map[string]map[string]string{"its.there": {"fr": "voila"}},
			existing:  map[string]map[string]string{"its.there": {"en": "there"}},
			result: map[string]map[string]string{
				"its.there": {
					"en": "there",
					"fr": "voila",
				},
			},
			want: 1,
		},
		{
			name:      "merge new key to exiting map",
			additions: map[string]map[string]string{"nope": {"fr": "non"}},
			existing: map[string]map[string]string{
				"its.there": {
					"en": "there",
					"fr": "voila",
				},
			},
			result: map[string]map[string]string{
				"nope": {
					"fr": "non",
				},
				"its.there": {
					"en": "there",
					"fr": "voila",
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeLocalizationMap(tt.additions, tt.existing); got != tt.want {
				t.Errorf("MergeLocalization() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(tt.existing, tt.result) {
				t.Errorf("existing map not updated correctly: got %v, want %v", tt.existing, tt.result)
			}
		})
	}
}
