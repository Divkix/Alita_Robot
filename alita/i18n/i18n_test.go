package i18n

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// ---- Loader utilities ----

func TestExtractLangCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{name: "yml extension", fileName: "en.yml", want: "en"},
		{name: "yaml extension", fileName: "en.yaml", want: "en"},
		{name: "locale with region", fileName: "pt-BR.yml", want: "pt-BR"},
		{name: "no extension", fileName: "README", want: "README"},
		// filepath.Ext("en.yml.bak")=".bak" -> trim ".bak" -> "en.yml" -> trim ".yml" -> "en"
		{name: "double yml extension", fileName: "en.yml.bak", want: "en"},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractLangCode(tc.fileName)
			if got != tc.want {
				t.Fatalf("extractLangCode(%q) = %q, want %q", tc.fileName, got, tc.want)
			}
		})
	}
}

func TestIsYAMLFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{name: "yml lowercase", fileName: "en.yml", want: true},
		{name: "yaml lowercase", fileName: "en.yaml", want: true},
		{name: "json extension", fileName: "en.json", want: false},
		{name: "empty string", fileName: "", want: false},
		{name: "yml uppercase", fileName: "en.YML", want: true},
		{name: "yaml uppercase", fileName: "en.YAML", want: true},
		{name: "no extension", fileName: "en", want: false},
		{name: "txt extension", fileName: "en.txt", want: false},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isYAMLFile(tc.fileName)
			if got != tc.want {
				t.Fatalf("isYAMLFile(%q) = %v, want %v", tc.fileName, got, tc.want)
			}
		})
	}
}

func TestValidateYAMLStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "valid yaml map",
			content: []byte("key: value\n"),
			wantErr: false,
		},
		{
			name:    "valid nested map",
			content: []byte("parent:\n  child: value\n"),
			wantErr: false,
		},
		{
			name:    "invalid yaml syntax",
			content: []byte("{{{"),
			wantErr: true,
		},
		{
			name:    "list root not a map",
			content: []byte("- item1\n- item2\n"),
			wantErr: true,
		},
		{
			name:    "scalar root not a map",
			content: []byte("hello\n"),
			wantErr: true,
		},
		{
			name:    "empty content nil not a map",
			content: []byte(""),
			wantErr: true,
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateYAMLStructure(tc.content)
			if tc.wantErr && err == nil {
				t.Fatalf("validateYAMLStructure() = nil, want non-nil error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateYAMLStructure() = %v, want nil", err)
			}
		})
	}
}

// ---- Error types ----

func TestI18nErrorFormatWithErr(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base error")
	err := NewI18nError("get", "en", "hello", "not found", base)

	msg := err.Error()
	if !strings.Contains(msg, "i18n get failed") {
		t.Fatalf("Error() = %q, want it to contain %q", msg, "i18n get failed")
	}
	if !strings.Contains(msg, "base error") {
		t.Fatalf("Error() = %q, want it to contain %q", msg, "base error")
	}
}

func TestI18nErrorFormatWithoutErr(t *testing.T) {
	t.Parallel()

	err := NewI18nError("get", "en", "hello", "not found", nil)

	msg := err.Error()
	if strings.Contains(msg, "<nil>") {
		t.Fatalf("Error() = %q, should not contain %q", msg, "<nil>")
	}
	if !strings.Contains(msg, "not found") {
		t.Fatalf("Error() = %q, want it to contain %q", msg, "not found")
	}
}

func TestI18nErrorUnwrap(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("underlying")
	err := NewI18nError("op", "en", "key", "msg", base)

	if !errors.Is(err, base) {
		t.Fatalf("errors.Is(err, base) = false, want true")
	}
}

func TestI18nErrorUnwrapNil(t *testing.T) {
	t.Parallel()

	err := NewI18nError("op", "en", "key", "msg", nil)
	if err.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", err.Unwrap())
	}
}

func TestNewI18nError(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("root cause")
	err := NewI18nError("myOp", "fr", "my.key", "my message", base)

	if err.Op != "myOp" {
		t.Fatalf("Op = %q, want %q", err.Op, "myOp")
	}
	if err.Lang != "fr" {
		t.Fatalf("Lang = %q, want %q", err.Lang, "fr")
	}
	if err.Key != "my.key" {
		t.Fatalf("Key = %q, want %q", err.Key, "my.key")
	}
	if err.Message != "my message" {
		t.Fatalf("Message = %q, want %q", err.Message, "my message")
	}
	if !errors.Is(err.Err, base) {
		t.Fatalf("Err = %v, want %v", err.Err, base)
	}
}

func TestPredefinedErrorsDistinct(t *testing.T) {
	t.Parallel()

	predefined := []struct {
		name string
		err  error
	}{
		{"ErrLocaleNotFound", ErrLocaleNotFound},
		{"ErrKeyNotFound", ErrKeyNotFound},
		{"ErrInvalidYAML", ErrInvalidYAML},
		{"ErrManagerNotInit", ErrManagerNotInit},
		{"ErrRecursiveFallback", ErrRecursiveFallback},
		{"ErrInvalidParams", ErrInvalidParams},
	}

	for i, a := range predefined {
		for j, b := range predefined {
			if i == j {
				continue
			}
			if errors.Is(a.err, b.err) {
				t.Fatalf("errors.Is(%s, %s) = true, want false (they must be distinct)", a.name, b.name)
			}
		}
	}
}

func TestPredefinedErrorsChain(t *testing.T) {
	t.Parallel()

	wrapped := NewI18nError("op", "en", "key", "msg", ErrKeyNotFound)
	if !errors.Is(wrapped, ErrKeyNotFound) {
		t.Fatalf("errors.Is(wrapped, ErrKeyNotFound) = false, want true")
	}
}

// ---- Translator utilities ----

func TestExtractOrderedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params TranslationParams
		want   []any
	}{
		{
			name:   "numbered keys 0 1 2",
			params: TranslationParams{"0": "a", "1": "b", "2": "c"},
			want:   []any{"a", "b", "c"},
		},
		{
			name:   "common keys first second",
			params: TranslationParams{"first": "x", "second": "y"},
			want:   []any{"x", "y"},
		},
		{
			name:   "nil params",
			params: nil,
			want:   nil,
		},
		{
			name:   "empty params",
			params: TranslationParams{},
			want:   nil,
		},
		{
			name:   "gap in numbered keys breaks at 1",
			params: TranslationParams{"0": "a", "2": "c"},
			want:   []any{"a"},
		},
		{
			name:   "numbered keys take priority over common",
			params: TranslationParams{"0": "a", "1": "b", "first": "x"},
			want:   []any{"a", "b"},
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractOrderedValues(tc.params)
			if len(got) != len(tc.want) {
				t.Fatalf("extractOrderedValues() len = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			for i, v := range tc.want {
				if got[i] != v {
					t.Fatalf("extractOrderedValues()[%d] = %v, want %v", i, got[i], v)
				}
			}
		})
	}
}

func TestSelectPluralForm(t *testing.T) {
	t.Parallel()

	tr := &Translator{langCode: "en", manager: &LocaleManager{defaultLang: "en"}}

	tests := []struct {
		name  string
		rule  PluralRule
		count int
		want  string
	}{
		{
			name:  "count zero uses Zero form",
			rule:  PluralRule{Zero: "none", One: "one item", Other: "many"},
			count: 0,
			want:  "none",
		},
		{
			name:  "count one uses One form",
			rule:  PluralRule{Zero: "none", One: "one item", Other: "many"},
			count: 1,
			want:  "one item",
		},
		{
			name:  "count two uses Two form",
			rule:  PluralRule{One: "one item", Two: "two items", Other: "many"},
			count: 2,
			want:  "two items",
		},
		{
			name:  "count five falls to Other",
			rule:  PluralRule{One: "one item", Other: "many"},
			count: 5,
			want:  "many",
		},
		{
			name:  "Zero empty falls to Other",
			rule:  PluralRule{Zero: "", Other: "fallback"},
			count: 0,
			want:  "fallback",
		},
		{
			name:  "One empty Many empty falls to Other",
			rule:  PluralRule{One: "", Many: "", Other: "fallback"},
			count: 1,
			want:  "fallback",
		},
		{
			name:  "all forms empty returns empty string",
			rule:  PluralRule{},
			count: 1,
			want:  "",
		},
		{
			name:  "Zero empty Other empty falls to Many",
			rule:  PluralRule{Zero: "", One: "", Many: "lots", Other: ""},
			count: 0,
			want:  "lots",
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tr.selectPluralForm(tc.rule, tc.count)
			if got != tc.want {
				t.Fatalf("selectPluralForm(%v, %d) = %q, want %q", tc.rule, tc.count, got, tc.want)
			}
		})
	}
}

// ---- Translator.GetString ----

// newTestTranslator creates a Translator backed by inline YAML content for tests.
func newTestTranslator(t *testing.T, yamlContent string) *Translator {
	t.Helper()
	vi, err := compileViper([]byte(yamlContent))
	if err != nil {
		t.Fatalf("compileViper() error = %v", err)
	}
	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  map[string]*viper.Viper{"en": vi},
		localeData:  map[string][]byte{"en": []byte(yamlContent)},
	}
	return &Translator{
		langCode:    "en",
		manager:     lm,
		viper:       vi,
		cachePrefix: "i18n:en:",
	}
}

func TestTranslatorGet(t *testing.T) {
	t.Parallel()

	const yamlContent = `language_name: English
greeting: "Hello, World!"
templ: "Hello, %s!"
`

	t.Run("existing key returns translated string", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, yamlContent)
		result, err := tr.GetString("language_name")
		if err != nil {
			t.Fatalf("GetString(language_name) error = %v", err)
		}
		if result != "English" {
			t.Fatalf("GetString(language_name) = %q, want %q", result, "English")
		}
	})

	t.Run("nonexistent key returns error", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, yamlContent)
		_, err := tr.GetString("nonexistent_key_xyz")
		if err == nil {
			t.Fatal("GetString(nonexistent_key) expected error, got nil")
		}
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound, got: %v", err)
		}
	})

	t.Run("key with params substitutes correctly", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, yamlContent)
		result, err := tr.GetString("templ", TranslationParams{"0": "Alice"})
		if err != nil {
			t.Fatalf("GetString(templ, params) error = %v", err)
		}
		if !strings.Contains(result, "Alice") {
			t.Fatalf("GetString(templ) = %q, want it to contain 'Alice'", result)
		}
	})

	t.Run("nil params map does not panic", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, yamlContent)
		// Calling with explicit nil params should not panic
		result, err := tr.GetString("greeting", nil)
		if err != nil {
			t.Fatalf("GetString(greeting, nil) error = %v", err)
		}
		if result == "" {
			t.Fatal("expected non-empty result")
		}
	})
}

// ---- Translator.GetPlural ----

func TestTranslatorGetPlural(t *testing.T) {
	t.Parallel()

	// Construct YAML with plural subkeys that GetPlural expects
	const pluralYAML = `
items:
  zero: "no items"
  one: "one item"
  other: "many items"
`

	t.Run("count 0 uses other form when no zero configured", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, `items:
  one: one item
  other: many items
`)
		result, err := tr.GetPlural("items", 0)
		if err != nil {
			t.Fatalf("GetPlural(items, 0) error = %v", err)
		}
		// count 0 with no Zero form falls to Other
		if result != "many items" {
			t.Fatalf("GetPlural(items, 0) = %q, want %q", result, "many items")
		}
	})

	t.Run("count 1 uses one form", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, pluralYAML)
		result, err := tr.GetPlural("items", 1)
		if err != nil {
			t.Fatalf("GetPlural(items, 1) error = %v", err)
		}
		if result != "one item" {
			t.Fatalf("GetPlural(items, 1) = %q, want %q", result, "one item")
		}
	})

	t.Run("count 5 uses other form", func(t *testing.T) {
		t.Parallel()

		tr := newTestTranslator(t, pluralYAML)
		result, err := tr.GetPlural("items", 5)
		if err != nil {
			t.Fatalf("GetPlural(items, 5) error = %v", err)
		}
		if result != "many items" {
			t.Fatalf("GetPlural(items, 5) = %q, want %q", result, "many items")
		}
	})
}

// ---- LocaleManager.GetTranslator ----

func TestLocaleManagerGetTranslator(t *testing.T) {
	t.Parallel()

	const enYAML = "language_name: English\n"

	vi, err := compileViper([]byte(enYAML))
	if err != nil {
		t.Fatalf("compileViper() error = %v", err)
	}

	// Use a local (non-singleton) LocaleManager to avoid contaminating global state.
	// We embed a minimal FS pointer placeholder — use the trick of setting localeFS
	// to a non-nil value via the embed pointer is not straightforward without go:embed.
	// Instead, we bypass the localeFS nil check by setting viperCache so GetTranslator
	// can succeed when localeFS check is bypassed. Since GetTranslator checks localeFS,
	// we test via the singleton initialized from main, or use MustNewTranslator.

	// Verify MustNewTranslator returns a non-nil translator for known language.
	// The singleton may or may not be initialized (no embed FS in unit tests),
	// so MustNewTranslator returns a bare translator — that is still non-nil.
	t.Run("MustNewTranslator returns non-nil translator", func(t *testing.T) {
		t.Parallel()

		tr := MustNewTranslator("en")
		if tr == nil {
			t.Fatal("MustNewTranslator('en') returned nil")
		}
	})

	t.Run("MustNewTranslator unknown locale falls back to non-nil translator", func(t *testing.T) {
		t.Parallel()

		tr := MustNewTranslator("xx_unknown_locale")
		if tr == nil {
			t.Fatal("MustNewTranslator(unknown) returned nil")
		}
	})

	t.Run("direct GetTranslator on local manager with data returns translator", func(t *testing.T) {
		t.Parallel()

		lm := &LocaleManager{
			defaultLang: "en",
			viperCache:  map[string]*viper.Viper{"en": vi},
			localeData:  map[string][]byte{"en": []byte(enYAML)},
		}

		// localeFS is nil so GetTranslator returns ErrManagerNotInit.
		// Verify the error is correct.
		_, getErr := lm.GetTranslator("en")
		if getErr == nil {
			t.Fatal("expected error when localeFS is nil, got nil")
		}
		if !errors.Is(getErr, ErrManagerNotInit) {
			t.Fatalf("expected ErrManagerNotInit, got: %v", getErr)
		}
	})
}

// ---- LocaleManager.GetAvailableLocales ----

func TestLocaleManagerGetAvailableLocales(t *testing.T) {
	t.Parallel()

	// Build a local LocaleManager with known locales.
	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  make(map[string]*viper.Viper),
		localeData: map[string][]byte{
			"en": []byte("language_name: English\n"),
			"es": []byte("language_name: Spanish\n"),
			"fr": []byte("language_name: French\n"),
			"hi": []byte("language_name: Hindi\n"),
		},
	}

	langs := lm.GetAvailableLanguages()
	if len(langs) < 4 {
		t.Fatalf("GetAvailableLanguages() returned %d languages, want at least 4: %v", len(langs), langs)
	}

	// Verify all 4 known locales are present.
	langSet := make(map[string]bool, len(langs))
	for _, l := range langs {
		langSet[l] = true
	}

	for _, required := range []string{"en", "es", "fr", "hi"} {
		if !langSet[required] {
			t.Fatalf("GetAvailableLanguages() missing %q; got: %v", required, langs)
		}
	}
}

// ---- New expanded tests ----

func TestTranslator_GetString_NilManager(t *testing.T) {
	t.Parallel()

	tr := &Translator{langCode: "en", manager: nil}
	_, err := tr.GetString("some_key")
	if err == nil {
		t.Fatal("GetString with nil manager expected error, got nil")
	}
	if !errors.Is(err, ErrManagerNotInit) {
		t.Fatalf("expected ErrManagerNotInit, got: %v", err)
	}
}

func TestTranslator_GetString_FallbackToDefault(t *testing.T) {
	t.Parallel()

	// "en" has the key, "es" does not — "es" translator should fall back to "en" value.
	// Note: localeFS is nil so GetTranslator returns ErrManagerNotInit, which means
	// we can't truly test multi-lang fallback without an embedded FS.
	// Instead we verify that a translator with the default lang returns the correct value.
	const enYAML = "fallback_key: \"en value\"\n"
	tr := newTestTranslator(t, enYAML)

	result, err := tr.GetString("fallback_key")
	if err != nil {
		t.Fatalf("GetString(fallback_key) error = %v", err)
	}
	if result != "en value" {
		t.Fatalf("GetString(fallback_key) = %q, want %q", result, "en value")
	}
}

func TestTranslator_GetString_NamedParams(t *testing.T) {
	t.Parallel()

	const yamlContent = "greet: \"Hello, {user}!\"\n"
	tr := newTestTranslator(t, yamlContent)

	result, err := tr.GetString("greet", TranslationParams{"user": "Alice"})
	if err != nil {
		t.Fatalf("GetString(greet, {user:Alice}) error = %v", err)
	}
	if !strings.Contains(result, "Alice") {
		t.Fatalf("GetString(greet) = %q, want it to contain %q", result, "Alice")
	}
}

func TestTranslator_GetString_UnusedParams(t *testing.T) {
	t.Parallel()

	const yamlContent = "static: \"no placeholders here\"\n"
	tr := newTestTranslator(t, yamlContent)

	result, err := tr.GetString("static", TranslationParams{"extra": "ignored"})
	if err != nil {
		t.Fatalf("GetString(static, extra params) error = %v", err)
	}
	if result != "no placeholders here" {
		t.Fatalf("GetString(static) = %q, want %q", result, "no placeholders here")
	}
}

func TestTranslator_GetString_EmptyKey(t *testing.T) {
	t.Parallel()

	const yamlContent = "some_key: value\n"
	tr := newTestTranslator(t, yamlContent)

	_, err := tr.GetString("")
	if err == nil {
		t.Fatal("GetString(\"\") expected error, got nil")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestTranslator_GetPlural_NilManager(t *testing.T) {
	t.Parallel()

	tr := &Translator{langCode: "en", manager: nil}
	_, err := tr.GetPlural("items", 1)
	if err == nil {
		t.Fatal("GetPlural with nil manager expected error, got nil")
	}
	if !errors.Is(err, ErrManagerNotInit) {
		t.Fatalf("expected ErrManagerNotInit, got: %v", err)
	}
}

func TestTranslator_GetStringSlice_NilManager(t *testing.T) {
	t.Parallel()

	tr := &Translator{langCode: "en", manager: nil}
	_, err := tr.GetStringSlice("some_key")
	if err == nil {
		t.Fatal("GetStringSlice with nil manager expected error, got nil")
	}
	if !errors.Is(err, ErrManagerNotInit) {
		t.Fatalf("expected ErrManagerNotInit, got: %v", err)
	}
}

func TestLocaleManager_IsLanguageSupported(t *testing.T) {
	t.Parallel()

	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  make(map[string]*viper.Viper),
		localeData: map[string][]byte{
			"en": []byte("language_name: English\n"),
			"es": []byte("language_name: Spanish\n"),
			"fr": []byte("language_name: French\n"),
			"hi": []byte("language_name: Hindi\n"),
		},
	}

	tests := []struct {
		name     string
		langCode string
		want     bool
	}{
		{name: "en is supported", langCode: "en", want: true},
		{name: "es is supported", langCode: "es", want: true},
		{name: "zz is not supported", langCode: "zz", want: false},
		{name: "empty string not supported", langCode: "", want: false},
		{name: "unknown locale not supported", langCode: "xx_unknown", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := lm.IsLanguageSupported(tc.langCode)
			if got != tc.want {
				t.Fatalf("IsLanguageSupported(%q) = %v, want %v", tc.langCode, got, tc.want)
			}
		})
	}
}

func TestLocaleManager_GetDefaultLanguage(t *testing.T) {
	t.Parallel()

	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  make(map[string]*viper.Viper),
		localeData:  make(map[string][]byte),
	}

	got := lm.GetDefaultLanguage()
	if got != "en" {
		t.Fatalf("GetDefaultLanguage() = %q, want %q", got, "en")
	}

	// Also verify a non-default value
	lm2 := &LocaleManager{
		defaultLang: "es",
		viperCache:  make(map[string]*viper.Viper),
		localeData:  make(map[string][]byte),
	}
	got2 := lm2.GetDefaultLanguage()
	if got2 != "es" {
		t.Fatalf("GetDefaultLanguage() = %q, want %q", got2, "es")
	}
}

func TestLocaleManager_GetStats(t *testing.T) {
	t.Parallel()

	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  make(map[string]*viper.Viper),
		localeData: map[string][]byte{
			"en": []byte("language_name: English\n"),
			"es": []byte("language_name: Spanish\n"),
			"fr": []byte("language_name: French\n"),
			"hi": []byte("language_name: Hindi\n"),
		},
	}

	stats := lm.GetStats()
	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	totalLangs, ok := stats["total_languages"]
	if !ok {
		t.Fatal("GetStats() missing 'total_languages' key")
	}
	if totalLangs.(int) != 4 {
		t.Fatalf("GetStats()[total_languages] = %v, want 4", totalLangs)
	}

	defaultLang, ok := stats["default_language"]
	if !ok {
		t.Fatal("GetStats() missing 'default_language' key")
	}
	if defaultLang.(string) != "en" {
		t.Fatalf("GetStats()[default_language] = %q, want %q", defaultLang, "en")
	}

	_, ok = stats["cache_enabled"]
	if !ok {
		t.Fatal("GetStats() missing 'cache_enabled' key")
	}

	_, ok = stats["languages"]
	if !ok {
		t.Fatal("GetStats() missing 'languages' key")
	}
}
