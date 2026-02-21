package config

import (
	"math"
	"testing"
)

func tc(s string) typeConvertor { return typeConvertor{str: s} }

func TestTypeConvertor_Bool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"1", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"no", false},
		{"0", false},
		{"", false},
		{"2", false},
		{"on", false},
		{"enabled", false},
		{" true ", true}, // trimmed
		{" yes ", true},  // trimmed
		{" false ", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := tc(tt.input).Bool(); got != tt.want {
				t.Errorf("Bool(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeConvertor_Int(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"-1", -1},
		{"-100", -100},
		{"2147483647", math.MaxInt32},
		{"", 0},
		{"abc", 0},
		{"1.5", 0},
		{"  ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := tc(tt.input).Int(); got != tt.want {
				t.Errorf("Int(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeConvertor_Int64(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"0", 0},
		{"1", 1},
		{"9223372036854775807", math.MaxInt64},
		{"-9223372036854775808", math.MinInt64},
		{"-42", -42},
		{"", 0},
		{"abc", 0},
		{"1.5", 0},
		{"9999999999999", 9999999999999},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := tc(tt.input).Int64(); got != tt.want {
				t.Errorf("Int64(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeConvertor_Float64(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"0", 0.0},
		{"1.5", 1.5},
		{"3.14", 3.14},
		{"-2.5", -2.5},
		{"42", 42.0},
		{"", 0.0},
		{"abc", 0.0},
		{"1e3", 1000.0},
		{"1.7976931348623157e+308", math.MaxFloat64},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := tc(tt.input).Float64(); got != tt.want {
				t.Errorf("Float64(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeConvertor_StringArray(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single", "a", []string{"a"}},
		{"two elements", "a,b", []string{"a", "b"}},
		{"three elements", "a,b,c", []string{"a", "b", "c"}},
		{"whitespace trimmed", " a , b , c ", []string{"a", "b", "c"}},
		{"empty string", "", []string{""}},
		{"trailing comma", "a,b,", []string{"a", "b", ""}},
		{"leading comma", ",a,b", []string{"", "a", "b"}},
		{"spaces only elements", " , , ", []string{"", "", ""}},
		{"mixed whitespace", "foo , bar,baz ", []string{"foo", "bar", "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tc(tt.input).StringArray()
			if len(got) != len(tt.want) {
				t.Fatalf("StringArray(%q) len = %d, want %d; got %v", tt.input, len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("StringArray(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
