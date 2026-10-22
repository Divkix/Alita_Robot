// Package spamtext normalises message text and compares two texts by the
// character n-grams they share, so a spam text an admin labelled matches its
// near-duplicates: the same pitch with a changed link, an emoji added, or
// letters swapped for look-alikes.
//
// Character n-grams need no tokeniser, so Chinese, Japanese and Thai, which
// whitespace splitting breaks, compare like any other script.
package spamtext

import (
	"math"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	// MaxRunes bounds the normalised text that is stored and compared. A spam
	// pitch fits; a longer message is judged on its start, so one message can
	// never cost more than about 3*MaxRunes hashes.
	MaxRunes = 1000

	// MinRunes is the shortest normalised text worth learning or matching: a
	// shorter one ("hello", "ok thanks") is said by everyone.
	MinRunes = 10

	minGram = 3
	maxGram = 5
)

// homoglyphs folds the look-alike letters spammers swap in to dodge filters:
// Cyrillic and Greek capitals and small letters that render as Latin ones.
// NFKC already folds fullwidth and mathematical letters (Ｃ, 𝗮), so this table
// only covers letters NFKC keeps. Keep it small: every entry also rewrites
// real Cyrillic or Greek text, the same way on both sides of a comparison.
var homoglyphs = map[rune]rune{
	// Cyrillic capitals.
	'А': 'a', 'В': 'b', 'Е': 'e', 'К': 'k', 'М': 'm', 'Н': 'h', 'О': 'o',
	'Р': 'p', 'С': 'c', 'Т': 't', 'Х': 'x', 'І': 'i', 'Ј': 'j', 'Ѕ': 's',
	// Cyrillic small letters.
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x',
	'і': 'i', 'ј': 'j', 'ѕ': 's', 'ԁ': 'd', 'һ': 'h', 'ӏ': 'l',
	// Greek capitals and small letters.
	'Α': 'a', 'Β': 'b', 'Ε': 'e', 'Η': 'h', 'Ι': 'i', 'Κ': 'k', 'Μ': 'm',
	'Ν': 'n', 'Ο': 'o', 'Ρ': 'p', 'Τ': 't', 'Χ': 'x', 'Υ': 'y', 'Ζ': 'z',
	'ο': 'o', 'ν': 'v', 'ρ': 'p', 'ι': 'i', 'κ': 'k', 'α': 'a',
}

// Normalize returns the text spam is compared on: NFKC, look-alike letters
// folded to Latin, lower case, punctuation, symbols, emoji and invisible
// characters dropped, and every whitespace run collapsed to one space. It
// keeps at most MaxRunes runes. Dropping punctuation outright, rather than
// turning it into a space, makes "c.a.s.i.n.o" read as "casino".
func Normalize(text string) string {
	folded := norm.NFKC.String(text)

	var b strings.Builder
	b.Grow(len(folded))
	runes := 0
	pendingSpace := false
	for _, r := range folded {
		if runes >= MaxRunes {
			break
		}
		if unicode.IsSpace(r) {
			pendingSpace = runes > 0
			continue
		}
		if mapped, ok := homoglyphs[r]; ok {
			r = mapped
		}
		r = unicode.ToLower(r)
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) {
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			runes++
			pendingSpace = false
			if runes >= MaxRunes {
				break
			}
		}
		b.WriteRune(r)
		runes++
	}
	return b.String()
}

// RuneCount is the length Normalize's output is measured in.
func RuneCount(normalized string) int {
	return len([]rune(normalized))
}

// Vector is the set of hashed 3- to 5-rune n-grams of a normalised text,
// sorted and without duplicates. A nil Vector matches nothing.
type Vector []uint32

// Vectorize hashes every 3-, 4- and 5-rune window of a normalised text. The
// set (not the counts) is kept: a spammer repeating a phrase does not move it.
func Vectorize(normalized string) Vector {
	runes := []rune(normalized)
	if len(runes) > MaxRunes {
		runes = runes[:MaxRunes]
	}
	if len(runes) < minGram {
		return nil
	}

	grams := make([]uint32, 0, 3*len(runes))
	for start := range runes {
		// FNV-1a over the rune values: each longer window extends the hash
		// of the shorter one, so a start costs maxGram steps, not 12.
		h := uint32(fnvOffset)
		for size := 1; size <= maxGram && start+size <= len(runes); size++ {
			h ^= uint32(runes[start+size-1])
			h *= fnvPrime
			if size >= minGram {
				grams = append(grams, h)
			}
		}
	}
	slices.Sort(grams)
	return Vector(slices.Compact(grams))
}

const (
	fnvOffset = 2166136261
	fnvPrime  = 16777619
)

// Similarity is the cosine of two binary n-gram vectors: shared n-grams over
// the geometric mean of their sizes, from 0 (nothing shared) to 1 (the same
// set).
func Similarity(a, b Vector) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			shared++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return float64(shared) / math.Sqrt(float64(len(a))*float64(len(b)))
}

// CanReach reports whether two vectors of these sizes can be at least
// threshold similar. The cosine of binary vectors is at most
// sqrt(min/max) of their sizes, so a text twice as long as another can never
// pass 0.71: the check skips a comparison without walking either vector.
func CanReach(a, b Vector, threshold float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	small, large := float64(len(a)), float64(len(b))
	if small > large {
		small, large = large, small
	}
	return math.Sqrt(small/large) >= threshold
}
