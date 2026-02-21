package string_handling

import "testing"

func TestFindInStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{"found in middle", []string{"a", "b", "c"}, "b", true},
		{"found at start", []string{"a", "b", "c"}, "a", true},
		{"found at end", []string{"a", "b", "c"}, "c", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"empty string found", []string{"", "a"}, "", true},
		{"empty string not found", []string{"a", "b"}, "", false},
		{"case sensitive miss", []string{"Hello"}, "hello", false},
		{"single element found", []string{"x"}, "x", true},
		{"single element not found", []string{"x"}, "y", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindInStringSlice(tt.slice, tt.val); got != tt.want {
				t.Errorf("FindInStringSlice(%v, %q) = %v, want %v", tt.slice, tt.val, got, tt.want)
			}
		})
	}
}

func TestFindInInt64Slice(t *testing.T) {
	tests := []struct {
		name  string
		slice []int64
		val   int64
		want  bool
	}{
		{"found in middle", []int64{1, 2, 3}, 2, true},
		{"found at start", []int64{1, 2, 3}, 1, true},
		{"found at end", []int64{1, 2, 3}, 3, true},
		{"not found", []int64{1, 2, 3}, 4, false},
		{"empty slice", []int64{}, 1, false},
		{"nil slice", nil, 1, false},
		{"zero found", []int64{0, 1}, 0, true},
		{"zero not found", []int64{1, 2}, 0, false},
		{"negative found", []int64{-1, 0, 1}, -1, true},
		{"negative not found", []int64{1, 2, 3}, -1, false},
		{"large value found", []int64{1 << 62, 2}, 1 << 62, true},
		{"single element found", []int64{42}, 42, true},
		{"single element not found", []int64{42}, 43, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindInInt64Slice(tt.slice, tt.val); got != tt.want {
				t.Errorf("FindInInt64Slice(%v, %d) = %v, want %v", tt.slice, tt.val, got, tt.want)
			}
		})
	}
}

func TestIsDuplicateInStringSlice(t *testing.T) {
	tests := []struct {
		name      string
		arr       []string
		wantDup   string
		wantFound bool
	}{
		{"no duplicates", []string{"a", "b", "c"}, "", false},
		{"duplicate at end", []string{"a", "b", "a"}, "a", true},
		{"duplicate adjacent", []string{"a", "a", "b"}, "a", true},
		{"multiple duplicates returns first", []string{"a", "b", "a", "b"}, "a", true},
		{"empty slice", []string{}, "", false},
		{"nil slice", nil, "", false},
		{"single element", []string{"x"}, "", false},
		{"empty string duplicate", []string{"", "a", ""}, "", true},
		{"empty string no duplicate", []string{"", "a", "b"}, "", false},
		{"all same", []string{"x", "x", "x"}, "x", true},
		{"case sensitive no dup", []string{"Hello", "hello"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDup, gotFound := IsDuplicateInStringSlice(tt.arr)
			if gotFound != tt.wantFound {
				t.Errorf("IsDuplicateInStringSlice(%v) found = %v, want %v", tt.arr, gotFound, tt.wantFound)
			}
			if tt.wantFound && gotDup != tt.wantDup {
				t.Errorf("IsDuplicateInStringSlice(%v) dup = %q, want %q", tt.arr, gotDup, tt.wantDup)
			}
		})
	}
}
