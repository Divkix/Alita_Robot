package i18n

import (
	"testing"
)

// minimalTranslator creates a Translator with only the fields needed for selectPluralForm tests.
// selectPluralForm only inspects the rule and count; it doesn't access t.manager or t.viper.
func minimalTranslator() *Translator {
	return &Translator{
		langCode: "en",
		manager:  &LocaleManager{defaultLang: "en"},
	}
}

func TestSelectPluralForm(t *testing.T) {
	t.Parallel()
	tr := minimalTranslator()

	tests := []struct {
		name  string
		rule  PluralRule
		count int
		want  string
	}{
		{
			name:  "zero form when count is 0 and zero is set",
			rule:  PluralRule{Zero: "no items", One: "one item", Other: "many items"},
			count: 0,
			want:  "no items",
		},
		{
			name:  "one form when count is 1",
			rule:  PluralRule{One: "one item", Other: "many items"},
			count: 1,
			want:  "one item",
		},
		{
			name:  "two form when count is 2 and two is set",
			rule:  PluralRule{One: "one item", Two: "two items", Other: "many items"},
			count: 2,
			want:  "two items",
		},
		{
			name:  "other form as fallback for count > 2",
			rule:  PluralRule{One: "one item", Other: "many items"},
			count: 5,
			want:  "many items",
		},
		{
			name:  "other form for count 0 when zero is empty",
			rule:  PluralRule{One: "one item", Other: "many items"},
			count: 0,
			want:  "many items",
		},
		{
			name:  "many form as fallback when other is empty",
			rule:  PluralRule{Many: "many items"},
			count: 10,
			want:  "many items",
		},
		{
			name:  "few form as fallback when other and many are empty",
			rule:  PluralRule{Few: "few items"},
			count: 10,
			want:  "few items",
		},
		{
			name:  "one form as final fallback",
			rule:  PluralRule{One: "one item"},
			count: 10,
			want:  "one item",
		},
		{
			name:  "empty rule returns empty string",
			rule:  PluralRule{},
			count: 5,
			want:  "",
		},
		{
			name:  "large count uses other form",
			rule:  PluralRule{One: "item", Other: "items"},
			count: 1000,
			want:  "items",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tr.selectPluralForm(tc.rule, tc.count)
			if got != tc.want {
				t.Errorf("selectPluralForm(%+v, %d) = %q, want %q", tc.rule, tc.count, got, tc.want)
			}
		})
	}
}

func TestExtractOrderedValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		params    TranslationParams
		wantLen   int
		wantFirst any
	}{
		{
			name:    "nil params returns nil",
			params:  nil,
			wantLen: 0,
		},
		{
			name:    "empty params returns empty",
			params:  TranslationParams{},
			wantLen: 0,
		},
		{
			name:      "numbered keys in order",
			params:    TranslationParams{"0": "first", "1": "second", "2": "third"},
			wantLen:   3,
			wantFirst: "first",
		},
		{
			name:      "numbered keys with gap stops at gap",
			params:    TranslationParams{"0": "first", "2": "third"},
			wantLen:   1,
			wantFirst: "first",
		},
		{
			name:      "named key 'first' is recognized",
			params:    TranslationParams{"first": "Alice"},
			wantLen:   1,
			wantFirst: "Alice",
		},
		{
			name:      "named key 'name' is recognized",
			params:    TranslationParams{"name": "Bob"},
			wantLen:   1,
			wantFirst: "Bob",
		},
		{
			name:      "common key 'count'",
			params:    TranslationParams{"count": 42},
			wantLen:   1,
			wantFirst: 42,
		},
		{
			name:      "numbered keys take priority over named",
			params:    TranslationParams{"0": "zero", "name": "ignored"},
			wantLen:   1,
			wantFirst: "zero",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractOrderedValues(tc.params)
			if len(got) != tc.wantLen {
				t.Errorf("extractOrderedValues() len = %d, want %d (values: %v)", len(got), tc.wantLen, got)
				return
			}
			if tc.wantLen > 0 && got[0] != tc.wantFirst {
				t.Errorf("extractOrderedValues()[0] = %v, want %v", got[0], tc.wantFirst)
			}
		})
	}
}
