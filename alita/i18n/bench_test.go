//go:build testtools

package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"embed"
)

// benchManager builds a LocaleManager over the real English locale file so the
// benchmarks exercise production key shapes and map sizes rather than a toy map.
func benchManager(b *testing.B) *LocaleManager {
	b.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "locales", "en.yml"))
	if err != nil {
		b.Skipf("locale file unavailable: %v", err)
	}
	data, err := parseYAML(raw)
	if err != nil {
		b.Fatalf("parseYAML: %v", err)
	}
	var dummy embed.FS
	lm := &LocaleManager{
		defaultLang: "en",
		localeMaps:  map[string]map[string]any{"en": data},
		localeFS:    &dummy,
	}
	_ = GetManager()
	managerInstance = lm
	return lm
}

func benchTranslator(b *testing.B) *Translator {
	b.Helper()
	lm := benchManager(b)
	tr, err := lm.GetTranslator("en")
	if err != nil {
		b.Fatalf("GetTranslator: %v", err)
	}
	return tr
}

// Keys below are real keys from locales/en.yml, which stores every translation
// as a flat top-level scalar (1023 of them).
var benchKeys = []string{
	"admin_no_visible_admins",
	"common_no_user_specified",
	"bans_restrict_question",
	"admin_adminlist_note_cached",
}

func BenchmarkGetString(b *testing.B) {
	tr := benchTranslator(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if _, err := tr.GetString(benchKeys[i%len(benchKeys)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetStringWithNamedParams(b *testing.B) {
	tr := benchTranslator(b)
	params := TranslationParams{"first": "Ada", "second": "Grace"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := tr.GetString("admin_no_visible_admins", params); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetStringMissingKey measures the miss path: no locale holds the key, so the
// default-language translator reports ErrKeyNotFound.
func BenchmarkGetStringMissingKey(b *testing.B) {
	tr := benchTranslator(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = tr.GetString("definitely_absent_key_xyz")
	}
}

// BenchmarkGetStringSlice hits a real sequence leaf, exercising the indexed slice lookup
// and the copy handed to callers.
func BenchmarkGetStringSlice(b *testing.B) {
	tr := benchTranslator(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := tr.GetStringSlice("misc_runs"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMustNewTranslator(b *testing.B) {
	benchManager(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if MustNewTranslator("en") == nil {
			b.Fatal("nil translator")
		}
	}
}

func BenchmarkGetTranslatorParallel(b *testing.B) {
	lm := benchManager(b)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := lm.GetTranslator("en"); err != nil {
				b.Fatal(err)
			}
		}
	})
}
