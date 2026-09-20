//go:build testtools

package i18n

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"embed"
)

// localeKeyPaths enumerates every dot-path a locale map can resolve to.
func localeKeyPaths(data map[string]any) []string {
	var out []string
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for key, value := range m {
			if strings.Contains(key, ".") {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			out = append(out, path)
			if child, ok := value.(map[string]any); ok {
				walk(path, child)
			}
		}
	}
	walk("", data)
	sort.Strings(out)
	return out
}

// TestTranslatorIndexMatchesFallbackWalk pins the invariant the flat index rests
// on: a translator built with a precomputed index answers exactly what one
// without it answers, which falls back to the generic dot-path walk. The index
// is the hot path, so a divergence would silently mistranslate every locale.
func TestTranslatorIndexMatchesFallbackWalk(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("read locales: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !isYAMLFile(name) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("..", "..", "locales", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		data, err := parseYAML(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		lang := extractLangCode(name)

		var dummy embed.FS
		// defaultLang matches the locale under test so neither translator takes
		// the English fallback, leaving the index as the only difference.
		lm := &LocaleManager{
			defaultLang: lang,
			localeMaps:  map[string]map[string]any{lang: data},
			localeFS:    &dummy,
		}
		indexed := &Translator{langCode: lang, manager: lm, data: data, index: buildLookupIndex(data)}
		walking := &Translator{langCode: lang, manager: lm, data: data}

		keys := localeKeyPaths(data)
		// Case variants are valid queries, since lookups are case-insensitive.
		queries := make([]string, 0, len(keys)*3+3)
		for _, key := range keys {
			queries = append(queries, key, strings.ToUpper(key), strings.ToLower(key))
		}
		queries = append(queries, "no_such_key_xyz", "alt_names.no_such", "a.b.c.d.e")

		for _, query := range queries {
			want, wantErr := walking.GetString(query)
			got, gotErr := indexed.GetString(query)
			if (wantErr == nil) != (gotErr == nil) || want != got {
				t.Fatalf("%s: GetString(%q) index = (%q, %v), walk = (%q, %v)", name, query, got, gotErr, want, wantErr)
			}

			wantSlice, wantSliceErr := walking.GetStringSlice(query)
			gotSlice, gotSliceErr := indexed.GetStringSlice(query)
			if (wantSliceErr == nil) != (gotSliceErr == nil) || strings.Join(wantSlice, "\x00") != strings.Join(gotSlice, "\x00") {
				t.Fatalf("%s: GetStringSlice(%q) index = (%v, %v), walk = (%v, %v)", name, query, gotSlice, gotSliceErr, wantSlice, wantSliceErr)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no keys checked")
	}
	t.Logf("verified %d lookups across every locale file", checked)
}

// TestBuildLookupIndexCaseCollisions pins the fallback that keeps lookupSegment's
// exact-case precedence. A lowercased table cannot hold "Foo" and "foo" apart, so a
// colliding locale must resolve through the walk; case variants that never collide, such
// as config.yml's "alt_names.<Module>" paths, must still be indexed.
func TestBuildLookupIndexCaseCollisions(t *testing.T) {
	t.Parallel()

	t.Run("colliding keys fall back to the walk", func(t *testing.T) {
		t.Parallel()

		const yamlContent = `
Foo: upper
foo: lower
nested:
  Bar: nested value
`
		data, err := parseYAML([]byte(yamlContent))
		if err != nil {
			t.Fatalf("parseYAML() error = %v", err)
		}
		if index := buildLookupIndex(data); index != nil {
			t.Fatalf("buildLookupIndex() = %#v, want nil for case-colliding keys", index)
		}

		lm := &LocaleManager{defaultLang: "en", localeMaps: map[string]map[string]any{"en": data}}
		tr := &Translator{langCode: "en", manager: lm, data: data, index: buildLookupIndex(data)}

		for key, want := range map[string]string{
			"Foo":        "upper",
			"foo":        "lower",
			"nested.Bar": "nested value",
		} {
			got, err := tr.GetString(key)
			if err != nil {
				t.Fatalf("GetString(%q) error = %v", key, err)
			}
			if got != want {
				t.Fatalf("GetString(%q) = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("mixed case without collisions stays indexed", func(t *testing.T) {
		t.Parallel()

		data, err := parseYAML([]byte("alt_names:\n  Admin:\n    - admin\n    - admins\n"))
		if err != nil {
			t.Fatalf("parseYAML() error = %v", err)
		}
		lm := &LocaleManager{defaultLang: "en", localeMaps: map[string]map[string]any{"en": data}}
		tr := &Translator{langCode: "en", manager: lm, data: data, index: buildLookupIndex(data)}
		if tr.index == nil {
			t.Fatal("buildLookupIndex() = nil for mixed-case keys that never collide")
		}

		for _, key := range []string{"alt_names.Admin", "alt_names.admin", "alt_names.ADMIN"} {
			got, err := tr.GetStringSlice(key)
			if err != nil {
				t.Fatalf("GetStringSlice(%q) error = %v", key, err)
			}
			if len(got) != 2 || got[0] != "admin" || got[1] != "admins" {
				t.Fatalf("GetStringSlice(%q) = %v, want [admin admins]", key, got)
			}
		}
	})
}
