//go:build testtools

package modules

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// utf16Units returns s as UTF-16 code units, the unit Telegram measures
// entity offsets and lengths in.
func utf16Units(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// FuzzPMGroupTokenCut checks parsePMGroupToken and cutPMGroupToken on any
// message text and any entities that lie inside it:
//   - the token's byte range lies inside the text and holds its non-empty Raw;
//   - the cut removes exactly the token's UTF-16 units from the text;
//   - every kept entity lies inside the new text;
//   - as stripPMGroupToken promises, an entity wholly inside the cut is
//     dropped and every other entity covers its old text minus the cut, so an
//     entity wholly after the cut covers the same text as before.
func FuzzPMGroupTokenCut(f *testing.F) {
	for _, s := range []struct {
		text           string
		count          uint8
		o1, l1, o2, l2 uint16
		o3, l3         uint16
	}{
		{"/save @grp note bold", 3, 0, 4, 6, 3, 16, 3},
		{"/save @grp note bold", 2, 3, 6, 8, 11, 0, 0},
		{`/rules "My Group" more text`, 3, 0, 5, 7, 9, 18, 3},
		{"/rules “Curly Group” rest", 2, 6, 8, 21, 3, 0, 0},
		{`/save "👍 grp" note bold`, 2, 0, 4, 20, 3, 0, 0},
		{"/cmd 😀grp 👍🏽 text", 3, 5, 1, 6, 3, 10, 5},
		{"/cmd @g tail", 2, 0, 3, 5, 6, 0, 0},
		{"/cmd\t-1001234567890\nrest", 1, 4, 16, 0, 0, 0, 0},
		{`/c "unterminated title`, 1, 0, 2, 0, 0, 0, 0},
		{`/c "" x`, 1, 3, 2, 0, 0, 0, 0},
		{`/c "a"b" c`, 2, 3, 4, 8, 1, 0, 0},
		{"\xff \xfe\xfd x", 2, 0, 1, 2, 2, 0, 0},
		{"/cmd \xf0\x9f", 1, 5, 1, 0, 0, 0, 0},
		{"/cmd", 1, 0, 4, 0, 0, 0, 0},
		{"/cmd ", 1, 0, 1, 0, 0, 0, 0},
		{"/cmd   ", 0, 0, 0, 0, 0, 0, 0},
		{"", 0, 0, 0, 0, 0, 0, 0},
		{" leading", 1, 0, 8, 0, 0, 0, 0},
		{"/c @g", 3, 65535, 65535, 0, 1, 4, 0},
	} {
		f.Add(s.text, s.count, s.o1, s.l1, s.o2, s.l2, s.o3, s.l3)
	}

	f.Fuzz(func(t *testing.T, text string, count uint8, o1, l1, o2, l2, o3, l3 uint16) {
		tok, ok := parsePMGroupToken(text)
		if !ok {
			return
		}
		if tok.start < 0 || tok.start > tok.end || tok.end > len(text) {
			t.Fatalf("parsePMGroupToken(%q) = byte range [%d, %d) outside the text (%d bytes)", text, tok.start, tok.end, len(text))
		}
		if tok.Raw == "" || !strings.Contains(text[tok.start:tok.end], tok.Raw) {
			t.Fatalf("parsePMGroupToken(%q): Raw %q is empty or not inside the cut range %q", text, tok.Raw, text[tok.start:tok.end])
		}

		old := utf16Units(text)
		n := len(old)
		cutStart := len(utf16Units(text[:tok.start]))
		cutEnd := cutStart + len(utf16Units(text[tok.start:tok.end]))

		// Entities lie inside the text, as Telegram sends them. Url tags each
		// one with its input index so a kept entity can be matched back.
		var entities []gotgbot.MessageEntity
		if n > 0 {
			for i, p := range [][2]uint16{{o1, l1}, {o2, l2}, {o3, l3}}[:int(count%4)] {
				off := int(p[0]) % n
				length := 1 + int(p[1])%(n-off)
				entities = append(entities, gotgbot.MessageEntity{
					Type:   "bold",
					Offset: int64(off),
					Length: int64(length),
					Url:    strconv.Itoa(i),
				})
			}
		}
		input := slices.Clone(entities)

		newText, kept := cutPMGroupToken(text, entities, tok)

		if !slices.Equal(entities, input) {
			t.Fatalf("cutPMGroupToken changed its input entities: %+v, was %+v", entities, input)
		}

		cur := utf16Units(newText)
		if want := slices.Concat(old[:cutStart], old[cutEnd:]); !slices.Equal(cur, want) {
			t.Fatalf("cut of %q from %q: new text %q is not the old text without units [%d, %d)", text[tok.start:tok.end], text, newText, cutStart, cutEnd)
		}

		byID := make(map[string]gotgbot.MessageEntity, len(kept))
		for _, e := range kept {
			if e.Offset < 0 || e.Length <= 0 || e.Offset+e.Length > int64(len(cur)) {
				t.Fatalf("cut of %q from %q: entity %+v lies outside the new text (%d UTF-16 units)", text[tok.start:tok.end], text, e, len(cur))
			}
			if _, dup := byID[e.Url]; dup {
				t.Fatalf("cut of %q from %q: entity %q kept twice: %+v", text[tok.start:tok.end], text, e.Url, kept)
			}
			byID[e.Url] = e
		}

		for _, in := range input {
			start, end := int(in.Offset), int(in.Offset+in.Length)
			// The entity's units outside the cut, in order.
			var want []uint16
			for u := start; u < end; u++ {
				if u < cutStart || u >= cutEnd {
					want = append(want, old[u])
				}
			}

			out, found := byID[in.Url]
			if len(want) == 0 {
				if found {
					t.Fatalf("cut [%d, %d) of %q: entity %+v lies wholly inside the cut but was kept as %+v", cutStart, cutEnd, text, in, out)
				}
				continue
			}
			if !found {
				t.Fatalf("cut [%d, %d) of %q: entity %+v has text outside the cut but was dropped", cutStart, cutEnd, text, in)
			}
			if got := cur[out.Offset : out.Offset+out.Length]; !slices.Equal(got, want) {
				t.Fatalf("cut [%d, %d) of %q: entity %+v became %+v covering %q; want %q",
					cutStart, cutEnd, text, in, out, string(utf16.Decode(got)), string(utf16.Decode(want)))
			}
		}
	})
}
