package spamtext

import (
	"strings"
	"testing"
)

func TestNormalizeFoldsLookAlikes(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"fullwidth, math bold and Cyrillic i", "Ｃ𝗮𝘀іno", "casino"},
		{"Cyrillic capitals", "ВUY СОІNS", "buy coins"},
		{"Greek capitals", "ΒΙΤCΟΙΝ", "bitcoin"},
		{"punctuation dropped inside a word", "c.a.s.i.n.o", "casino"},
		{"emoji and symbols dropped", "🔥 FREE $$$ money!!! 🔥", "free money"},
		{"whitespace runs collapse", "  join\n\n\tnow  ", "join now"},
		{"zero-width characters dropped", "fr\u200bee\u200d gift", "free gift"},
		{"ligature unfolded", "ﬁnance", "finance"},
		{"CJK kept as is", "免费 加入 群组", "免费 加入 群组"},
		{"Devanagari marks kept", "मुफ़्त पैसा", "मुफ़्त पैसा"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeBoundsTheLength(t *testing.T) {
	got := Normalize(strings.Repeat("ab ", 2*MaxRunes))
	if n := RuneCount(got); n > MaxRunes {
		t.Fatalf("RuneCount(Normalize(long)) = %d, want at most %d", n, MaxRunes)
	}
	if strings.HasSuffix(got, " ") {
		t.Fatalf("Normalize(long) ends in a space: %q", got[len(got)-5:])
	}
}

func TestSimilarityOfNearDuplicates(t *testing.T) {
	spam := Vectorize(Normalize("🔥 Earn $500 a day from home! DM me for the crypto signals group t.me/richfast 🔥"))
	for _, tc := range []struct {
		name string
		text string
		min  float64
		max  float64
	}{
		{"same text", "🔥 Earn $500 a day from home! DM me for the crypto signals group t.me/richfast 🔥", 1, 1},
		{"look-alike letters and new link", "Еаrn $500 а dау frоm hоmе! DM me for the crypto signals group t.me/getrich2", 0.85, 1},
		{"emoji and case changed", "EARN $500 A DAY FROM HOME 💰 dm me for the crypto signals group t.me/richfast", 0.85, 1},
		{"unrelated chat message", "Has anyone tried the new release? The build fails on my laptop since Monday.", 0, 0.3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Similarity(spam, Vectorize(Normalize(tc.text)))
			if got < tc.min || got > tc.max {
				t.Fatalf("Similarity = %.3f, want between %.2f and %.2f", got, tc.min, tc.max)
			}
		})
	}
}

func TestSimilarityWorksWithoutWhitespace(t *testing.T) {
	spam := Vectorize(Normalize("加入我们的投资群每天赚五百美元请私信我"))
	near := Vectorize(Normalize("加入我们的投资群，每天赚五百美元！请私信我😀"))
	other := Vectorize(Normalize("今天的会议改到下午三点在二楼会议室"))

	if got := Similarity(spam, near); got < 0.9 {
		t.Fatalf("Similarity(near duplicate) = %.3f, want at least 0.9", got)
	}
	if got := Similarity(spam, other); got > 0.1 {
		t.Fatalf("Similarity(unrelated) = %.3f, want at most 0.1", got)
	}
}

func TestVectorizeOfShortTextIsEmpty(t *testing.T) {
	if v := Vectorize("ab"); v != nil {
		t.Fatalf("Vectorize(%q) = %v, want nil", "ab", v)
	}
	if got := Similarity(nil, Vectorize("abcdef")); got != 0 {
		t.Fatalf("Similarity(nil, v) = %v, want 0", got)
	}
}

func TestCanReachNeverRejectsAPassingPair(t *testing.T) {
	texts := []string{
		"earn money from home with crypto signals",
		"earn money from home with crypto signals today only, limited seats",
		"earn money",
		"crypto signals group, earn from home",
	}
	for _, a := range texts {
		for _, b := range texts {
			va, vb := Vectorize(Normalize(a)), Vectorize(Normalize(b))
			for _, threshold := range []float64{0.5, 0.7, 0.85, 0.95} {
				if Similarity(va, vb) >= threshold && !CanReach(va, vb, threshold) {
					t.Fatalf("CanReach(%q, %q, %.2f) = false, but the similarity passes", a, b, threshold)
				}
			}
		}
	}
	short, long := Vectorize("earn money"), Vectorize(Normalize(strings.Repeat("earn money from home ", 10)))
	if CanReach(short, long, 0.85) {
		t.Fatal("CanReach(short, ten times longer, 0.85) = true, want false")
	}
}

func TestEveryHomoglyphFoldsToOneLatinLetter(t *testing.T) {
	for from, to := range homoglyphs {
		if got := Normalize(string(from)); got != string(to) || to < 'a' || to > 'z' {
			t.Errorf("Normalize(%q) = %q, want the Latin letter %q", from, got, to)
		}
	}
}
