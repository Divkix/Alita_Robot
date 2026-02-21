package i18n

import (
	"errors"
	"fmt"
	"strings"
	"testing"
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
