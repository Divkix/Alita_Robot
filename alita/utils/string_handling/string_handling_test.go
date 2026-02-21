package string_handling

import (
	"math"
	"testing"
)

func TestFindInStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{name: "nil slice", slice: nil, val: "x", want: false},
		{name: "empty slice", slice: []string{}, val: "x", want: false},
		{name: "value present", slice: []string{"a", "b", "c"}, val: "b", want: true},
		{name: "value absent", slice: []string{"a", "b", "c"}, val: "d", want: false},
		{name: "empty string present", slice: []string{"a", "", "c"}, val: "", want: true},
		{name: "empty string absent", slice: []string{"a", "b", "c"}, val: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FindInStringSlice(tc.slice, tc.val)
			if got != tc.want {
				t.Fatalf("FindInStringSlice(%v, %q) = %v, want %v", tc.slice, tc.val, got, tc.want)
			}
		})
	}
}

func TestFindInInt64Slice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		slice []int64
		val   int64
		want  bool
	}{
		{name: "nil slice", slice: nil, val: 1, want: false},
		{name: "empty slice", slice: []int64{}, val: 1, want: false},
		{name: "value present", slice: []int64{1, 2, 3}, val: 2, want: true},
		{name: "value absent", slice: []int64{1, 2, 3}, val: 4, want: false},
		{name: "zero present", slice: []int64{0, 1, 2}, val: 0, want: true},
		{name: "zero absent", slice: []int64{1, 2, 3}, val: 0, want: false},
		{name: "negative channel ID present", slice: []int64{-1001234567890, 1, 2}, val: -1001234567890, want: true},
		{name: "math.MaxInt64 present", slice: []int64{math.MaxInt64, 1}, val: math.MaxInt64, want: true},
		{name: "math.MinInt64 present", slice: []int64{math.MinInt64, 1}, val: math.MinInt64, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FindInInt64Slice(tc.slice, tc.val)
			if got != tc.want {
				t.Fatalf("FindInInt64Slice(%v, %d) = %v, want %v", tc.slice, tc.val, got, tc.want)
			}
		})
	}
}

func TestIsDuplicateInStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arr      []string
		wantStr  string
		wantBool bool
	}{
		{name: "nil slice", arr: nil, wantStr: "", wantBool: false},
		{name: "empty slice", arr: []string{}, wantStr: "", wantBool: false},
		{name: "single element", arr: []string{"a"}, wantStr: "", wantBool: false},
		{name: "no duplicates", arr: []string{"a", "b", "c"}, wantStr: "", wantBool: false},
		{name: "duplicate a b a", arr: []string{"a", "b", "a"}, wantStr: "a", wantBool: true},
		{name: "all identical", arr: []string{"x", "x", "x"}, wantStr: "x", wantBool: true},
		{name: "empty string duplicates", arr: []string{"", ""}, wantStr: "", wantBool: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotStr, gotBool := IsDuplicateInStringSlice(tc.arr)
			if gotStr != tc.wantStr || gotBool != tc.wantBool {
				t.Fatalf("IsDuplicateInStringSlice(%v) = (%q, %v), want (%q, %v)", tc.arr, gotStr, gotBool, tc.wantStr, tc.wantBool)
			}
		})
	}
}
