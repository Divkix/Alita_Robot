package extraction

import (
	"math"
	"testing"
	"unicode/utf16"
)

// entityTextReference is the contract ExtractEntityText documents, built on
// the standard library: encode the whole source to UTF-16, take the code
// units [offset, offset+length) when that range lies inside it, and decode
// them (a unit that splits a surrogate pair decodes to U+FFFD). An empty
// source, a negative offset, a non-positive length or a range that runs past
// the source gives "".
func entityTextReference(source string, offset, length int64) string {
	if source == "" || offset < 0 || length <= 0 {
		return ""
	}
	units := utf16.Encode([]rune(source))
	n := int64(len(units))
	if offset >= n || length > n-offset {
		return ""
	}
	return string(utf16.Decode(units[offset : offset+length]))
}

func FuzzExtractEntityText(f *testing.F) {
	for _, s := range []struct {
		source         string
		offset, length int64
	}{
		{"hello world", 0, 5},
		{"hello world", 6, 5},
		{"hello world", 6, 6},
		{"hello world", 11, 1},
		{"hello world", 10, 1},
		{"hello", -1, 2},
		{"hello", 0, 0},
		{"hello", 0, -3},
		{"", 0, 1},
		{"", 0, 0},
		{"😀 ok", 0, 2},
		{"😀 ok", 0, 1},
		{"😀 ok", 1, 1},
		{"😀 ok", 1, 3},
		{"a😀b👍🏽c", 1, 6},
		{"a😀b👍🏽c", 4, 3},
		{"\xff\xfe abc", 0, 3},
		{"\xed\xa0\x80 lone", 0, 1},
		{"x", math.MaxInt64, math.MaxInt64},
		{"xy", 1, math.MaxInt64},
		{"xy", math.MinInt64, 1},
	} {
		f.Add(s.source, s.offset, s.length)
	}

	f.Fuzz(func(t *testing.T, source string, offset, length int64) {
		check := func(offset, length int64) {
			got := ExtractEntityText(source, offset, length)
			if want := entityTextReference(source, offset, length); got != want {
				t.Fatalf("ExtractEntityText(%q, %d, %d) = %q; want %q", source, offset, length, got, want)
			}
		}
		check(offset, length)
		// Random int64s almost never land inside the source; fold them into
		// [-1, len(source)+1] so most runs also hit the in-range paths.
		span := int64(len(source)) + 3
		check(offset%span-1, length%span-1)
	})
}
