//go:build testtools

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func TestSetRulesReportsDatabaseFailureInsteadOfSuccess(t *testing.T) {
	restoreI18n, err := i18n.OverrideManagerForTest(`
common_settings_save_failed: save failed
rules_set_successfully: saved
`)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)

	if err := db.DB.Migrator().DropTable(&models.RulesSettings{}); err != nil {
		t.Fatalf("DropTable RulesSettings: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.RulesSettings{}); err != nil {
			t.Fatalf("restore RulesSettings: %v", err)
		}
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/setrules **Be kind**")

	if err := rulesModule.setRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("setRules() error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if text := calls[0].Params["text"]; text != "save failed" {
		t.Fatalf("failure reply = %q, want %q", text, "save failed")
	}
}
