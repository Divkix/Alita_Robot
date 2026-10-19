//go:build testtools

package keyboard

import (
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

func TestInitButtonsUsesTheGivenTranslator(t *testing.T) {
	locales := map[string]string{}
	for _, code := range []string{"en", "es"} {
		raw, err := os.ReadFile("../../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		locales[code] = string(raw)
	}
	restore, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restore)

	es := i18n.MustNewTranslator("es")
	want, _ := es.GetString("helpers_user_commands")
	english, _ := i18n.MustNewTranslator("en").GetString("helpers_user_commands")
	if want == "" || want == english {
		t.Fatalf("es helpers_user_commands = %q, en = %q; want a distinct Spanish label", want, english)
	}

	kb := InitButtons(newKeyboardBot(), es, -1001, 42)
	if got := kb.InlineKeyboard[0][0].Text; got != want {
		t.Fatalf("InitButtons(es) user button = %q, want %q", got, want)
	}
}
