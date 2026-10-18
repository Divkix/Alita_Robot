//go:build testtools

package modules

import (
	"os"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The stored-messages summary must render with every value in every locale,
// and with {name} placeholders too.
func TestCaptchaStoredMessagesSummaryFillsEveryLocale(t *testing.T) {
	sources := map[string]string{
		"named": "captcha_stored_messages_summary: \"{user} sent {count} ({types})\"\n",
	}
	for _, code := range localeCodes {
		raw, err := os.ReadFile("../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		sources[code] = string(raw)
	}

	params := captchaStoredSummaryParams(`<a href="tg://user?id=1">Bob</a>`, 3, "text, photo")
	for name, content := range sources {
		tr, err := i18n.NewTestTranslator(content)
		if err != nil {
			t.Fatalf("%s: NewTestTranslator: %v", name, err)
		}
		text, _ := tr.GetString("captcha_stored_messages_summary", params)
		for _, want := range []string{"Bob", "3", "text, photo"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s summary = %q, want it to contain %q", name, text, want)
			}
		}
		for _, bad := range []string{"%s", "%d", "%!", "{"} {
			if strings.Contains(text, bad) {
				t.Errorf("%s summary = %q, want no %q", name, text, bad)
			}
		}
	}
}
