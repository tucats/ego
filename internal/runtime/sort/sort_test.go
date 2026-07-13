package sort

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestFunctionSort(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    any
		wantErr bool
	}{
		{
			name: "integer sort",
			args: data.NewList(
				data.NewArrayFromList(data.IntType,
					data.NewList(55, 2, 18),
				),
			),
			want: []any{2, 18, 55},
		},
		{
			name:    "scalar args",
			args:    data.NewList(66, 55),
			want:    []any{55, 66},
			wantErr: false,
		},
		{
			name:    "mixed scalar args",
			args:    data.NewList("tom", 3),
			want:    []any{"3", "tom"},
			wantErr: false,
		},
		{
			name: "integer sort",
			args: data.NewList(
				data.NewArrayFromList(data.IntType,
					data.NewList(55, 2, 18),
				),
			),
			want: []any{2, 18, 55},
		},
		{
			name: "float64 sort",
			args: data.NewList(
				data.NewArrayFromList(data.Float64Type,
					data.NewList(55.0, 2, 18.5),
				),
			),
			want: []any{2.0, 18.5, 55.0},
		},
		{
			name: "string sort",
			args: data.NewList(
				data.NewArrayFromList(data.StringType,
					data.NewList("pony", "cake", "unicorn", 5),
				),
			),
			want: []any{"5", "cake", "pony", "unicorn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("sort testing")
			s.SetAlways(defs.ExtensionsVariable, true)

			result, err := genericSort(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionSort() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			list, ok := result.(data.List)
			if !ok {
				t.Fatalf("genericSort() returned %T, want data.List", result)
			}

			gotArray, ok := list.Get(0).(*data.Array)
			if !ok || !reflect.DeepEqual(gotArray.BaseArray(), tt.want) {
				t.Errorf("FunctionSort() = %v, want %v", list.Get(0), tt.want)
			}
		})
	}
}

func TestSearchTypeSpecificFunctions(t *testing.T) {
	s := symbols.NewSymbolTable("sort search testing")

	t.Run("SearchInts", func(t *testing.T) {
		args := data.NewList(
			data.NewArrayFromList(data.IntType, data.NewList(1, 3, 5, 7, 9)),
			6,
		)

		result, err := sortSearchInts(s, args)
		if err != nil {
			t.Fatalf("sortSearchInts() error = %v", err)
		}

		list := result.(data.List)
		if got := list.Get(0); got != 3 {
			t.Errorf("sortSearchInts() = %v, want 3", got)
		}

		if list.Get(1) != nil {
			t.Errorf("sortSearchInts() err = %v, want nil", list.Get(1))
		}
	})

	t.Run("SearchInts wrong type", func(t *testing.T) {
		args := data.NewList(
			data.NewArrayFromList(data.StringType, data.NewList("a", "b")),
			1,
		)

		result, err := sortSearchInts(s, args)
		if err == nil {
			t.Fatalf("sortSearchInts() expected error, got nil")
		}

		list := result.(data.List)
		if list.Get(1) == nil {
			t.Errorf("sortSearchInts() list error slot is nil, want non-nil")
		}
	})

	t.Run("SearchFloat64s", func(t *testing.T) {
		args := data.NewList(
			data.NewArrayFromList(data.Float64Type, data.NewList(1.1, 3.3, 5.5, 7.7)),
			5.5,
		)

		result, err := sortSearchFloat64s(s, args)
		if err != nil {
			t.Fatalf("sortSearchFloat64s() error = %v", err)
		}

		list := result.(data.List)
		if got := list.Get(0); got != 2 {
			t.Errorf("sortSearchFloat64s() = %v, want 2", got)
		}
	})

	t.Run("SearchStrings", func(t *testing.T) {
		args := data.NewList(
			data.NewArrayFromList(data.StringType, data.NewList("apple", "banana", "cherry")),
			"banana",
		)

		result, err := sortSearchStrings(s, args)
		if err != nil {
			t.Fatalf("sortSearchStrings() error = %v", err)
		}

		list := result.(data.List)
		if got := list.Get(0); got != 1 {
			t.Errorf("sortSearchStrings() = %v, want 1", got)
		}
	})
}
