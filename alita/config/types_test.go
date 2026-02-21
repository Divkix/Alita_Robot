package config

import (
	"math"
	"reflect"
	"testing"
)

func TestTypeConvertorBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "true", input: "true", want: true},
		{name: "yes", input: "yes", want: true},
		{name: "one", input: "1", want: true},
		{name: "TRUE uppercase", input: "TRUE", want: true},
		{name: "YES uppercase", input: "YES", want: true},
		{name: "true with spaces", input: " true ", want: true},
		{name: "false", input: "false", want: false},
		{name: "no", input: "no", want: false},
		{name: "zero", input: "0", want: false},
		{name: "empty", input: "", want: false},
		{name: "two", input: "2", want: false},
		{name: "random string", input: "random", want: false},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := typeConvertor{str: tc.input}.Bool()
			if got != tc.want {
				t.Fatalf("typeConvertor{%q}.Bool() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTypeConvertorInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "positive", input: "42", want: 42},
		{name: "negative", input: "-100", want: -100},
		{name: "zero", input: "0", want: 0},
		{name: "empty", input: "", want: 0},
		{name: "not a number", input: "not_a_number", want: 0},
		// strconv.Atoi clamps to math.MaxInt64 on overflow (error is ignored by implementation)
		{name: "overflow clamps to MaxInt64", input: "9999999999999999999", want: math.MaxInt64},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := typeConvertor{str: tc.input}.Int()
			if got != tc.want {
				t.Fatalf("typeConvertor{%q}.Int() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTypeConvertorInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "positive", input: "42", want: 42},
		{name: "negative", input: "-100", want: -100},
		{name: "zero", input: "0", want: 0},
		{name: "max int64", input: "9223372036854775807", want: math.MaxInt64},
		{name: "empty", input: "", want: 0},
		{name: "invalid", input: "invalid", want: 0},
		{name: "whitespace not trimmed", input: " 42 ", want: 0},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := typeConvertor{str: tc.input}.Int64()
			if got != tc.want {
				t.Fatalf("typeConvertor{%q}.Int64() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTypeConvertorFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    float64
		isNaN   bool
		isInf   bool
		infSign int
	}{
		{name: "pi", input: "3.14", want: 3.14},
		{name: "zero", input: "0", want: 0.0},
		{name: "negative", input: "-1.5", want: -1.5},
		{name: "empty", input: "", want: 0.0},
		{name: "invalid", input: "invalid", want: 0.0},
		{name: "NaN", input: "NaN", isNaN: true},
		{name: "Inf", input: "Inf", isInf: true, infSign: 1},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := typeConvertor{str: tc.input}.Float64()
			if tc.isNaN {
				if !math.IsNaN(got) {
					t.Fatalf("typeConvertor{%q}.Float64() = %v, want NaN", tc.input, got)
				}
				return
			}
			if tc.isInf {
				if !math.IsInf(got, tc.infSign) {
					t.Fatalf("typeConvertor{%q}.Float64() = %v, want Inf(sign=%d)", tc.input, got, tc.infSign)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("typeConvertor{%q}.Float64() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTypeConvertorStringArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "three elements", input: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "trimmed elements", input: " a , b , c ", want: []string{"a", "b", "c"}},
		{name: "single element", input: "single", want: []string{"single"}},
		{name: "empty string", input: "", want: []string{""}},
		{name: "consecutive commas", input: ",,", want: []string{"", "", ""}},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := typeConvertor{str: tc.input}.StringArray()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("typeConvertor{%q}.StringArray() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
