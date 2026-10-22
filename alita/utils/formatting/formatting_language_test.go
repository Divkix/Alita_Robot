//go:build testtools

package formatting

import (
	"os"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// formattingLanguageFixture loads the real en and ru locales and an in-memory
// database that holds chat and user language rows.
func formattingLanguageFixture(t *testing.T, name string) {
	t.Helper()

	locales := map[string]string{}
	for _, code := range []string{"en", "ru"} {
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

	originalDB := db.DB
	sqliteDB, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.DB = sqliteDB
	t.Cleanup(func() {
		if sqlDB, sqlErr := db.DB.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		db.DB = originalDB
	})
	if err := db.DB.AutoMigrate(&db.Chat{}, &models.User{}, &models.RulesSettings{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
}

// russianLabel returns the ru value of key and fails unless it differs from the
// en value, so a test comparing against it cannot pass on English text.
func russianLabel(t *testing.T, key string) string {
	t.Helper()
	ru, _ := i18n.MustNewTranslator("ru").GetString(key)
	en, _ := i18n.MustNewTranslator("en").GetString(key)
	if ru == "" || ru == en {
		t.Fatalf("locales ru %s = %q, en = %q; want a distinct Russian value", key, ru, en)
	}
	return ru
}

func TestFormattingReplacerFallbacksFollowTheChatLanguage(t *testing.T) {
	formattingLanguageFixture(t, "formatting_language_group")
	chat := &gotgbot.Chat{Id: -100888, Type: "supergroup", Title: "Russian Chat"}
	if err := db.DB.Create(&db.Chat{ChatId: chat.Id, Language: "ru"}).Error; err != nil {
		t.Fatalf("create chat: %v", err)
	}
	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}

	noName := russianLabel(t, "helpers_person_no_name")
	got, _ := FormattingReplacer(bot, chat, nil, "{first}", nil)
	if got != noName {
		t.Errorf("nil user in a ru chat = %q, want %q", got, noName)
	}
	got, _ = FormattingReplacer(bot, chat, &gotgbot.User{Id: 5}, "{first}", nil)
	if got != noName {
		t.Errorf("user without a first name in a ru chat = %q, want %q", got, noName)
	}

	if err := rules.SetChatRules(chat.Id, "Keep it tidy."); err != nil {
		t.Fatalf("SetChatRules: %v", err)
	}
	_, buttons := FormattingReplacer(bot, chat, &gotgbot.User{Id: 5, FirstName: "Ada"}, "{rules}", nil)
	if len(buttons) != 1 {
		t.Fatalf("buttons = %#v, want one rules button", buttons)
	}
	if want := russianLabel(t, "button_rules_default"); buttons[0].Name != want {
		t.Errorf("default rules button in a ru chat = %q, want %q", buttons[0].Name, want)
	}
}

func TestFormattingReplacerFallbacksInPrivateChatFollowTheUserLanguage(t *testing.T) {
	formattingLanguageFixture(t, "formatting_language_private")
	chat := &gotgbot.Chat{Id: 7001, Type: "private", FirstName: "Ivan"}
	if err := db.DB.Create(&models.User{UserId: chat.Id, Language: "ru"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, _ := FormattingReplacer(nil, chat, nil, "{first}", nil)
	if want := russianLabel(t, "helpers_person_no_name"); got != want {
		t.Errorf("nil user in a ru user's PM = %q, want %q", got, want)
	}
}
